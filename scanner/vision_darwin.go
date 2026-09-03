//go:build darwin && cgo

package scanner

// macOS-only QR backend built on Apple's Vision framework. Vision decodes
// real-world photos, perspective-warped, low-contrast, small-module receipt
// QRs, that the pure-Go gozxing path and zbarimg both miss. The framework
// ships with every macOS install, so this adds no runtime dependency for the
// user (unlike zbarimg, which needs `brew install zbar` and can segfault).

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Vision -framework CoreImage -framework ImageIO -framework CoreGraphics -framework Foundation
#include <stdlib.h>

// decodeQRVisionAll returns a malloc'd, length-prefixed buffer describing every
// QR found in the image at cpath, writing its length to outLen. It returns NULL
// only when the file could not be read at all; a readable image with no QR in
// it yields a valid, empty list. Caller owns the buffer and must free() it.
char* decodeQRVisionAll(const char* cpath, int* outLen);
*/
import "C"

import (
	"encoding/binary"
	"math"
	"unsafe"

	"github.com/makiuchi-d/gozxing"
)

// visionByteOrder matches the C side, which writes machine words directly.
// The buffer never leaves this process, so host order is the whole contract.
var visionByteOrder = binary.NativeEndian

// decodeWithVision runs Apple Vision's QR detector on the original file and
// returns every code it found. It loads from disk directly so Vision sees full
// resolution and the embedded EXIF orientation, both of which matter for
// marginal real-world QRs.
//
// Points arrive already mapped to original-image pixels, honouring hit's
// coordinate contract. They are absent when an EXIF orientation was applied:
// Vision then works in an upright frame that Go's image.Decode never sees, and
// emitting points in a frame the caller cannot reconcile would make dedup
// report one code as two. Payload matching covers those instead.
// The second return reports whether Vision could read the file at all. Callers
// use it to tell "this decoder could not open it" apart from "it opened fine
// and holds no QR", which matters because Core Image reads formats Go's
// image.Decode does not, HEIC above all.
func decodeWithVision(path string) ([]hit, bool) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	var clen C.int
	buf := C.decodeQRVisionAll(cpath, &clen)
	if buf == nil {
		return nil, false
	}
	defer C.free(unsafe.Pointer(buf))
	if clen <= 0 {
		return nil, true
	}
	// CIDetector and Vision are both run over each variant and routinely find
	// the same code, so the raw records hold duplicates. Deduping here means
	// this returns a clean set like decodeRaster does, rather than leaving it
	// to whichever caller happens to merge it.
	hits := mergeHits(nil, parseVisionBuffer(C.GoBytes(unsafe.Pointer(buf), clen)))
	sortHits(hits)
	return hits, true
}

// parseVisionBuffer decodes the wire format written by serialize() in
// vision_darwin.m:
//
//	uint32 count
//	per record: uint32 textLen, textLen bytes,
//	            uint32 pointCount, pointCount * (double x, double y)
//
// A malformed buffer yields whatever was read before the damage rather than a
// panic: this is a fallback decoder, and a garbled record should cost recall,
// not the whole scan.
func parseVisionBuffer(b []byte) []hit {
	pos := 0
	readU32 := func() (uint32, bool) {
		if pos+4 > len(b) {
			return 0, false
		}
		v := visionByteOrder.Uint32(b[pos : pos+4])
		pos += 4
		return v, true
	}
	readF64 := func() (float64, bool) {
		if pos+8 > len(b) {
			return 0, false
		}
		v := math.Float64frombits(visionByteOrder.Uint64(b[pos : pos+8]))
		pos += 8
		return v, true
	}

	count, ok := readU32()
	if !ok {
		return nil
	}

	hits := make([]hit, 0, count)
	for range count {
		textLen, ok := readU32()
		if !ok || pos+int(textLen) > len(b) {
			return hits
		}
		text := string(b[pos : pos+int(textLen)])
		pos += int(textLen)

		nPoints, ok := readU32()
		if !ok {
			return hits
		}
		points := make([]gozxing.ResultPoint, 0, nPoints)
		for range nPoints {
			x, okX := readF64()
			y, okY := readF64()
			if !okX || !okY {
				return hits
			}
			points = append(points, gozxing.NewResultPoint(x, y))
		}
		if text == "" {
			continue
		}
		hits = append(hits, hit{text: text, points: points})
	}
	return hits
}

// visionAvailable reports whether Apple Vision can read files in this build.
const visionAvailable = true
