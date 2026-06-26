//go:build !darwin

package scanner

// decodeWithVision is a no-op off macOS: Apple Vision is unavailable, so the
// pure-Go gozxing path and the optional zbarimg fallback do all the work.
func decodeWithVision(string) string { return "" }
