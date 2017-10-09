package server

import (
	"errors"
	"expvar"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/snapshot"
	metrics "github.com/docker/go-metrics"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"golang.org/x/net/context"

	"google.golang.org/grpc"
)

// New creates and initializes a new containerd server
func New(ctx context.Context, config *Config) (*Server, error) {
	if config.Root == "" {
		return nil, errors.New("root must be specified")
	}
	if config.State == "" {
		return nil, errors.New("state must be specified")
	}
	if err := os.MkdirAll(config.Root, 0711); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(config.State, 0711); err != nil {
		return nil, err
	}
	if err := apply(ctx, config); err != nil {
		return nil, err
	}
	plugins, err := loadPlugins(config)
	if err != nil {
		return nil, err
	}
	interceptors := []interceptor{
		&moduleInterceptor{},
		&promInterceptor{},
	}
	rpc := grpc.NewServer(
		grpc.UnaryInterceptor(newMultiInterceptor(interceptors...)),
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
	)
	var (
		services []plugin.Service
		s        = &Server{
			rpc:    rpc,
			events: events.NewExchange(),
		}
		initialized = make(map[plugin.Type]map[string]interface{})
	)
	for _, p := range plugins {
		id := p.URI()
		log.G(ctx).WithField("type", p.Type).Infof("loading plugin %q...", id)

		initContext := plugin.NewContext(
			ctx,
			initialized,
			config.Root,
			config.State,
			id,
		)
		initContext.Events = s.events
		initContext.Address = config.GRPC.Address

		// load the plugin specific configuration if it is provided
		if p.Config != nil {
			pluginConfig, err := config.Decode(p.ID, p.Config)
			if err != nil {
				return nil, err
			}
			initContext.Config = pluginConfig
		}
		instance, err := p.Init(initContext)
		if err != nil {
			if plugin.IsSkipPlugin(err) {
				log.G(ctx).WithField("type", p.Type).Infof("skip loading plugin %q...", id)
			} else {
				log.G(ctx).WithError(err).Warnf("failed to load plugin %s", id)
			}
			continue
		}

		if types, ok := initialized[p.Type]; ok {
			types[p.ID] = instance
		} else {
			initialized[p.Type] = map[string]interface{}{
				p.ID: instance,
			}
		}
		// check for grpc services that should be registered with the server
		if service, ok := instance.(plugin.Service); ok {
			services = append(services, service)
		}
	}
	// register services after all plugins have been initialized
	for _, service := range services {
		if err := service.Register(rpc); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Server is the containerd main daemon
type Server struct {
	rpc    *grpc.Server
	events *events.Exchange
}

// ServeGRPC provides the containerd grpc APIs on the provided listener
func (s *Server) ServeGRPC(l net.Listener) error {
	// before we start serving the grpc API regster the grpc_prometheus metrics
	// handler.  This needs to be the last service registered so that it can collect
	// metrics for every other service
	grpc_prometheus.Register(s.rpc)
	return trapClosedConnErr(s.rpc.Serve(l))
}

// ServeMetrics provides a prometheus endpoint for exposing metrics
func (s *Server) ServeMetrics(l net.Listener) error {
	m := http.NewServeMux()
	m.Handle("/v1/metrics", metrics.Handler())
	return trapClosedConnErr(http.Serve(l, m))
}

// ServeDebug provides a debug endpoint
func (s *Server) ServeDebug(l net.Listener) error {
	// don't use the default http server mux to make sure nothing gets registered
	// that we don't want to expose via containerd
	m := http.NewServeMux()
	m.Handle("/debug/vars", expvar.Handler())
	m.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	m.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	m.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	m.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	m.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	return trapClosedConnErr(http.Serve(l, m))
}

// Stop the containerd server canceling any open connections
func (s *Server) Stop() {
	s.rpc.Stop()
}

func loadPlugins(config *Config) ([]*plugin.Registration, error) {
	// load all plugins into containerd
	if err := plugin.Load(filepath.Join(config.Root, "plugins")); err != nil {
		return nil, err
	}
	// load additional plugins that don't automatically register themselves
	plugin.Register(&plugin.Registration{
		Type: plugin.ContentPlugin,
		ID:   "content",
		Init: func(ic *plugin.InitContext) (interface{}, error) {
			return local.NewStore(ic.Root)
		},
	})
	plugin.Register(&plugin.Registration{
		Type: plugin.MetadataPlugin,
		ID:   "bolt",
		Requires: []plugin.Type{
			plugin.ContentPlugin,
			plugin.SnapshotPlugin,
		},
		Init: func(ic *plugin.InitContext) (interface{}, error) {
			if err := os.MkdirAll(ic.Root, 0711); err != nil {
				return nil, err
			}
			cs, err := ic.Get(plugin.ContentPlugin)
			if err != nil {
				return nil, err
			}

			rawSnapshotters, err := ic.GetAll(plugin.SnapshotPlugin)
			if err != nil {
				return nil, err
			}

			snapshotters := make(map[string]snapshot.Snapshotter)
			for name, sn := range rawSnapshotters {
				snapshotters[name] = sn.(snapshot.Snapshotter)
			}

			db, err := bolt.Open(filepath.Join(ic.Root, "meta.db"), 0644, nil)
			if err != nil {
				return nil, err
			}
			mdb := metadata.NewDB(db, cs.(content.Store), snapshotters)
			if err := mdb.Init(ic.Context); err != nil {
				return nil, err
			}
			return mdb, nil
		},
	})
	// return the ordered graph for plugins
	return plugin.Graph(), nil
}

func trapClosedConnErr(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "use of closed network connection") {
		return nil
	}
	return err
}
