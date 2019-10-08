package main

import (
	"fmt"
	"net"
	"os"

	"google.golang.org/grpc"

	snapshotsapi "github.com/containerd/containerd/api/services/snapshots/v1"
	"github.com/containerd/containerd/contrib/snapshotservice"
	"github.com/containerd/containerd/snapshots/overlay"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "o9p"
	app.Version = "1"
	app.Usage = "overlayfs over 9p"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug output in the logs",
		},
		cli.StringFlag{
			Name:  "address,a",
			Usage: "address",
			Value: "127.0.0.1",
		},
		cli.StringFlag{
			Name:  "root",
			Usage: "root dir",
			Value: "/var/lib/containerd/o9p",
		},
		cli.StringFlag{
			Name:  "port",
			Value: "9876",
		},
	}
	app.Action = func(clix *cli.Context) error {
		// Create a gRPC server
		var (
			rpc     = grpc.NewServer()
			root    = clix.String("root")
			address = clix.String("address")
		)
		if root == "" {
			return errors.New("root cannot be empty")
		}
		sn, err := overlay.NewSnapshotter(root, overlay.AsynchronousRemove)
		if err != nil {
			return err
		}
		defer sn.Close()

		service := snapshotservice.FromSnapshotter(NewSnapshotterServer(root, address, sn))

		snapshotsapi.RegisterSnapshotsServer(rpc, service)

		l, err := net.Listen("tcp", net.JoinHostPort(address, clix.String("port")))
		if err != nil {
			return err
		}
		defer l.Close()

		return rpc.Serve(l)

	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
}
