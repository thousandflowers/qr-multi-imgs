//go:build darwin && cgo

package scanner

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// Verifies the cgo bridge into Apple Vision round-trips a path to a payload.
// A clean QR is enough to exercise the marshalling; Vision's real value (warped
// real-world photos) can't be asserted deterministically from committed data.
func TestDecodeWithVision(t *testing.T) {
	const want = "https://example.com/vision-test"
	got := decodeWithVision("testdata/vision_qr.png")
	if len(got) != 1 || got[0].text != want {
		t.Fatalf("decodeWithVision = %v, want [%q]", hitTexts(got), want)
	}
	if !got[0].hasGeometry() {
		t.Error("decodeWithVision returned no points for an unrotated image")
	}
}

func TestDecodeWithVisionMissingFile(t *testing.T) {
	if got := decodeWithVision("testdata/does-not-exist.png"); len(got) != 0 {
		t.Fatalf("decodeWithVision on missing file = %v, want empty", hitTexts(got))
	}
}

// Vision must return every code in the frame, each with its own location.
func TestDecodeWithVisionFindsEveryCode(t *testing.T) {
	img := canvas(t, 900, 500, map[image.Point]image.Image{
		{X: 50, Y: 50}:  qrImage(t, "VISION-LEFT", 300),
		{X: 500, Y: 50}: qrImage(t, "VISION-RIGHT", 300),
	})
	path := filepath.Join(t.TempDir(), "two.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	got := decodeWithVision(path)
	if len(got) != 2 {
		t.Fatalf("decodeWithVision found %d codes (%v), want 2", len(got), hitTexts(got))
	}
	if got[0].text != "VISION-LEFT" || got[1].text != "VISION-RIGHT" {
		t.Errorf("decodeWithVision = %v, want [VISION-LEFT VISION-RIGHT]", hitTexts(got))
	}
	// Points must land in original-image pixels, not Vision's normalised or
	// internally-rescaled frame: the left code sits in the left half of a
	// 900px-wide image, the right code in the right half.
	lx, ly := got[0].centroid()
	rx, _ := got[1].centroid()
	if !(lx > 0 && lx < 450) {
		t.Errorf("left code centroid x = %.1f, want inside 0..450 of the original image", lx)
	}
	if !(rx > 450 && rx < 900) {
		t.Errorf("right code centroid x = %.1f, want inside 450..900 of the original image", rx)
	}
	if !(ly > 0 && ly < 500) {
		t.Errorf("left code centroid y = %.1f, want inside 0..500 of the original image", ly)
	}
}

// A garbled buffer must cost recall, not crash a scan.
func TestParseVisionBufferTruncated(t *testing.T) {
	full := []byte{}
	appendU32 := func(v uint32) {
		b := make([]byte, 4)
		visionByteOrder.PutUint32(b, v)
		full = append(full, b...)
	}
	appendU32(2) // claims two records
	appendU32(3) // first payload length
	full = append(full, "abc"...)
	appendU32(0)  // no points
	appendU32(99) // second record claims 99 bytes that are not there

	got := parseVisionBuffer(full)
	if len(got) != 1 || got[0].text != "abc" {
		t.Fatalf("parseVisionBuffer = %v, want the one intact record [abc]", hitTexts(got))
	}

	for n := range len(full) {
		parseVisionBuffer(full[:n]) // must not panic at any truncation
	}
	if got := parseVisionBuffer(nil); got != nil {
		t.Errorf("parseVisionBuffer(nil) = %v, want nil", got)
	}
}
