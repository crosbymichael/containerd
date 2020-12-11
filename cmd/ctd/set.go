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
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var setCommand = cli.Command{
	Name:  "set",
	Usage: "set container configuration options",
	Action: func(clix *cli.Context) error {
		var (
			key   = clix.Args().First()
			args  = clix.Args().Tail()
			parts = strings.Split(key, ".")
			err   error
		)

		var config Config
		if _, err := toml.DecodeFile(".ctd", &config); err != nil {
			return err
		}
		switch parts[0] {
		case "cpu", "memory", "io", "pids":
			if config.Resources == nil {
				config.Resources = make(map[string]string)
			}
			a := args[0]
			if len(args) > 0 {
				a = strings.Join(args, " ")
			}
			config.Resources[key] = a
		default:
			return errors.Errorf("unknown key %s", key)
		}
		f, err := os.Create(".ctd")
		if err != nil {
			return err
		}
		defer f.Close()
		return config.Write(f)
	},
}
