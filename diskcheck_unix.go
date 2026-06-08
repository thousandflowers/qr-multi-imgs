//go:build !windows

package main

import (
	"fmt"
	"syscall"

	"github.com/thousandflowers/qr-multi-imgs/scanner"
)

// hasDiskSpace checks that at least `needed` bytes are available on the filesystem.
func hasDiskSpace(path string, needed int64) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil
	}
	available := int64(stat.Bavail) * int64(stat.Bsize)
	if available < needed {
		return fmt.Errorf(
			"insufficient disk space: need ~%s, only %s available",
			scanner.FormatBytes(needed), scanner.FormatBytes(available),
		)
	}
	return nil
}
