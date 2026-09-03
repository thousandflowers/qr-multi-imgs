//go:build heic

package scanner

import (
	"image"
	"os"
	"testing"
)

// With -tags heic the raster path itself reads HEIC, which is what Linux and
// Windows need, macOS gets there through Vision either way. Asserting on
// image.Decode rather than ScanImage keeps this honest: on a macOS test run
// Vision would decode the file regardless and hide a broken raster path.
func TestRasterDecodesHEIC(t *testing.T) {
	f, err := os.Open("testdata/vision_qr.heic")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		t.Fatalf("image.Decode on HEIC with -tags heic: %v", err)
	}
	if format != "heic" {
		t.Errorf("format = %q, want heic", format)
	}
	if b := img.Bounds(); b.Dx() == 0 || b.Dy() == 0 {
		t.Fatal("decoded HEIC has no pixels")
	}

	hits, err := decodeRaster(img)
	if err != nil {
		t.Fatalf("decodeRaster: %v", err)
	}
	const want = "https://example.com/vision-test"
	got := hitTexts(hits)
	if len(got) != 1 || got[0] != want {
		t.Fatalf("decodeRaster on HEIC pixels = %v, want [%q]", got, want)
	}
}
