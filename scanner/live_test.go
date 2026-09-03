package scanner

import (
	"image"
	"image/color"
	"testing"
	"time"
)

// Il costo di un frame del mirino, che è la ragione per cui esiste un secondo
// percorso di decodifica. La cascata completa non si ferma quando non trova
// niente - non ha un conteggio a cui fermarsi - e quello è il caso in cui si
// sta mentre si inquadra.
func TestLiveFrameCost(t *testing.T) {
	withCode := centred(t, qrImage(t, "https://example.com/live", 300), 720, 540)
	empty := image.NewRGBA(image.Rect(0, 0, 720, 540))
	for i := range empty.Pix {
		empty.Pix[i] = 200
	}

	for _, c := range []struct {
		name string
		img  image.Image
	}{{"con un codice", withCode}, {"frame vuoto", empty}} {
		var full, live time.Duration
		const n = 3
		for range n {
			s := time.Now()
			ScanDecodedImageDetail(c.img)
			full += time.Since(s)
			s = time.Now()
			ScanLive(c.img)
			live += time.Since(s)
		}
		full /= n
		live /= n
		t.Logf("%-14s completa %6v   live %6v   (%.1fx)", c.name,
			full.Round(time.Millisecond), live.Round(time.Millisecond),
			float64(full)/float64(live))
		if live > full {
			t.Errorf("%s: il percorso live costa piu' della cascata completa", c.name)
		}
	}
}

// Veloce e cieco non è un compromesso, è una regressione.
func TestLiveStillReads(t *testing.T) {
	img := centred(t, qrImage(t, "https://example.com/live", 300), 720, 540)
	d := ScanLive(img)
	if len(d.Codes) != 1 || d.Codes[0] != "https://example.com/live" {
		t.Fatalf("il percorso live non ha letto un codice in mezzo al frame: %q", d.Codes)
	}
	if len(d.Detections) != 1 || d.Detections[0].Text == "" {
		t.Errorf("nessun riquadro da disegnare: %+v", d.Detections)
	}
}

func centred(t *testing.T, src image.Image, w, h int) image.Image {
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
