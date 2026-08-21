package scanner

// Covers the path-free decode seam. ScanImage's own tests already exercise
// decodeRaster indirectly; these assert the exported entry point works from
// pixels alone, which is what a wasm caller has.

import (
	"image"
	"os"
	"testing"
)

func TestScanDecodedImage(t *testing.T) {
	const want = "https://example.com/vision-test"

	f, err := os.Open("testdata/vision_qr.png")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	got, err := ScanDecodedImage(img)
	if err != nil {
		t.Fatalf("ScanDecodedImage error: %v", err)
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("ScanDecodedImage = %v, want [%q]", got, want)
	}
}

func TestScanDecodedImageNoQR(t *testing.T) {
	blank := image.NewGray(image.Rect(0, 0, 64, 64))
	for i := range blank.Pix {
		blank.Pix[i] = 200
	}

	got, err := ScanDecodedImage(blank)
	if err != nil {
		t.Fatalf("ScanDecodedImage error on blank image: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ScanDecodedImage on blank = %v, want none", got)
	}
}
