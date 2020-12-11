package main

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

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Layer interface {
	Init(context.Context, *Config) error
}

type CgroupLayer struct {
}

func (l *CgroupLayer) Init(ctx context.Context, config *Config) error {
	if err := l.create(config.id); err != nil {
		return err
	}
	for k, v := range config.Resources {
		if err := ioutil.WriteFile(l.path(config.id, k), []byte(v), 0); err != nil {
			return err
		}
	}
	return nil
}

func (l *CgroupLayer) create(id string) error {
	path := l.path(id)
	if err := os.Mkdir(path, 0755); err != nil {
		return err
	}
	return nil
}

func (l *CgroupLayer) path(args ...string) string {
	return filepath.Join(append([]string{"/sys/fs/cgroup"}, args...)...)
}
