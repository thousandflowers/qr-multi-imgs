//go:build !heic

package scanner

// Without -tags heic the raster path has no HEIC decoder, so image.Decode
// fails on those files. That is not the end of the scan: ScanImage remembers
// the failure and the path-based decoders still run, which is why HEIC works
// on macOS regardless — Apple Vision reads it from the path. See
// heic_enabled.go for why this is the default.
