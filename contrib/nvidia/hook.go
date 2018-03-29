package nvidia

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
)

const cliName = "nvidia-container-cli"

var Hook = cli.Command{
	Name:  "nvidia",
	Usage: "nvidia runc hook",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "load-kmods",
			Usage: "load kernel modules",
		},
		cli.StringFlag{
			Name:  "device",
			Usage: "device id",
		},
		cli.StringSliceFlag{
			Name:  "caps",
			Usage: "gpu capabilities",
			Value: &cli.StringSlice{},
		},
	},
	Action: func(context *cli.Context) error {
		state, err := loadHookState()
		if err != nil {
			return err
		}
		var args []string
		if context.Bool("load-kmods") {
			args = append(args, "--load-kmods")
		}
		args = append(args,
			"configure",
			fmt.Sprintf("--device=%s", context.String("device")),
		)
		for _, c := range context.StringSlice("caps") {
			args = append(args, fmt.Sprintf("--%s", c))
		}
		args = append(args,
			fmt.Sprintf("--pid=%d", state.Pid),
			filepath.Join(state.Bundle, "rootfs"),
		)
		cmd := exec.Command(cliName, args...)
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
		return cmd.Run()
	},
}

func loadHookState() (*specs.State, error) {
	var s specs.State
	if err := json.NewDecoder(os.Stdin).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}
