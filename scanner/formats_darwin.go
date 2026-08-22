//go:build darwin && cgo

package scanner

// The raster formats a Mac can read are not a fixed list — Core Image decodes
// every camera raw ImageIO knows, and that set grows with the OS and with each
// camera Apple adds support for. Hardcoding one would mean a stale list that
// silently skips a user's files, so the system is asked instead.

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework ImageIO -framework UniformTypeIdentifiers -framework Foundation
#include <stdlib.h>

// systemReadableExtensions returns a malloc'd, NUL-separated, double-NUL
// terminated list of lowercase filename extensions (no leading dot) that
// ImageIO can decode on this machine. Caller owns the buffer.
char* systemReadableExtensions(void);
*/
import "C"

import (
	"strings"
	"unsafe"
)

// systemExtensions asks ImageIO which raster formats this Mac decodes, returned
// lowercase and without a leading dot. Anything unexpected yields an empty
// list, leaving the portable set as the only source of truth.
func systemExtensions() []string {
	buf := C.systemReadableExtensions()
	if buf == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(buf))

	// The buffer ends in a double NUL, so scanning for it bounds the read
	// before any of the bytes are handed to Go.
	const maxLen = 1 << 16
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(buf)), maxLen)
	end := 0
	for end+1 < maxLen && !(bytes[end] == 0 && bytes[end+1] == 0) {
		end++
	}

	var out []string
	for _, ext := range strings.Split(string(bytes[:end]), "\x00") {
		if ext = strings.ToLower(strings.TrimSpace(ext)); ext != "" {
			out = append(out, ext)
		}
	}
	return out
}
