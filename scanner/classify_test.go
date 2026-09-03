package scanner

import (
	"image"
	"image/color"
	"image/draw"
	"math/rand"
	"os"
	"testing"
	"time"
)

func TestClassifyDecoded(t *testing.T) {
	d, err := ScanDecodedImageDetail(qrImage(t, "READABLE", 256))
	if err != nil {
		t.Fatalf("ScanDecodedImageDetail: %v", err)
	}
	if d.Classification != Decoded {
		t.Errorf("classification = %q, want %q", d.Classification, Decoded)
	}
	if len(d.Codes) != 1 || d.Codes[0] != "READABLE" {
		t.Errorf("codes = %v, want [READABLE]", d.Codes)
	}
	// A decoded code DOES get a box here, and it carries its payload.
	//
	// This reverses an earlier decision, deliberately. "The payload is the
	// answer and a box adds nothing" holds for a list of files and breaks the
	// moment something has to point at a code in a moving picture: the live
	// camera view draws every code it can see and colours it by whether the
	// text came out. The path-based scan is unchanged - see
	// TestScanImageDetailDecodedHasNoBox - so no export or API consumer moves.
	if len(d.Detections) != 1 {
		t.Fatalf("detections on a decoded image = %d, want 1", len(d.Detections))
	}
	if d.Detections[0].Text != "READABLE" {
		t.Errorf("detection text = %q, want READABLE", d.Detections[0].Text)
	}
	b := d.Detections[0].Box
	if b.W <= 0 || b.H <= 0 {
		t.Errorf("decoded detection has no area: %+v", b)
	}
	if b.X < 0 || b.Y < 0 || b.X+b.W > 256 || b.Y+b.H > 256 {
		t.Errorf("decoded detection escaped the image: %+v", b)
	}
}

// TestScanImageDetailDecodedHasNoBox pins the half that did NOT change. The
// file path feeds the JSON export and the local API, where a box on a decoded
// row is a new field in a released output format for no reader.
func TestScanImageDetailDecodedHasNoBox(t *testing.T) {
	path := createTestQR(t, "READABLE")
	d, err := ScanImageDetail(path, ScanFast)
	if err != nil {
		t.Fatalf("ScanImageDetail: %v", err)
	}
	if d.Classification != Decoded {
		t.Fatalf("classification = %q, want %q", d.Classification, Decoded)
	}
	if len(d.Detections) != 0 {
		t.Errorf("the file path grew boxes on a decoded image: %d", len(d.Detections))
	}
}

func TestClassifyNoQRFound(t *testing.T) {
	blank := image.NewGray(image.Rect(0, 0, 200, 200))
	draw.Draw(blank, blank.Bounds(), image.NewUniform(color.Gray{Y: 255}), image.Point{}, draw.Src)

	d, err := ScanDecodedImageDetail(blank)
	if err != nil {
		t.Fatalf("ScanDecodedImageDetail: %v", err)
	}
	if d.Classification != NoQRFound {
		t.Errorf("classification = %q, want %q", d.Classification, NoQRFound)
	}
	if len(d.Detections) != 0 {
		t.Errorf("detections on a blank image = %d, want 0", len(d.Detections))
	}
}

// The whole point of the change: an image whose finder patterns are plainly
// there and whose payload cannot be recovered must not be reported as an image
// with no code in it.
//
// The damage is applied to the data area only, the three finder corners are
// left intact, because that is the real-world failure this bucket names: the
// code is visible, the payload is gone.
func TestClassifyDetectedButUndecodable(t *testing.T) {
	src, ok := qrImage(t, "https://example.com/order/12345678", 256).(*image.Gray)
	if !ok {
		t.Fatal("qrImage no longer returns *image.Gray; this test reads its pixels")
	}
	b := src.Bounds()
	damaged := image.NewGray(b)
	copy(damaged.Pix, src.Pix)

	// Noise over the data area only. The damaged rectangle starts past every
	// finder pattern, each of the three sits inside its own corner, within the
	// first 30% of the frame on at least one axis, and past the timing
	// patterns that run between them, so the code stays findable while its
	// payload is destroyed well beyond what error correction can recover.
	rng := rand.New(rand.NewSource(7))
	x0, y0 := b.Dx()*35/100, b.Dy()*35/100
	x1, y1 := b.Dx()*95/100, b.Dy()*95/100
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			damaged.SetGray(x, y, color.Gray{Y: uint8(rng.Intn(256))})
		}
	}

	d, err := ScanDecodedImageDetail(damaged)
	if err != nil {
		t.Fatalf("ScanDecodedImageDetail: %v", err)
	}
	if len(d.Codes) != 0 {
		t.Skipf("the damaged image still decoded (%v), nothing to classify", d.Codes)
	}
	if d.Classification != QRDetectedDecodeFailed {
		t.Fatalf("classification = %q, want %q", d.Classification, QRDetectedDecodeFailed)
	}
	if len(d.Detections) == 0 {
		t.Fatal("no detections carried on a detected-but-failed image")
	}

	for i, det := range d.Detections {
		box := det.Box
		if box.W <= 0 || box.H <= 0 {
			t.Errorf("detection %d: empty box %+v", i, box)
		}
		if box.X < 0 || box.Y < 0 || box.X+box.W > b.Dx() || box.Y+box.H > b.Dy() {
			t.Errorf("detection %d: box %+v escapes the image %v", i, box, b)
		}
		if det.ModuleSize <= 0 {
			t.Errorf("detection %d: module size %v, want > 0", i, det.ModuleSize)
		}
		// The box should hug the code. It is checked per axis rather than by
		// area because the rendered QR carries a four-module quiet zone, so a
		// correct box covers about two thirds of each side and under half the
		// frame's area, an area threshold would fail on a right answer.
		if box.W*2 < b.Dx() || box.H*2 < b.Dy() {
			t.Errorf("detection %d: box %+v is under half the frame %v on an axis",
				i, box, b)
		}
	}
}

// A box built from finder-pattern centres alone cuts off the code's own
// corners, because a centre sits 3.5 modules inside the edge. This asserts the
// padding is actually applied.
func TestDetectionBoxIncludesFinderCorners(t *testing.T) {
	src := qrImage(t, "PADDING", 200)
	infos := detectFinders(src)
	if len(infos) == 0 {
		t.Skip("detector found no finder patterns in a clean QR, nothing to check")
	}
	dets := detectionsFrom(infos, src.Bounds())
	if len(dets) == 0 {
		t.Fatal("finder patterns located but no detection produced")
	}

	tl := infos[0].GetTopLeft()
	box := dets[0].Box
	if float64(box.X) >= tl.GetX() || float64(box.Y) >= tl.GetY() {
		t.Errorf("box %+v starts at or after the top-left finder centre (%.1f, %.1f); padding missing",
			box, tl.GetX(), tl.GetY())
	}
}

// A box must never escape the image it describes: the UI draws it over the
// picture, and a rectangle hanging off the edge is a visible bug.
func TestDetectionBoxClampedToImage(t *testing.T) {
	src := qrImage(t, "CLAMP", 200)
	infos := detectFinders(src)
	if len(infos) == 0 {
		t.Skip("detector found no finder patterns in a clean QR")
	}
	// A deliberately smaller frame than the one the patterns were found in.
	tight := image.Rect(0, 0, 50, 50)
	for i, det := range detectionsFrom(infos, tight) {
		box := det.Box
		if box.X < 0 || box.Y < 0 || box.X+box.W > 50 || box.Y+box.H > 50 {
			t.Errorf("detection %d: box %+v escapes %v", i, box, tight)
		}
	}
}

func TestEstimateVersionOnKnownQR(t *testing.T) {
	// A short payload is a low-version code; the estimate must land in a
	// plausible range rather than on an exact number, because the version is
	// measured off detected geometry, not read out of the code.
	src := qrImage(t, "VERSION", 256)
	infos := detectFinders(src)
	if len(infos) == 0 {
		t.Skip("detector found no finder patterns in a clean QR")
	}
	dets := detectionsFrom(infos, src.Bounds())
	if len(dets) == 0 {
		t.Fatal("finder patterns located but no detection produced")
	}
	if v := dets[0].Version; v < 1 || v > 10 {
		t.Errorf("estimated version = %d, want a small plausible version (1-10)", v)
	}
}

func TestClassificationReasonsAreDistinct(t *testing.T) {
	seen := map[string]Classification{}
	for _, c := range []Classification{Decoded, QRDetectedDecodeFailed, NoQRFound} {
		r := c.Reason()
		if r == "" {
			t.Errorf("%q has no reason string", c)
		}
		if prev, dup := seen[r]; dup {
			t.Errorf("%q and %q share the reason %q", prev, c, r)
		}
		seen[r] = c
	}
	if got := Classification("nonsense").Reason(); got != "" {
		t.Errorf("unknown classification reason = %q, want empty", got)
	}
}

// A scan of a folder must carry the reason all the way out to ScanResult, and
// the summary must count the located-but-unread images as a subset of the
// images without a payload, not as a fourth bucket beside them.
func TestSummaryCountsDetectedWithinWithoutQR(t *testing.T) {
	dir := t.TempDir()
	writeQRToFile(t, dir+"/readable.png", "SUMMARY-OK")

	s, err := ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder: %v", err)
	}
	if s.WithQR != 1 {
		t.Fatalf("WithQR = %d, want 1", s.WithQR)
	}
	if s.Results[0].Classification != Decoded {
		t.Errorf("classification = %q, want %q", s.Results[0].Classification, Decoded)
	}
	if s.Detected > s.WithoutQR {
		t.Errorf("Detected (%d) exceeds WithoutQR (%d); it must be a subset",
			s.Detected, s.WithoutQR)
	}
}

// ─── ordering and counting ──────────────────────────────────────────────────

// The terminal list leads with the images that decoded. Nothing in the TUI is
// per-row (every action works on the whole set)so ordering by what a person
// could act on buys a reader nothing, while it costs them the payloads: 25 rows
// are drawn at a time, and on a camera roll of mostly codeless photos putting
// NO_QR_FOUND above DECODED pushes every payload off the first screen.
//
// The web deliberately does the opposite, and pins it in its own check:
// web/order.js --test, run by CI beside this one. Two orders, two tests, so a
// change to either cannot quietly move the other.
func TestSortResultsPutsDecodedFirst(t *testing.T) {
	s := &Summary{Results: []ScanResult{
		{FilePath: "d-broken.jpg", Error: "unsupported format"},
		{FilePath: "c-ok.png", HasQR: true, Contents: []string{"X"}, Classification: Decoded},
		{FilePath: "b-nothing.png", Classification: NoQRFound},
		{FilePath: "a-unread.png", Classification: QRDetectedDecodeFailed},
	}}
	sortResults(s)

	want := []string{"c-ok.png", "a-unread.png", "b-nothing.png", "d-broken.jpg"}
	for i, w := range want {
		if got := s.Results[i].FilePath; got != w {
			t.Errorf("position %d = %q, want %q (full order: %v)", i, got, w, paths(s.Results))
		}
	}
}

// Classification still reaches the row even though it no longer moves it,
// viewList marks a located-but-unread image and names the reason. Saying what a
// row is and deciding where it sits are separate jobs, and this pins that the
// first one survived the second being reverted.
func TestSortResultsKeepsClassificationOnEveryRow(t *testing.T) {
	s := &Summary{Results: []ScanResult{
		{FilePath: "b-nothing.png", Classification: NoQRFound},
		{FilePath: "a-unread.png", Classification: QRDetectedDecodeFailed},
	}}
	sortResults(s)
	for _, r := range s.Results {
		if r.Classification == "" {
			t.Errorf("%s lost its classification in the sort", r.FilePath)
		}
	}
}

// Within one bucket the order must not depend on the order the files arrived
// in, or the same folder scanned twice reads differently each time.
func TestSortResultsStableWithinBucket(t *testing.T) {
	s := &Summary{Results: []ScanResult{
		{FilePath: "z.png", Classification: QRDetectedDecodeFailed},
		{FilePath: "a.png", Classification: QRDetectedDecodeFailed},
		{FilePath: "m.png", Classification: QRDetectedDecodeFailed},
	}}
	sortResults(s)
	if got := paths(s.Results); got[0] != "a.png" || got[1] != "m.png" || got[2] != "z.png" {
		t.Errorf("within-bucket order = %v, want sorted by path", got)
	}
}

// The count a person reads must not merge the two states back together after
// the scanner has gone to the trouble of telling them apart.
func TestSummarySeparatesUnreadFromEmpty(t *testing.T) {
	list := []ScanResult{
		{FilePath: "ok.png", HasQR: true, Contents: []string{"X"}, Classification: Decoded},
		{FilePath: "unread.png", Classification: QRDetectedDecodeFailed},
		{FilePath: "cat.jpg", Classification: NoQRFound},
		{FilePath: "broken.heic", Error: "unsupported format"},
	}
	s := tallySummary(list, time.Now())

	if s.WithQR != 1 {
		t.Errorf("WithQR = %d, want 1", s.WithQR)
	}
	if s.Errors != 1 {
		t.Errorf("Errors = %d, want 1", s.Errors)
	}
	if s.Detected != 1 {
		t.Errorf("Detected = %d, want 1", s.Detected)
	}
	// WithoutQR keeps its old meaning (no payload came out)so the organise
	// and delete actions still move the same files they always did. What is new
	// is that the actionable part of it can be named.
	if s.WithoutQR != 2 {
		t.Errorf("WithoutQR = %d, want 2 (unread + empty)", s.WithoutQR)
	}
	if empty := s.WithoutQR - s.Detected; empty != 1 {
		t.Errorf("images with genuinely nothing in them = %d, want 1", empty)
	}
}

// A file that could not be opened is not an image with no code in it. It gets
// no classification at all, because nothing ever looked at its pixels.
func TestUnreadableFileHasNoClassification(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/truncated.png"
	if err := os.WriteFile(path, []byte("\x89PNG\r\n\x1a\n not a png"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder: %v", err)
	}
	if len(s.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(s.Results))
	}
	r := s.Results[0]
	if r.Error == "" {
		t.Fatal("a truncated PNG produced no error")
	}
	if r.Classification != "" {
		t.Errorf("classification = %q, want empty on a file that never opened", r.Classification)
	}
	if s.Detected != 0 {
		t.Errorf("Detected = %d, want 0; an unopenable file is not a located code", s.Detected)
	}
}

func paths(rs []ScanResult) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.FilePath
	}
	return out
}
