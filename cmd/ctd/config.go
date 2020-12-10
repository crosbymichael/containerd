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

package main

import (
	"context"
	"io"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
)

const maxUID = 65536

type Root struct {
	Device string `toml:"device"`
	Key    string `toml:"key"`
}

type Volume struct {
	Path string `toml:"path"`
	RO   bool   `toml:"ro"`
}

type Mount struct {
	Source  string   `toml:"source"`
	Dest    string   `toml:"dest"`
	Type    string   `toml:"type"`
	Options []string `toml:"options"`
}

type CPU struct {
	Max   float64 `toml:"max"`
	Nodes []int   `toml:"nodes"`
	Cores []int   `toml:"cores"`
}

type Memory struct {
	Max int `toml:"max"`
}

type Process struct {
	NoFile   int      `toml:"no_file"`
	Pids     int      `toml:"pids"`
	Args     []string `toml:"args"`
	Env      []string `toml:"env"`
	UID      *uint32  `toml:"uid"`
	GID      *uint32  `toml:"gid"`
	Terminal bool     `toml:"terminal"`
}

type IO struct {
	IOPS    int `toml:"iops"`
	ReadBW  int `toml:"rbw"`
	WriteBW int `toml:"wbw"`
}

type Config struct {
	id   string
	path string

	Image   string            `toml:"image"`
	Root    *Root             `toml:"root"`
	Volumes map[string]Volume `toml:"volumes"`
	Mounts  []Mount           `toml:"mount"`
	CPU     *CPU              `toml:"cpu"`
	Memory  *Memory           `toml:"memory"`
	Process *Process          `toml:"process"`
	IO      *IO               `toml:"io"`
	Network *Network          `toml:"network"`
}

func (c *Config) rootfs() string {
	return filepath.Join(c.path, "rootfs")
}

// Write the config
func (c *Config) Write(w io.Writer) error {
	return toml.NewEncoder(w).Encode(c)
}

func (c *Config) Spec(ctx context.Context, mounts []specs.Mount) (*oci.Spec, error) {
	opts := []oci.SpecOpts{
		oci.WithRootFSPath(c.rootfs()),
		oci.WithHostname(c.id),
		oci.WithDefaultPathEnv,
	}
	//	if i != nil {
	//		opts = append(opts, oci.WithImageConfigArgs(i, c.Process.Args))
	//	} else {
	opts = append(opts, oci.WithProcessArgs(c.Process.Args...))
	//	}
	opts = append(opts, oci.WithCgroup("/"+c.id))

	if c.Process.Terminal {
		opts = append(opts, oci.WithTTY)
	}
	//if c.Cgroups {
	mounts = append(mounts, specs.Mount{
		Type:        "cgroup",
		Source:      "none",
		Destination: "/sys/fs/cgroup",
	})
	opts = append(opts, oci.WithLinuxNamespace(specs.LinuxNamespace{
		Type: specs.CgroupNamespace,
	}))
	//}

	if len(mounts) > 0 {
		opts = append(opts, oci.WithMounts(mounts))
	}

	opts = append(opts, oci.WithLinuxNamespace(specs.LinuxNamespace{
		Type: specs.NetworkNamespace,
	}))

	var remap uint32
	// if c.Profile == "default" || c.Profile == "" {
	remap = 1000
	mapping := []specs.LinuxIDMapping{
		{
			ContainerID: 0,
			HostID:      remap,
			Size:        maxUID,
		},
	}
	opts = append(opts, oci.WithUserNamespace(mapping, mapping))
	// }
	if len(c.Process.Env) > 0 {
		opts = append(opts, oci.WithEnv(c.Process.Env))
	}
	// if config.NewPrivs {
	//	opts = append(opts, oci.WithNewPrivileges)
	// }
	if c.Process.NoFile > 0 {
		opts = append(opts, withNoFile(c.Process.NoFile))
	}
	//	if len(config.Caps) > 0 {
	//		opts = append(opts, oci.WithAddedCapabilities(config.Caps))
	//	}
	if c.Process.UID != nil {
		uid := *c.Process.UID
		gid := uint32(0)
		if c.Process.GID != nil {
			gid = *c.Process.GID
		}
		opts = append(opts, oci.WithUIDGID(uid, gid))
	}
	// reset rootfs after all the other opts are run
	opts = append(opts, oci.WithRootFSPath("rootfs"))
	return oci.GenerateSpec(ctx, nil, &containers.Container{
		ID: c.id,
	}, opts...)
}

func toStrings(ii []int) (o []string) {
	for _, i := range ii {
		o = append(o, strconv.Itoa(i))
	}
	return o
}
