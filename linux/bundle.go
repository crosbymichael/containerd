// +build linux

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package linux

import (
	"context"
	"path/filepath"

	"github.com/containerd/containerd/events/exchange"
	"github.com/containerd/containerd/linux/runctypes"
	"github.com/containerd/containerd/linux/shim"
	"github.com/containerd/containerd/linux/shim/client"
	"github.com/containerd/containerd/runtime/bundle"
)

// ShimOpt specifies shim options for initialization and connection
type ShimOpt func(*bundle.Bundle, string, *runctypes.RuncOptions) (shim.Config, client.Opt)

// ShimRemote is a ShimOpt for connecting and starting a remote shim
func ShimRemote(c *Config, daemonAddress, cgroup string, exitHandler func()) ShimOpt {
	return func(b *bundle.Bundle, ns string, ropts *runctypes.RuncOptions) (shim.Config, client.Opt) {
		config := shimConfig(ns, b, c, ropts)
		return config,
			client.WithStart(c.Shim, shimAddress(ns, b), daemonAddress, cgroup, c.ShimDebug, exitHandler)
	}
}

// ShimLocal is a ShimOpt for using an in process shim implementation
func ShimLocal(c *Config, exchange *exchange.Exchange) ShimOpt {
	return func(b *bundle.Bundle, ns string, ropts *runctypes.RuncOptions) (shim.Config, client.Opt) {
		return shimConfig(ns, b, c, ropts), client.WithLocal(exchange)
	}
}

// ShimConnect is a ShimOpt for connecting to an existing remote shim
func ShimConnect(c *Config, onClose func()) ShimOpt {
	return func(b *bundle.Bundle, ns string, ropts *runctypes.RuncOptions) (shim.Config, client.Opt) {
		return shimConfig(ns, b, c, ropts), client.WithConnect(shimAddress(ns, b), onClose)
	}
}

// NewShimClient connects to the shim managing the bundle and tasks creating it if needed
func NewShimClient(ctx context.Context, namespace string, b *bundle.Bundle, getClientOpts ShimOpt, runcOpts *runctypes.RuncOptions) (*client.Client, error) {
	cfg, opt := getClientOpts(b, namespace, runcOpts)
	return client.New(ctx, cfg, opt)
}

func shimAddress(namespace string, b *bundle.Bundle) string {
	return filepath.Join(string(filepath.Separator), "containerd-shim", namespace, b.ID, "shim.sock")
}

func shimConfig(namespace string, b *bundle.Bundle, c *Config, runcOptions *runctypes.RuncOptions) shim.Config {
	var (
		criuPath      string
		runtimeRoot   = c.RuntimeRoot
		systemdCgroup bool
	)
	if runcOptions != nil {
		criuPath = runcOptions.CriuPath
		systemdCgroup = runcOptions.SystemdCgroup
		if runcOptions.RuntimeRoot != "" {
			runtimeRoot = runcOptions.RuntimeRoot
		}
	}
	return shim.Config{
		Path:          b.Path,
		WorkDir:       b.WorkDir,
		Namespace:     namespace,
		Criu:          criuPath,
		RuntimeRoot:   runtimeRoot,
		SystemdCgroup: systemdCgroup,
	}
}
