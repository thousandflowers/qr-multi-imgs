package scanner

import (
	"image"
	"image/color"
	"testing"
	"time"
)

// How long one live frame costs today, decoded the way the viewfinder decodes
// it. Two cases, because they are not the same cost at all: a frame holding a
// code stops early, a frame holding nothing runs every strategy there is.
func TestLiveFrameCost(t *testing.T) {
	withCode := frameWith(t, qrImage(t, "https://example.com/live", 300), 720, 540)
	empty := image.NewRGBA(image.Rect(0, 0, 720, 540))
	for i := range empty.Pix {
		empty.Pix[i] = 200
	}

	for _, c := range []struct {
		name string
		img  image.Image
	}{{"con un codice", withCode}, {"senza niente", empty}} {
		for _, m := range []struct {
			how string
			run func(image.Image)
		}{
			{"cascata completa", func(i image.Image) { ScanDecodedImageDetail(i) }},
			{"live", func(i image.Image) { ScanLive(i) }},
		} {
			start := time.Now()
			const n = 5
			for range n {
				m.run(c.img)
			}
			per := time.Since(start) / n
			t.Logf("%-16s %-16s %6v per frame  (%5.1f fps)", c.name, m.how,
				per.Round(time.Millisecond), 1/per.Seconds())
		}
	}

	// The live path must still read a code that is plainly in shot. Fast and
	// blind is not a trade, it is a regression.
	d := ScanLive(withCode)
	if len(d.Codes) != 1 || d.Codes[0] != "https://example.com/live" {
		t.Errorf("the live path did not read a code in the middle of the frame: %q", d.Codes)
	}
	if len(d.Detections) != 1 || d.Detections[0].Text == "" {
		t.Errorf("the live path returned no box to draw: %+v", d.Detections)
	}
}

// frameWith centres an image on a grey background of the given size, the way a
// camera sees a code held up in front of it.
func frameWith(t *testing.T, src image.Image, w, h int) image.Image {
	t.Helper()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			dst.Set(x, y, color.RGBA{210, 210, 210, 255})
		}
	}
	b := src.Bounds()
	ox, oy := (w-b.Dx())/2, (h-b.Dy())/2
	for y := range b.Dy() {
		for x := range b.Dx() {
			dst.Set(ox+x, oy+y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
