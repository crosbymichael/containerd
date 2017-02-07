package main

import (
	gocontext "context"
	"fmt"

	"github.com/docker/containerd/api/execution"
	"github.com/urfave/cli"
)

var deleteCommand = cli.Command{
	Name:      "delete",
	Usage:     "delete a container from containerd",
	ArgsUsage: "CONTAINER",
	Flags:     []cli.Flag{},
	Action: func(context *cli.Context) error {
		executionService, err := getExecutionService(context)
		if err != nil {
			return err
		}
		id := context.Args().First()
		if id == "" {
			return fmt.Errorf("container id must be provided")
		}
		if _, err := executionService.Delete(gocontext.Background(), &execution.DeleteContainerRequest{
			ID: id,
		}); err != nil {
			return err
		}
		return nil
	},
}
