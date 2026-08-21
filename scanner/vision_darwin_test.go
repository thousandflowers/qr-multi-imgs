//go:build darwin && cgo

package scanner

import "testing"

// Verifies the cgo bridge into Apple Vision round-trips a path to a payload.
// A clean QR is enough to exercise the marshalling; Vision's real value (warped
// real-world photos) can't be asserted deterministically from committed data.
func TestDecodeWithVision(t *testing.T) {
	const want = "https://example.com/vision-test"
	got := decodeWithVision("testdata/vision_qr.png")
	if got != want {
		t.Fatalf("decodeWithVision = %q, want %q", got, want)
	}
}

func TestDecodeWithVisionMissingFile(t *testing.T) {
	if got := decodeWithVision("testdata/does-not-exist.png"); got != "" {
		t.Fatalf("decodeWithVision on missing file = %q, want empty", got)
	}
}
