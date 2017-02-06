package main

import (
	_ "expvar"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	gocontext "golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/Sirupsen/logrus"
	"github.com/docker/containerd"
	api "github.com/docker/containerd/api/execution"
	"github.com/docker/containerd/log"
	"github.com/docker/containerd/supervisor"
	"github.com/docker/containerd/utils"
	metrics "github.com/docker/go-metrics"
	"github.com/urfave/cli"
)

const usage = `
                    __        _                     __
  _________  ____  / /_____ _(_)___  ___  _________/ /
 / ___/ __ \/ __ \/ __/ __ ` + "`" + `/ / __ \/ _ \/ ___/ __  /
/ /__/ /_/ / / / / /_/ /_/ / / / / /  __/ /  / /_/ /
\___/\____/_/ /_/\__/\__,_/_/_/ /_/\___/_/   \__,_/

high performance container runtime
`

func main() {
	app := cli.NewApp()
	app.Name = "containerd"
	app.Version = containerd.Version
	app.Usage = usage
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug output in logs",
		},
		cli.StringFlag{
			Name:  "root",
			Usage: "containerd state directory",
			Value: "/run/containerd",
		},
		cli.StringFlag{
			Name:  "socket,s",
			Usage: "socket path for containerd's GRPC server",
			Value: "/run/containerd/containerd.sock",
		},
		cli.StringFlag{
			Name:  "debug-socket,d",
			Usage: "socket path for containerd's debug server",
			Value: "/run/containerd/containerd-debug.sock",
		},
		cli.StringFlag{
			Name:  "metrics-address,m",
			Usage: "tcp address to serve metrics on",
			Value: "127.0.0.1:7897",
		},
	}
	app.Before = func(context *cli.Context) error {
		if context.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	app.Action = func(context *cli.Context) error {
		signals := make(chan os.Signal, 2048)
		signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

		ctx := log.WithModule(gocontext.Background(), "containerd")

		if address := context.GlobalString("metrics-address"); address != "" {
			go serveMetrics(ctx, address)
		}
		if err := serveDebug(ctx, context); err != nil {
			return err
		}
		path := context.GlobalString("socket")
		if path == "" {
			return fmt.Errorf("--socket path cannot be empty")
		}
		l, err := utils.CreateUnixSocket(path)
		if err != nil {
			return err
		}

		execCtx := log.WithModule(ctx, "execution")
		execService, err := supervisor.New(execCtx, context.GlobalString("root"))
		if err != nil {
			return err
		}

		server := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
		api.RegisterExecutionServiceServer(server, execService)

		go serveGRPC(ctx, server, l)
		for s := range signals {
			switch s {
			default:
				log.G(ctx).WithField("signal", s).Info("stopping GRPC server")
				server.Stop()
				return nil
			}
		}
		return nil
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "containerd: %s\n", err)
		os.Exit(1)
	}
}

func interceptor(ctx gocontext.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	ctx = log.WithModule(ctx, "containerd")
	switch info.Server.(type) {
	case api.ExecutionServiceServer:
		ctx = log.WithModule(ctx, "execution")
	default:
		fmt.Printf("unknown type: %#v\n", info.Server)
	}
	return handler(ctx, req)
}

func serveMetrics(ctx gocontext.Context, address string) {
	log.G(ctx).WithField("metrics-address", address).Info("listening and serving metrics")
	m := http.NewServeMux()
	m.Handle("/metrics", metrics.Handler())
	if err := http.ListenAndServe(address, m); err != nil {
		log.G(ctx).WithError(err).Fatal("metrics server failure")
	}
}

func serveGRPC(ctx gocontext.Context, server *grpc.Server, l net.Listener) {
	log.G(ctx).WithField("socket", l.Addr()).Info("start serving GRPC API")
	defer l.Close()
	if err := server.Serve(l); err != nil {
		log.G(ctx).WithError(err).Fatal("GRPC server failure")
	}
}

func serveProfiler(ctx gocontext.Context, l net.Listener) {
	if err := http.Serve(l, nil); err != nil {
		log.G(ctx).WithError(err).Fatal("profiler server failure")
	}
}

func serveDebug(ctx gocontext.Context, context *cli.Context) error {
	debugPath := context.GlobalString("debug-socket")
	if debugPath == "" {
		return nil
	}
	d, err := utils.CreateUnixSocket(debugPath)
	if err != nil {
		return err
	}

	//publish profiling and debug socket.
	log.G(ctx).WithField("socket", debugPath).Info("starting profiler handlers")
	log.G(ctx).WithFields(logrus.Fields{
		"expvars": "/debug/vars",
		"socket":  debugPath,
	}).Debug("serve expvars")
	log.G(ctx).WithFields(logrus.Fields{
		"pprof":  "/debug/pprof",
		"socket": debugPath,
	}).Debug("serve pprof")
	go serveProfiler(ctx, d)
	return nil
}
