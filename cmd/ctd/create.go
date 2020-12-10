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
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/urfave/cli"
)

var createCommand = cli.Command{
	Name:  "create",
	Usage: "create a new container",
	Action: func(clix *cli.Context) error {
		var (
			id         = clix.Args().First()
			configPath = clix.Args().Get(1)
		)
		if id == "" {
			return ErrNoID
		}
		abs, err := filepath.Abs(id)
		if err != nil {
			return err
		}
		config := Config{
			id:   id,
			path: abs,
		}
		// create common directories for the container
		if err := os.Mkdir(config.path, 0755); err != nil {
			return err
		}
		if configPath != "" {
			if _, err := toml.DecodeFile(configPath, &config); err != nil {
				return err
			}
			config.id = id
			config.path = abs
			// bootstrap information from the config
			if config.Image != "" {
			}
		}
		f, err := os.Create(filepath.Join(config.id, ".ctd"))
		if err != nil {
			return err
		}
		defer f.Close()
		return config.Write(f)
	},
}
