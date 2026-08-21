//go:build darwin && cgo

package scanner

// macOS-only QR backend built on Apple's Vision framework. Vision decodes
// real-world photos — perspective-warped, low-contrast, small-module receipt
// QRs — that the pure-Go gozxing path and zbarimg both miss. The framework
// ships with every macOS install, so this adds no runtime dependency for the
// user (unlike zbarimg, which needs `brew install zbar` and can segfault).

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Vision -framework CoreImage -framework ImageIO -framework CoreGraphics -framework Foundation
#include <stdlib.h>

// decodeQRVision returns the first QR payload found in the image at cpath, or
// NULL if none. Caller owns the returned C string and must free() it.
char* decodeQRVision(const char* cpath);
*/
import "C"

import "unsafe"

// decodeWithVision runs Apple Vision's QR detector on the original file. It
// loads from disk directly so Vision sees full resolution and the embedded
// EXIF orientation, both of which matter for marginal real-world QRs.
func decodeWithVision(path string) string {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	res := C.decodeQRVision(cpath)
	if res == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(res))
	return C.GoString(res)
}
