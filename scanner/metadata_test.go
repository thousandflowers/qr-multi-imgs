package scanner

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing/qrcode/decoder"
	"github.com/makiuchi-d/gozxing/qrcode/encoder"
)

// flatGray is an image with no edges anywhere: the floor both measures must
// report.
func flatGray(w, h int, v uint8) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = v
	}
	return img
}

// checkerGray alternates black and white every `cell` pixels, which is the
// densest edge pattern an 8-bit image can hold at that scale.
func checkerGray(w, h, cell int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := uint8(0)
			if (x/cell+y/cell)%2 == 0 {
				v = 255
			}
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	return img
}

func TestMeasureFlatImageHasNoEdges(t *testing.T) {
	lap, edge := measure(flatGray(128, 128, 200))
	if lap != 0 {
		t.Errorf("laplacian variance on a flat image = %v, want 0", lap)
	}
	if edge != 0 {
		t.Errorf("edge density on a flat image = %v, want 0", edge)
	}
}

func TestMeasureRanksSharpAboveBlurred(t *testing.T) {
	sharp := checkerGray(128, 128, 8)
	// A box blur over the same pattern: the same content, softened, which is
	// exactly the comparison the number exists to make.
	blurred := boxBlur(sharp, 3)

	sharpLap, _ := measure(sharp)
	blurLap, _ := measure(blurred)
	if !(sharpLap > blurLap) {
		t.Errorf("laplacian variance: sharp %v, blurred %v — sharp must score higher", sharpLap, blurLap)
	}
}

// TestEdgeDensityIsLocal is the property the field is named for: a busy patch
// in a corner of an otherwise empty image must register. An image-wide average
// would divide it away, which is the implementation this test rejects.
func TestEdgeDensityIsLocal(t *testing.T) {
	const size = 512
	img := flatGray(size, size, 255)
	patch := checkerGray(edgeWindow, edgeWindow, 2)
	for y := range edgeWindow {
		for x := range edgeWindow {
			img.SetGray(x, y, color.Gray{Y: patch.GrayAt(x, y).Y})
		}
	}

	_, local := measure(img)
	_, flat := measure(flatGray(size, size, 255))
	if !(local > flat) {
		t.Fatalf("edge density with a busy corner = %v, flat = %v — the patch must register", local, flat)
	}

	// And it must not be diluted to near-nothing by the empty 98% around it.
	_, everywhere := measure(checkerGray(size, size, 2))
	if local < everywhere/2 {
		t.Errorf("edge density: one busy block scored %v against %v when the whole image is busy; "+
			"that is an image-wide average, not a local one", local, everywhere)
	}
}

func TestEdgeDensityStaysInRange(t *testing.T) {
	for _, img := range []image.Image{
		checkerGray(128, 128, 1),
		checkerGray(128, 128, 2),
		flatGray(128, 128, 0),
	} {
		if _, edge := measure(img); edge < 0 || edge > 1 {
			t.Errorf("edge density %v outside 0-1", edge)
		}
	}
}

func TestMeasureTinyImage(t *testing.T) {
	// A 3x3 kernel has one valid position on a 3x3 image and none on 2x2.
	// Neither may panic or report a number it did not compute.
	for _, d := range []int{1, 2, 3} {
		lap, edge := measure(flatGray(d, d, 128))
		if lap != 0 || edge != 0 {
			t.Errorf("%dx%d flat image measured %v/%v, want 0/0", d, d, lap, edge)
		}
	}
}

// TestGrayOfAgreesAcrossImageTypes pins the fast paths against the generic
// one. They exist for speed and would otherwise be free to disagree quietly.
func TestGrayOfAgreesAcrossImageTypes(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 33, 17))
	for y := range 17 {
		for x := range 33 {
			src.Set(x, y, color.RGBA{R: uint8(x * 7), G: uint8(y * 13), B: uint8(x + y), A: 255})
		}
	}

	fast := grayOf(src)
	slow := grayOf(genericImage{src})
	for i := range fast.Pix {
		if d := int(fast.Pix[i]) - int(slow.Pix[i]); d < -1 || d > 1 {
			t.Fatalf("pixel %d: fast path %d, generic path %d", i, fast.Pix[i], slow.Pix[i])
		}
	}
}

// genericImage hides the concrete type so grayOf takes its At() fallback.
type genericImage struct{ img image.Image }

func (g genericImage) ColorModel() color.Model { return g.img.ColorModel() }
func (g genericImage) Bounds() image.Rectangle { return g.img.Bounds() }
func (g genericImage) At(x, y int) color.Color { return g.img.At(x, y) }

func boxBlur(src *image.Gray, r int) *image.Gray {
	b := src.Bounds()
	dst := image.NewGray(b)
	for y := range b.Dy() {
		for x := range b.Dx() {
			var sum, n int
			for dy := -r; dy <= r; dy++ {
				for dx := -r; dx <= r; dx++ {
					px, py := x+dx, y+dy
					if px < 0 || py < 0 || px >= b.Dx() || py >= b.Dy() {
						continue
					}
					sum += int(src.GrayAt(px, py).Y)
					n++
				}
			}
			dst.SetGray(x, y, color.Gray{Y: uint8(sum / n)})
		}
	}
	return dst
}

func TestStrategyNamesAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range strategies {
		n := s.name()
		if seen[n] {
			t.Errorf("two strategies share the name %q; the benchmark cannot attribute a hit", n)
		}
		seen[n] = true
		if strings.Count(n, "/") != 2 {
			t.Errorf("strategy name %q is not channel/binarizer/scale", n)
		}
	}
	if got := (decodeStrategy{channel: "lum", global: true, scale: 2}).name(); got != "lum/global/2x" {
		t.Errorf("name() = %q, want lum/global/2x", got)
	}
	if got := (decodeStrategy{channel: "r", scale: 1}).name(); got != "r/hybrid/1x" {
		t.Errorf("name() = %q, want r/hybrid/1x", got)
	}
	// A preprocessing rung is named for its transform, and a rung without one
	// keeps the name it had before transforms existed so old measurements still
	// line up.
	if got := (decodeStrategy{channel: "lum", global: true, scale: 1, prep: "sauvola"}).name(); got != "lum+sauvola/global/1x" {
		t.Errorf("name() = %q, want lum+sauvola/global/1x", got)
	}
}

// TestStrategyPrepsExist stops a table row naming a transform that is not
// registered. decodeAttempt skips such a strategy rather than panicking, which
// means the failure would otherwise show up as a quiet loss of recall.
func TestStrategyPrepsExist(t *testing.T) {
	for _, s := range strategies {
		if s.prep == "" {
			continue
		}
		if _, ok := transforms[s.prep]; !ok {
			t.Errorf("strategy %q names transform %q, which is not in the registry", s.name(), s.prep)
		}
	}
}

func TestScanDecodedImageDetailRecordsAttempts(t *testing.T) {
	img := qrImage(t, "https://example.com/metadata", 256)
	d, err := ScanDecodedImageDetail(img)
	if err != nil {
		t.Fatalf("ScanDecodedImageDetail: %v", err)
	}
	m := d.Metadata
	if m == nil {
		t.Fatal("no metadata on a decoded image")
	}
	if m.Width != img.Bounds().Dx() || m.Height != img.Bounds().Dy() {
		t.Errorf("dimensions %dx%d, want %dx%d", m.Width, m.Height, img.Bounds().Dx(), img.Bounds().Dy())
	}
	if len(m.Strategies) == 0 {
		t.Fatal("no strategy attempts recorded")
	}
	// The first strategy in the table is the one that runs first; the order of
	// the record is the order of the cascade, which is what makes it readable
	// as a ladder later.
	if m.Strategies[0].Name != strategies[0].name() {
		t.Errorf("first attempt %q, want %q", m.Strategies[0].Name, strategies[0].name())
	}
	var found int
	for _, a := range m.Strategies {
		found += a.Found
	}
	if found == 0 {
		t.Error("a decoded image recorded no strategy that found anything")
	}
	if m.LaplacianVariance <= 0 {
		t.Errorf("laplacian variance on a QR = %v, want > 0", m.LaplacianVariance)
	}
}

// TestMetadataRecordsEveryStrategyOnFailure is the measurement case: an image
// nothing can read must still say what was tried, or the ladder has no
// baseline to be compared against.
func TestMetadataRecordsEveryStrategyOnFailure(t *testing.T) {
	d, err := ScanDecodedImageDetail(flatGray(200, 200, 128))
	if err != nil {
		t.Fatalf("ScanDecodedImageDetail: %v", err)
	}
	if d.Metadata == nil {
		t.Fatal("no metadata on an image with nothing in it")
	}
	if got, want := len(d.Metadata.Strategies), len(strategies); got != want {
		t.Errorf("recorded %d attempts on a total failure, want all %d", got, want)
	}
	for _, a := range d.Metadata.Strategies {
		if a.Found != 0 {
			t.Errorf("strategy %q found %d codes in a flat grey image", a.Name, a.Found)
		}
	}
}

// TestMetadataIsAdditiveInJSON pins the promise made to existing consumers of
// --export json: new keys only, and nothing at all when there is nothing to say.
func TestMetadataIsAdditiveInJSON(t *testing.T) {
	blob, err := json.Marshal(ScanResult{FilePath: "a.png", HasQR: false})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(blob), "metadata") {
		t.Errorf("a result with no metadata still emits the key: %s", blob)
	}

	blob, err = json.Marshal(ScanResult{
		FilePath: "a.png",
		Metadata: &Metadata{Width: 4, Height: 5, Strategies: []StrategyAttempt{{Name: "lum/hybrid/1x", Found: 1}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"metadata"`, `"width":4`, `"strategies"`, `"lum/hybrid/1x"`, `"found":1`} {
		if !strings.Contains(string(blob), want) {
			t.Errorf("marshalled result missing %s: %s", want, blob)
		}
	}
}

// writeMaskFor writes the module bit-matrix of a QR as the .npy companion the
// dataset ships, beside the image at imgPath.
func writeMaskFor(t *testing.T, imgPath, content string) {
	t.Helper()
	qr, err := encoder.Encoder_encodeWithoutHint(content, decoder.ErrorCorrectionLevel_L)
	if err != nil {
		t.Fatalf("encode %q: %v", content, err)
	}
	m := qr.GetMatrix()
	w, h := m.GetWidth(), m.GetHeight()

	// numpy bool array: one byte per element, C order, row-major.
	data := make([]byte, 0, w*h)
	for y := range h {
		for x := range w {
			if m.Get(x, y) == 1 {
				data = append(data, 1)
			} else {
				data = append(data, 0)
			}
		}
	}
	hdr := fmt.Sprintf("{'descr': '|b1', 'fortran_order': False, 'shape': (%d, %d), }", h, w)
	writeNPY(t, strings.TrimSuffix(imgPath, filepath.Ext(imgPath))+".npy", hdr, data)
}

// TestNPYMaskGate covers the switch the corpus harness relies on. With masks
// on, the companion answers and no raster strategy needs to; with masks off
// the same file must go through the raster loop instead. A benchmark that
// cannot turn this off measures mask parsing, not decoding.
func TestNPYMaskGate(t *testing.T) {
	const content = "https://example.com/mask-gate"
	dir := t.TempDir()
	path := filepath.Join(dir, "coded.png")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, qrImage(t, content, 256)); err != nil {
		t.Fatal(err)
	}
	f.Close()
	writeMaskFor(t, path, content)

	prev := npyMaskEnabled
	defer func() { npyMaskEnabled = prev }()

	npyMaskEnabled = true
	on, err := ScanImageDetail(path, ScanFast)
	if err != nil {
		t.Fatalf("with masks: %v", err)
	}
	if !hasStrategy(on.Metadata, strategyNPYMask) {
		t.Fatalf("with masks enabled the companion was not used: %+v", on.Metadata)
	}

	npyMaskEnabled = false
	off, err := ScanImageDetail(path, ScanFast)
	if err != nil {
		t.Fatalf("without masks: %v", err)
	}
	if hasStrategy(off.Metadata, strategyNPYMask) {
		t.Errorf("with masks ignored the companion was still read: %+v", off.Metadata)
	}
	if len(off.Codes) != 1 || off.Codes[0] != content {
		t.Errorf("raster loop decoded %q, want [%q]", off.Codes, content)
	}
	if off.Metadata == nil || len(off.Metadata.Strategies) == 0 {
		t.Fatal("no raster strategy ran with masks ignored")
	}
}

func hasStrategy(m *Metadata, name string) bool {
	if m == nil {
		return false
	}
	for _, a := range m.Strategies {
		if a.Name == name {
			return true
		}
	}
	return false
}
