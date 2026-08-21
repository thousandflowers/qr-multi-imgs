package scanner

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// qrImage renders content as a QR of the given pixel size.
func qrImage(t *testing.T, content string, size int) image.Image {
	t.Helper()
	m, err := qrcode.NewQRCodeWriter().Encode(content, gozxing.BarcodeFormat_QR_CODE, size, size, nil)
	if err != nil {
		t.Fatalf("encode %q: %v", content, err)
	}
	w, h := m.GetWidth(), m.GetHeight()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := uint8(255)
			if m.Get(x, y) {
				v = 0
			}
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	return img
}

// canvas lays QRs out at given top-left offsets on a white background, with
// generous quiet zones so the detector has a fair chance at each one.
func canvas(t *testing.T, w, h int, place map[image.Point]image.Image) image.Image {
	t.Helper()
	dst := image.NewGray(image.Rect(0, 0, w, h))
	for i := range dst.Pix {
		dst.Pix[i] = 255
	}
	for at, src := range place {
		b := src.Bounds()
		for y := range b.Dy() {
			for x := range b.Dx() {
				g, _, _, _ := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
				dst.SetGray(at.X+x, at.Y+y, color.Gray{Y: uint8(g >> 8)})
			}
		}
	}
	return dst
}

func textsOf(hits []hit) []string { return hitTexts(hits) }

func TestDecodeRasterFindsEveryCode(t *testing.T) {
	img := canvas(t, 900, 500, map[image.Point]image.Image{
		{X: 50, Y: 50}:  qrImage(t, "LEFT-CODE", 300),
		{X: 500, Y: 50}: qrImage(t, "RIGHT-CODE", 300),
	})

	hits, err := decodeRaster(img)
	if err != nil {
		t.Fatalf("decodeRaster: %v", err)
	}
	got := textsOf(hits)
	if len(got) != 2 {
		t.Fatalf("decoded %d codes (%v), want 2", len(got), got)
	}
	// Reading order: same row, so left before right.
	if got[0] != "LEFT-CODE" || got[1] != "RIGHT-CODE" {
		t.Errorf("decodeRaster = %v, want [LEFT-CODE RIGHT-CODE]", got)
	}
}

// The case geometric dedup exists for: two physically distinct codes carrying
// identical payloads must stay two. Payload dedup would merge them into one.
func TestDecodeRasterKeepsIdenticalPayloadsApart(t *testing.T) {
	const same = "IDENTICAL-RECEIPT"
	img := canvas(t, 900, 500, map[image.Point]image.Image{
		{X: 50, Y: 50}:  qrImage(t, same, 300),
		{X: 500, Y: 50}: qrImage(t, same, 300),
	})

	hits, err := decodeRaster(img)
	if err != nil {
		t.Fatalf("decodeRaster: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("decoded %d codes, want 2 distinct codes with the same payload: %v", len(hits), textsOf(hits))
	}
	for _, h := range hits {
		if h.text != same {
			t.Errorf("payload = %q, want %q", h.text, same)
		}
	}
	// They must be at genuinely different places.
	ax, ay := hits[0].centroid()
	bx, by := hits[1].centroid()
	if ax == bx && ay == by {
		t.Error("both hits report the same centroid")
	}
}

// Ten strategies see the same code repeatedly; the union must report it once.
func TestDecodeRasterDedupesAcrossStrategies(t *testing.T) {
	img := canvas(t, 500, 500, map[image.Point]image.Image{
		{X: 100, Y: 100}: qrImage(t, "ONLY-ONE", 300),
	})
	hits, err := decodeRaster(img)
	if err != nil {
		t.Fatalf("decodeRaster: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("decoded %d codes (%v), want exactly 1", len(hits), textsOf(hits))
	}
}

func TestDecodeRasterOrderIsDeterministic(t *testing.T) {
	img := canvas(t, 900, 900, map[image.Point]image.Image{
		{X: 500, Y: 500}: qrImage(t, "bottom-right", 300),
		{X: 50, Y: 500}:  qrImage(t, "bottom-left", 300),
		{X: 500, Y: 50}:  qrImage(t, "top-right", 300),
		{X: 50, Y: 50}:   qrImage(t, "top-left", 300),
	})

	var first []string
	for i := range 3 {
		hits, err := decodeRaster(img)
		if err != nil {
			t.Fatalf("decodeRaster: %v", err)
		}
		got := textsOf(hits)
		if i == 0 {
			first = got
			continue
		}
		if len(got) != len(first) {
			t.Fatalf("run %d decoded %d codes, first run decoded %d", i, len(got), len(first))
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("run %d order = %v, first run = %v", i, got, first)
			}
		}
	}
	if len(first) == 4 {
		want := []string{"top-left", "top-right", "bottom-left", "bottom-right"}
		for j := range want {
			if first[j] != want[j] {
				t.Errorf("reading order = %v, want %v", first, want)
				break
			}
		}
	} else {
		t.Logf("decoded %d of 4 codes: %v", len(first), first)
	}
}

// ─── geometry unit tests ────────────────────────────────────────────────────

func pointsAt(cx, cy, size float64) []gozxing.ResultPoint {
	h := size / 2
	return []gozxing.ResultPoint{
		gozxing.NewResultPoint(cx-h, cy-h),
		gozxing.NewResultPoint(cx+h, cy-h),
		gozxing.NewResultPoint(cx-h, cy+h),
	}
}

func TestSameCodeTolerance(t *testing.T) {
	// size() is the diagonal of the point triangle, not the side.
	base := hit{text: "a", points: pointsAt(100, 100, 100)}
	size := base.size()

	tests := []struct {
		name   string
		offset float64
		want   bool
	}{
		{"identical position", 0, true},
		{"few pixels apart, same code re-found", 3, true},
		{"just inside tolerance", sameCodeFactor*size - 1, true},
		{"just outside tolerance", sameCodeFactor*size + 1, false},
		{"adjacent printed codes", size, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			other := hit{text: "a", points: pointsAt(100+tt.offset, 100, 100)}
			if got := sameCode(base, other); got != tt.want {
				t.Errorf("sameCode at offset %.1f (size %.1f) = %v, want %v", tt.offset, size, got, tt.want)
			}
		})
	}
}

// Distinct codes with equal payloads must not merge; that is the whole point.
func TestSameCodeIgnoresPayloadWhenGeometryIsPresent(t *testing.T) {
	a := hit{text: "same", points: pointsAt(100, 100, 100)}
	b := hit{text: "same", points: pointsAt(400, 100, 100)}
	if sameCode(a, b) {
		t.Error("two distant codes with equal payloads were merged")
	}
	c := hit{text: "different", points: pointsAt(100, 100, 100)}
	if !sameCode(a, c) {
		t.Error("one code re-found with a different payload was not merged")
	}
}

// A decoder that reports no position has nothing but its payload to go on.
func TestSameCodeFallsBackToPayloadWithoutGeometry(t *testing.T) {
	geo := hit{text: "x", points: pointsAt(100, 100, 100)}
	bare := hit{text: "x"}
	if !sameCode(geo, bare) {
		t.Error("geometry-free hit with a matching payload was not merged")
	}
	if sameCode(geo, hit{text: "y"}) {
		t.Error("geometry-free hit with a different payload was merged")
	}
}

func TestSortHitsPutsGeometryFreeLast(t *testing.T) {
	hits := []hit{
		{text: "bare-b"},
		{text: "placed", points: pointsAt(100, 100, 100)},
		{text: "bare-a"},
	}
	sortHits(hits)
	if hits[0].text != "placed" {
		t.Errorf("first = %q, want the located hit", hits[0].text)
	}
	if hits[1].text != "bare-a" || hits[2].text != "bare-b" {
		t.Errorf("geometry-free hits = %q,%q, want bare-a,bare-b", hits[1].text, hits[2].text)
	}
}

func TestMergeHitsKeepsFirstSighting(t *testing.T) {
	base := []hit{{text: "first", points: pointsAt(100, 100, 100)}}
	got := mergeHits(base, []hit{
		{text: "second", points: pointsAt(103, 100, 100)}, // same code, re-found
		{text: "elsewhere", points: pointsAt(500, 100, 100)},
	})
	if len(got) != 2 {
		t.Fatalf("merged to %d hits, want 2: %v", len(got), textsOf(got))
	}
	if got[0].text != "first" {
		t.Errorf("kept %q, want the first sighting", got[0].text)
	}
}

// End to end through the public path: a file holding two codes yields two
// payloads, under both modes. The modes only diverge when the raster path
// finds nothing, which is host-dependent and not asserted here.
func TestScanImageModeFindsEveryCode(t *testing.T) {
	img := canvas(t, 900, 500, map[image.Point]image.Image{
		{X: 50, Y: 50}:  qrImage(t, "FILE-LEFT", 300),
		{X: 500, Y: 50}: qrImage(t, "FILE-RIGHT", 300),
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

	for _, tc := range []struct {
		name string
		mode ScanMode
	}{{"fast", ScanFast}, {"exhaustive", ScanExhaustive}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ScanImageMode(path, tc.mode)
			if err != nil {
				t.Fatalf("ScanImageMode: %v", err)
			}
			if len(got) != 2 || got[0] != "FILE-LEFT" || got[1] != "FILE-RIGHT" {
				t.Errorf("ScanImageMode = %v, want [FILE-LEFT FILE-RIGHT]", got)
			}
		})
	}
}
