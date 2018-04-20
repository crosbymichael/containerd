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

package bundle

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const ConfigFilename = "config.json"

// Load loads an existing bundle from disk
func Load(id, path, workdir string) *Bundle {
	return &Bundle{
		ID:      id,
		Path:    path,
		WorkDir: workdir,
	}
}

// New creates a new bundle on disk at the provided path for the given id
func New(id, path, workDir string, spec []byte) (b *Bundle, err error) {
	if err := os.MkdirAll(path, 0711); err != nil {
		return nil, err
	}
	path = filepath.Join(path, id)
	defer func() {
		if err != nil {
			os.RemoveAll(path)
		}
	}()
	workDir = filepath.Join(workDir, id)
	if err := os.MkdirAll(workDir, 0711); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(workDir)
		}
	}()
	if err := os.Mkdir(path, 0711); err != nil {
		return nil, err
	}
	if err := os.Mkdir(filepath.Join(path, "rootfs"), 0711); err != nil {
		return nil, err
	}
	err = ioutil.WriteFile(filepath.Join(path, ConfigFilename), spec, 0666)
	return &Bundle{
		ID:      id,
		Path:    path,
		WorkDir: workDir,
	}, err
}

type Bundle struct {
	ID      string
	Path    string
	WorkDir string
}

// Delete deletes the bundle from disk
func (b *Bundle) Delete() error {
	err := os.RemoveAll(b.Path)
	if err == nil {
		return os.RemoveAll(b.WorkDir)
	}
	// error removing the bundle path; still attempt removing work dir
	err2 := os.RemoveAll(b.WorkDir)
	if err2 == nil {
		return err
	}
	return errors.Wrapf(err, "Failed to remove both bundle and workdir locations: %v", err2)
}
