package nvidia

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/containerd/containerd"
)

const cliName = "nvidia-container-cli"

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
	Compute:  "--compute",
	Compat32: "--compat32",
	Graphics: "--graphics",
	Utility:  "--utility",
	Video:    "--video",
	Display:  "--display",
}

func AttachGPUs(ctx context.Context, task containerd.Task, device int, capabilities []NVIDIACapability) error {
	args, err := buildArgs(device, capabilities)
	if err != nil {
		return err
	}
	status, err := task.Status(ctx)
	if err != nil {
		return err
	}
	args = append(args,
		fmt.Sprintf("--pid=%d", task.Pid()),
		filepath.Join(status.BundlePath, "rootfs"),
	)
	cmd := exec.Command(cliName, args...)
	cmd.Dir = status.BundlePath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}

func buildArgs(device int, capabilities []NVIDIACapability) ([]string, error) {
	args := []string{
		"--load-kmods",
		"configure",
		fmt.Sprintf("--device=%d", device),
	}
	for _, c := range capabilities {
		f, ok := capFlags[c]
		if !ok {
			return nil, fmt.Errorf("unknown driver capability %d", c)
		}
		args = append(args, f)
	}
	// requirements
	return args, nil
}
