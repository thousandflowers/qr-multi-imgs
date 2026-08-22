//go:build !darwin || !cgo

package scanner

// Off macOS there is no system decoder to ask: the readable formats are exactly
// the ones Go's image package and the x/image codecs registered here can read,
// which the portable set already lists.
func systemExtensions() []string { return nil }
