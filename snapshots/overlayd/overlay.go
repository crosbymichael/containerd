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

package overlayd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/snapshots"
	"golang.org/x/sys/unix"
)

type snapshotter struct {
	remote bool
	state  string
	source string
	ov     snapshots.Snapshotter
}

// NewSnapshotter returns a Snapshotter which uses overlayfs. The overlayfs
// diffs are stored under the provided root. A metadata file is stored under
// the root.
func NewSnapshotter(address, state string, ov snapshots.Snapshotter) (snapshots.Snapshotter, error) {
	if err := os.MkdirAll(state, 0755); err != nil {
		return nil, err
	}
	return &snapshotter{
		remote: address != "127.0.0.1",
		state:  state,
		source: address,
		ov:     ov,
	}, nil
}

// Stat returns the info for an active or committed snapshot by name or
// key.
//
// Should be used for parent resolution, existence checks and to discern
// the kind of snapshot.
func (o *snapshotter) Stat(ctx context.Context, key string) (snapshots.Info, error) {
	fmt.Println("stat")
	return o.ov.Stat(ctx, key)
}

func (o *snapshotter) Update(ctx context.Context, info snapshots.Info, fieldpaths ...string) (snapshots.Info, error) {
	return o.ov.Update(ctx, info, fieldpaths...)
}

// Usage returns the resources taken by the snapshot identified by key.
//
// For active snapshots, this will scan the usage of the overlay "diff" (aka
// "upper") directory and may take some time.
//
// For committed snapshots, the value is returned from the metadata database.
func (o *snapshotter) Usage(ctx context.Context, key string) (snapshots.Usage, error) {
	return o.ov.Usage(ctx, key)
}

func (o *snapshotter) Prepare(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	mounts, err := o.ov.Prepare(ctx, key, parent, opts...)
	if err != nil {
		return nil, err
	}
	return o.proxyMounts(key, mounts)
}

func (o *snapshotter) View(ctx context.Context, key, parent string, opts ...snapshots.Opt) ([]mount.Mount, error) {
	mounts, err := o.ov.View(ctx, key, parent, opts...)
	if err != nil {
		return nil, err
	}
	return o.proxyMounts(key, mounts)
}

// Mounts returns the mounts for the transaction identified by key. Can be
// called on an read-write or readonly transaction.
//
// This can be used to recover mounts after calling View or Prepare.
func (o *snapshotter) Mounts(ctx context.Context, key string) ([]mount.Mount, error) {
	mounts, err := o.ov.Mounts(ctx, key)
	if err != nil {
		return nil, err
	}
	return o.proxyMounts(key, mounts)
}

func (o *snapshotter) proxyMounts(key string, mounts []mount.Mount) ([]mount.Mount, error) {
	if o.remote {
		return mounts, nil
	}
	path := filepath.Join(o.state, key)
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}
	if _, err := mount.Lookup(path); err != nil {
		if err := mount.All(mounts, path); err != nil {
			return nil, err
		}
	}
	return []mount.Mount{
		{
			Type:   "9p",
			Source: o.source,
			Options: []string{
				"version=9p2000.L",
				"uname=root",
				"access=user",
				fmt.Sprintf("aname=%s", path),
			},
		},
	}, nil
}

func (o *snapshotter) Commit(ctx context.Context, name, key string, opts ...snapshots.Opt) error {
	return o.ov.Commit(ctx, name, key, opts...)
}

// Remove abandons the snapshot identified by key. The snapshot will
// immediately become unavailable and unrecoverable. Disk space will
// be freed up on the next call to `Cleanup`.
func (o *snapshotter) Remove(ctx context.Context, key string) (err error) {
	// remove any mounts if needed
	for {
		if err := unix.Unmount(filepath.Join(o.state, key), 0); err != nil {
			break
		}
	}
	return o.ov.Remove(ctx, key)
}

// Walk the committed snapshots.
func (o *snapshotter) Walk(ctx context.Context, fn func(context.Context, snapshots.Info) error) error {
	fmt.Println("walk")
	return o.ov.Walk(ctx, fn)
}

// Close closes the snapshotter
func (o *snapshotter) Close() error {
	return o.ov.Close()
}
