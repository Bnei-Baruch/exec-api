//go:build darwin
// +build darwin

package wf

import (
	"os"
	"syscall"
)

// getCtime returns the creation time of a file on macOS
func getCtime(stat os.FileInfo) int64 {
	return stat.Sys().(*syscall.Stat_t).Ctimespec.Nsec
}
