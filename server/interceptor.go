package server

import (
	containers "github.com/containerd/containerd/api/services/containers/v1"
	contentapi "github.com/containerd/containerd/api/services/content/v1"
	diff "github.com/containerd/containerd/api/services/diff/v1"
	eventsapi "github.com/containerd/containerd/api/services/events/v1"
	images "github.com/containerd/containerd/api/services/images/v1"
	namespaces "github.com/containerd/containerd/api/services/namespaces/v1"
	snapshotapi "github.com/containerd/containerd/api/services/snapshot/v1"
	tasks "github.com/containerd/containerd/api/services/tasks/v1"
	version "github.com/containerd/containerd/api/services/version/v1"
	"github.com/containerd/containerd/log"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"golang.org/x/net/context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type interceptContext struct {
	req      interface{}
	info     *grpc.UnaryServerInfo
	handler  grpc.UnaryHandler
	response interface{}
	err      error
}

type interceptor interface {
	intercept(ctx context.Context, ic *interceptContext) error
}

type multiInterceptor struct {
	interceptors []interceptor
}

func (m *multiInterceptor) intercept(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	ic := &interceptContext{
		req:     req,
		info:    info,
		handler: handler,
	}
	for _, i := range m.interceptors {
		if err := i.intercept(ctx, ic); err != nil {
			return nil, err
		}
	}
	return ic.response, ic.err
}

func newMultiInterceptor(interceptors ...interceptor) grpc.UnaryServerInterceptor {
	m := &multiInterceptor{
		interceptors: interceptors,
	}
	return grpc.UnaryServerInterceptor(m.intercept)
}

type moduleInterceptor struct {
}

func (m *moduleInterceptor) intercept(ctx context.Context, ic *interceptContext) error {
	ctx = log.WithModule(ctx, "containerd")
	switch ic.info.Server.(type) {
	case tasks.TasksServer:
		ctx = log.WithModule(ctx, "tasks")
	case containers.ContainersServer:
		ctx = log.WithModule(ctx, "containers")
	case contentapi.ContentServer:
		ctx = log.WithModule(ctx, "content")
	case images.ImagesServer:
		ctx = log.WithModule(ctx, "images")
	case grpc_health_v1.HealthServer:
		// No need to change the context
	case version.VersionServer:
		ctx = log.WithModule(ctx, "version")
	case snapshotapi.SnapshotsServer:
		ctx = log.WithModule(ctx, "snapshot")
	case diff.DiffServer:
		ctx = log.WithModule(ctx, "diff")
	case namespaces.NamespacesServer:
		ctx = log.WithModule(ctx, "namespaces")
	case eventsapi.EventsServer:
		ctx = log.WithModule(ctx, "events")
	default:
		log.G(ctx).Warnf("unknown GRPC server type: %#v\n", ic.info.Server)
	}
	return nil
}

type promInterceptor struct {
}

func (p *promInterceptor) intercept(ctx context.Context, ic *interceptContext) error {
	ic.response, ic.err = grpc_prometheus.UnaryServerInterceptor(ctx, ic.req, ic.info, ic.handler)
	return nil
}
