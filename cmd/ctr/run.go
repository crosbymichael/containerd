package main

import (
	"fmt"
	"os"
	"path/filepath"

	gocontext "context"

	"github.com/Sirupsen/logrus"
	"github.com/crosbymichael/console"
	"github.com/docker/containerd/api/execution"
	"github.com/urfave/cli"
)

var runCommand = cli.Command{
	Name:  "run",
	Usage: "run a container",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "bundle, b",
			Usage: "path to the container's bundle",
		},
		cli.BoolFlag{
			Name:  "tty, t",
			Usage: "allocate a TTY for the container",
		},
	},
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		if id == "" {
			return fmt.Errorf("container id must be provided")
		}
		executionService, err := getExecutionService(context)
		if err != nil {
			return err
		}
		tmpDir, err := getTempDir(id)
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		bundle, err := filepath.Abs(context.String("bundle"))
		if err != nil {
			return err
		}
		opts := &execution.CreateContainerRequest{
			ID:         id,
			BundlePath: bundle,
			Console:    context.Bool("tty"),
			Stdin:      filepath.Join(tmpDir, "stdin"),
			Stdout:     filepath.Join(tmpDir, "stdout"),
			Stderr:     filepath.Join(tmpDir, "stderr"),
		}
		term := console.Current()
		if opts.Console {
			if term.SetRaw(); err != nil {
				return err
			}
			defer term.Reset()
		}
		fwg, err := prepareStdio(opts.Stdin, opts.Stdout, opts.Stderr, opts.Console)
		if err != nil {
			return err
		}
		events, err := executionService.Events(gocontext.Background(), &execution.EventsRequest{})
		if err != nil {
			return err
		}
		cr, err := executionService.Create(gocontext.Background(), opts)
		if err != nil {
			return err
		}
		if _, err := executionService.Start(gocontext.Background(), &execution.StartContainerRequest{
			ID: id,
		}); err != nil {
			return err
		}

		var ec uint32
	eventLoop:
		for {
			e, err := events.Recv()
			if err != nil {
				logrus.WithError(err).Error("ctr: receive events")
				continue
			}
			if e.ID == id && e.Pid == cr.InitProcess.Pid {
				ec = e.ExitStatus
				break eventLoop
			}
		}

		if _, err := executionService.Delete(gocontext.Background(), &execution.DeleteContainerRequest{
			ID: id,
		}); err != nil {
			return err
		}
		// Ensure we read all io
		fwg.Wait()

		term.Reset()
		os.Exit(int(ec))

		return nil
	},
}
