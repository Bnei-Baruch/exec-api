//go:build linux
// +build linux

package wf

import (
	"os"
	"syscall"
)

// getCtime returns the creation time of a file on Linux
func getCtime(stat os.FileInfo) int64 {
	return stat.Sys().(*syscall.Stat_t).Ctim.Nsec
}
