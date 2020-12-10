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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Network struct {
	Type        string   `toml:"type"`
	Mode        string   `toml:"mode"`
	Master      string   `toml:"master"`
	Addr        string   `toml:"addr"`
	Gateway     string   `toml:"gateway"`
	Nameservers []string `toml:"nameservers"`
}

func (n *Network) SetupFiles(id, path string) error {
	if err := n.writeHostname(id, path); err != nil {
		return err
	}
	if err := n.writeResolvconf(path); err != nil {
		return err
	}
	if err := n.writeHosts(id, path); err != nil {
		return err
	}
	return nil
}

func (n *Network) writeHostname(id, path string) error {
	f, err := os.Create(filepath.Join(path, "etc/hostname"))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(id)
	return err
}

func (n *Network) writeHosts(id, path string) error {
	f, err := os.Create(filepath.Join(path, "etc/hosts"))
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, `127.0.0.1       localhost localhost.localdomain %s
::1             localhost localhost.localdomain %s`, id, id); err != nil {
		return err
	}
	return nil

}

func (n *Network) writeResolvconf(path string) error {
	if len(n.Nameservers) == 0 {
		return nil
	}
	f, err := os.Create(filepath.Join(path, "etc/resolv.conf"))
	if err != nil {
		return err
	}
	defer f.Close()

	for _, ns := range n.Nameservers {
		if _, err := fmt.Fprintf(f, "nameserver %s\n", ns); err != nil {
			return err
		}
	}
	return nil
}

func (n *Network) Bytes(id string) []byte {
	c := &cni{
		Version: "0.4.0",
		Name:    "id",
		Plugins: []Plugin{
			{
				Type:   n.Type,
				Master: n.Master,
				Mode:   n.Mode,
				IPAM: IPAM{
					Type: "static",
					Addresses: []Address{
						{Address: n.Addr, Gateway: n.Gateway},
					},
					Routes: []Route{
						{Dst: "0.0.0.0/0", Gw: n.Gateway},
					},
				},
			},
		},
	}
	data, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return data
}

type cni struct {
	Version string   `json:"cniVersion"`
	Name    string   `json:"name"`
	Plugins []Plugin `json:"plugins"`
}

type Plugin struct {
	Type   string `json:"type"`
	Master string `json:"master"`
	Mode   string `json:"mode"`
	IPAM   IPAM   `json:"ipam"`
}

type Address struct {
	Address string `json:"address"`
	Gateway string `json:"gateway"`
}

type IPAM struct {
	Type      string    `json:"type"`
	Addresses []Address `json:"addresses"`
	Routes    []Route   `json:"routes"`
	DNS       DNS       `json:"dns"`
}

type Route struct {
	Dst string `json:"dst"`
	Gw  string `json:"gw"`
}

type DNS struct {
	Nameservers []string `json:"nameservers"`
	Domain      string   `json:"domain"`
	Search      []string `json:"search"`
}
