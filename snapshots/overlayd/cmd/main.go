package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"

	"github.com/containerd/containerd"
	snapshotsapi "github.com/containerd/containerd/api/services/snapshots/v1"
	"github.com/containerd/containerd/contrib/snapshotservice"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/containerd/snapshots/overlayd"
	snproxy "github.com/containerd/containerd/snapshots/proxy"
)

func main() {
	address := os.Args[1]
	root, err := ioutil.TempDir("", "overlayd")
	if err != nil {
		exit(err)
	}
	log.Println("root:", root)
	remote := "127.0.0.1"

	var ov snapshots.Snapshotter
	if len(os.Args) < 3 {
		log.Println("running as server connected to containerd")
		client, err := containerd.New(defaults.DefaultAddress)
		if err != nil {
			exit(err)
		}
		defer client.Close()
		ov = client.SnapshotService(containerd.DefaultSnapshotter)
	} else {
		log.Printf("running as proxy connected to %s\n", os.Args[2])
		remote, _, _ = net.SplitHostPort(os.Args[2])
		conn, err := grpc.Dial(os.Args[2], grpc.WithInsecure())
		if err != nil {
			exit(err)
		}
		defer conn.Close()
		ov = snproxy.NewSnapshotter(snapshotsapi.NewSnapshotsClient(conn), "overlay")
	}

	// Configure your custom snapshotter, this example uses the native
	// snapshotter and a root directory. Your custom snapshotter will be
	// much more useful than using a snapshotter which is already included.
	// https://godoc.org/github.com/containerd/containerd/snapshots#Snapshotter
	sn, err := overlayd.NewSnapshotter(remote, root, ov)
	if err != nil {
		exit(err)
	}

	// Convert the snapshotter to a gRPC service,
	// example in github.com/containerd/containerd/contrib/snapshotservice
	service := snapshotservice.FromSnapshotter(sn)

	rpc := grpc.NewServer()
	snapshotsapi.RegisterSnapshotsServer(rpc, service)

	// Listen and serve
	l, err := net.Listen("tcp", address)
	if err != nil {
		exit(err)
	}
	log.Println("serve", address)
	if err := rpc.Serve(l); err != nil {
		exit(err)
	}
}

func exit(err error) {
	fmt.Printf("error: %v\n", err)
	os.Exit(1)
}
