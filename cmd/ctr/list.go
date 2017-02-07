package main

import (
	gocontext "context"
	"fmt"

	"github.com/docker/containerd/api/execution"
	"github.com/urfave/cli"
)

var listCommand = cli.Command{
	Name:  "list",
	Usage: "list containers",
	Action: func(context *cli.Context) error {
		executionService, err := getExecutionService(context)
		if err != nil {
			return err
		}
		listResponse, err := executionService.List(gocontext.Background(), &execution.ListContainersRequest{
			Owner: []string{},
		})
		if err != nil {
			return err
		}
		fmt.Printf("ID\tSTATUS\tPROCS\tBUNDLE\n")
		for _, c := range listResponse.Containers {
			fmt.Printf("%s\t%s\t%d\t%s\n",
				c.ID,
				c.Status,
				0, // number of processes
				c.Bundle,
			)
		}
		return nil
	},
}
