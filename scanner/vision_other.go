//go:build !darwin || !cgo

package scanner

// decodeWithVision is a no-op off macOS: Apple Vision is unavailable, so the
// pure-Go gozxing path and the optional zbarimg fallback do all the work. It
// reads nothing, so it never reports a file as readable.
func decodeWithVision(string) ([]hit, bool) { return nil, false }
