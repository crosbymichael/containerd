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

// Hook is the containerd hook for interfacing with nvidia's libnvidia-container
var Hook = cli.Command{
	Name:  "nvidia",
	Usage: "nvidia runc hook for gpu support",
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
		cli.StringSliceFlag{
			Name:  "requirements,r",
			Usage: "gpu requirements",
			Value: &cli.StringSlice{},
		},
		cli.StringFlag{
			Name:  "ld-cache",
			Usage: "set the ldcache",
		},
		cli.StringFlag{
			Name:  "ld-config",
			Usage: "set the ldconfig",
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
		if cache := context.String("ld-cache"); cache != "" {
			args = append(args, fmt.Sprintf("--ldcache=%s", cache))
		}
		args = append(args,
			"configure",
			fmt.Sprintf("--device=%s", context.String("device")),
		)
		for _, c := range context.StringSlice("caps") {
			args = append(args, fmt.Sprintf("--%s", c))
		}
		if config := context.String("ld-config"); config != "" {
			args = append(args, fmt.Sprintf("--ldconfig=%s", config))
		}
		for _, r := range context.StringSlice("requirements") {
			args = append(args, fmt.Sprintf("--require=%s", r))
		}
		args = append(args,
			fmt.Sprintf("--pid=%d", state.Pid),
			filepath.Join(state.Bundle, "rootfs"),
		)
		cmd := exec.Command(cliName, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
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
