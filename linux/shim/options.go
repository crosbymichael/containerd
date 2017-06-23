// +build !windows

package shim

import "encoding/json"

const (
	NoPivotRoot              = "io.runc.option.no-pivot-root"
	NoSubReaper              = "io.runc.option.no-subreaper"
	CheckpointExit           = "io.runc.option.checkpoint-exit"
	AllowOpenTCP             = "io.runc.option.allow-open-tcp"
	AllowExternalUnixSockets = "io.runc.option.allow-external-unix-sockets"
	AllowTerminal            = "io.runc.option.allow-terminal"
	FileLocks                = "io.runc.option.file-locks"
	EmptyNamespaces          = "io.runc.option.empty-namespaces"
)

func containsOption(options map[string]string, k string) bool {
	if options == nil {
		return false
	}
	_, ok := options[k]
	return ok
}

func getOptionList(options map[string]string, k string) []string {
	if options == nil {
		return nil
	}
	data, ok := options[k]
	if !ok {
		return nil
	}
	var out []string
	json.Unmarshal([]byte(data), &out)
	return out
}
