package nvidia

import (
	"context"
	"os"
	"os/exec"
	"strconv"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type NVIDIACapability int

const (
	Compute NVIDIACapability = iota + 1
	Compat32
	Graphics
	Utility
	Video
	Display
)

var capFlags = map[NVIDIACapability]string{
	Compute:  "compute",
	Compat32: "compat32",
	Graphics: "graphics",
	Utility:  "utility",
	Video:    "video",
	Display:  "display",
}

func WithGPUs(device int, capabilities ...NVIDIACapability) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		path, err := exec.LookPath("containerd")
		if err != nil {
			return err
		}
		if s.Hooks == nil {
			s.Hooks = &specs.Hooks{}
		}
		s.Hooks.Prestart = append(s.Hooks.Prestart, specs.Hook{
			Path: path,
			Args: []string{
				"containerd",
				"nvidia",
				"--load-kmods",
				"--device", strconv.Itoa(device),
				"--caps", "utility",
			},
			Env: os.Environ(),
		})
		return nil
	}
}
