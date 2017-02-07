package main

import (
	"bytes"
	gocontext "context"
	"encoding/json"
	"io"
	"os"

	"github.com/docker/containerd/api/execution"
	"github.com/urfave/cli"
)

var eventsCommand = cli.Command{
	Name:  "events",
	Usage: "display containerd events",
	Action: func(context *cli.Context) error {
		executionService, err := getExecutionService(context)
		if err != nil {
			return err
		}
		events, err := executionService.Events(gocontext.Background(), &execution.EventsRequest{})
		if err != nil {
			return err
		}
		for {
			e, err := events.Recv()
			if err != nil {
				return err
			}
			data, err := json.Marshal(e)
			if err != nil {
				return err
			}
			buf := bytes.NewBuffer(nil)
			if err := json.Indent(buf, data, "", "\t"); err != nil {
				return err
			}
			io.Copy(os.Stdout, buf)
			buf.Reset()
		}
		return nil
	},
}
