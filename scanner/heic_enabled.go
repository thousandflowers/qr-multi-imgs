//go:build heic

package scanner

// Optional pure-Go HEIC/HEIF decoding for the raster path, off unless built
// with -tags heic.
//
// WHY IT IS OPT-IN, AND WHY RELEASED BINARIES DO NOT CARRY IT
//
// gen2brain/heic is MIT, but the MIT covers the Go wrapper. What it embeds is
// a WASM build of libheif, which is LGPL-3.0. Whether an MIT wrapper discharges
// an LGPL blob's relinking obligation is a question for a lawyer, not for a
// build file, so the release does not ask it. With the tag off, the goreleaser
// binaries and the Homebrew and npm artifacts contain no libheif at all, and
// the obligation never attaches. A go.mod reference is not binary distribution.
//
// It is also never built for GOOS=js. It does run there, wazero interprets the
// embedded WASM inside the host's WASM, but at roughly 4.5 minutes per 12MP
// photo against 0.1s native, measured. A browser build should hand pixels to
// ScanDecodedImage from createImageBitmap instead, which needs no HEIC decoder
// in Go at all.
//
// Upstream ships no NOTICE naming libheif's licence. Asked about it at
// https://github.com/gen2brain/heic/issues/17, if that comes back saying the
// embedded blob is safe to redistribute, this can become a default build.
//
// Cost when enabled: about +4.65 MB of binary, almost all of it the wazero
// runtime rather than the 138 KB compressed codec.
//
// On macOS none of this is needed: Apple Vision reads HEIC straight from the
// path, so HEIC works in the default build.

import _ "github.com/gen2brain/heic"

// heicRaster reports whether image.Decode in this build can read HEIC/HEIF.
const heicRaster = true
