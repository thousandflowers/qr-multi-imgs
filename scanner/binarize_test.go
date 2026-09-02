package scanner

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// twoValued reports whether every pixel is 0 or 255. A transform whose output
// still holds mid-tones has not made the decision the binarizer after it is
// being told to skip.
func twoValued(g *image.Gray) bool {
	for _, v := range g.Pix {
		if v != 0 && v != 255 {
			return false
		}
	}
	return true
}

func TestInvertRoundTrips(t *testing.T) {
	src := checkerGray(64, 64, 4)
	once := invert(src)
	twice := invert(once)
	for i := range src.Pix {
		if src.Pix[i] != twice.Pix[i] {
			t.Fatalf("pixel %d: %d -> %d -> %d", i, src.Pix[i], once.Pix[i], twice.Pix[i])
		}
		if once.Pix[i] != 255-src.Pix[i] {
			t.Fatalf("pixel %d inverted to %d, want %d", i, once.Pix[i], 255-src.Pix[i])
		}
	}
}

// TestInvertedQRIsUndecodableUntilInverted is the correctness case for the
// inversion rung, and the reason it ships without a corpus number: the
// benchmark set contains no light-on-dark codes, so a recall delta would be
// exactly zero and would say nothing about whether the rung works.
//
// It asserts the shape of the problem, not just the fix: an inverted code is
// invisible to the *detector*, not merely unreadable, so no binarizer choice
// recovers it.
func TestInvertedQRIsUndecodableUntilInverted(t *testing.T) {
	const content = "https://example.com/inverted"
	upright := qrImage(t, content, 256)
	flipped := invert(upright)

	if got, _ := ScanDecodedImage(upright); len(got) != 1 || got[0] != content {
		t.Fatalf("the upright fixture does not decode: %q", got)
	}

	// Nothing in the stock cascade sees it: no payload and no finder patterns.
	// If this stops being true the rung is unnecessary and this test says so.
	if infos := detectFinders(flipped); len(infos) != 0 {
		t.Errorf("the detector located %d finder triples in an inverted code; "+
			"inversion may no longer be needed", len(infos))
	}

	// And the transform recovers it.
	if got, _ := ScanDecodedImage(invert(flipped)); len(got) != 1 || got[0] != content {
		t.Fatalf("inverting an inverted code did not decode it: %q", got)
	}
}

// TestInversionRungIsInTheCascade closes the loop: the transform working is
// not the same as the ladder using it.
func TestInversionRungIsInTheCascade(t *testing.T) {
	const content = "https://example.com/inverted-cascade"
	flipped := invert(qrImage(t, content, 256))

	d, err := ScanDecodedImageDetail(flipped)
	if err != nil {
		t.Fatalf("ScanDecodedImageDetail: %v", err)
	}
	if len(d.Codes) != 1 || d.Codes[0] != content {
		t.Fatalf("cascade decoded %q, want [%q]", d.Codes, content)
	}
	if !hasStrategy(d.Metadata, "lum+invert/hybrid/1x") {
		t.Fatalf("the inversion rung never ran: %+v", d.Metadata)
	}
	for _, a := range d.Metadata.Strategies {
		if a.Name == "lum+invert/hybrid/1x" && a.Found == 0 {
			t.Error("the inversion rung ran and found nothing on an inverted code")
		}
	}
}

func TestOtsuSplitsABimodalImage(t *testing.T) {
	// Two populations, 60 and 200, with nothing between them. The only
	// defensible threshold is somewhere in the gap.
	img := image.NewGray(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			v := uint8(60)
			if x >= 32 {
				v = 200
			}
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	out := otsu(img)
	if !twoValued(out) {
		t.Fatal("otsu left mid-tones in its output")
	}
	for y := range 64 {
		for x := range 64 {
			want := uint8(0)
			if x >= 32 {
				want = 255
			}
			if got := out.GrayAt(x, y).Y; got != want {
				t.Fatalf("(%d,%d) = %d, want %d", x, y, got, want)
			}
		}
	}
}

func TestOtsuOnAFlatImageDoesNotPanic(t *testing.T) {
	// No between-class variance exists, so no threshold is better than any
	// other. The contract is that it returns something and does not crash.
	out := otsu(flatGray(32, 32, 128))
	if out == nil || len(out.Pix) != 32*32 {
		t.Fatal("otsu returned nothing for a flat image")
	}
}

// TestSauvolaBeatsAGlobalThresholdUnderAGradient is the property the rung
// exists for. The image is dark squares on a background that ramps from black
// to white across the frame, so no single global threshold is right everywhere:
// one that keeps the squares on the bright side erases the dark side entirely.
func TestSauvolaBeatsAGlobalThresholdUnderAGradient(t *testing.T) {
	const w, h = 256, 64
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			bg := float64(x) / float64(w-1) * 255
			v := bg
			// Dark squares, always 60 below their local background, so they are
			// locally obvious and globally unseparable.
			if (x/16)%2 == 0 && (y/16)%2 == 0 {
				v = math.Max(0, bg-60)
			}
			img.SetGray(x, y, color.Gray{Y: uint8(v)})
		}
	}

	local := sauvola(img)
	global := otsu(img)
	if !twoValued(local) {
		t.Fatal("sauvola left mid-tones in its output")
	}

	// Score each against the truth: a square pixel should end black, a
	// background pixel white.
	score := func(g *image.Gray) float64 {
		right := 0
		for y := range h {
			for x := range w {
				want := uint8(255)
				if (x/16)%2 == 0 && (y/16)%2 == 0 {
					want = 0
				}
				if g.GrayAt(x, y).Y == want {
					right++
				}
			}
		}
		return float64(right) / float64(w*h)
	}
	l, gl := score(local), score(global)
	if l <= gl {
		t.Errorf("under a gradient the local threshold scored %.3f and the global one %.3f; "+
			"the local one must do better or the rung has no reason to exist", l, gl)
	}
	if l < 0.9 {
		t.Errorf("sauvola recovered only %.3f of the gradient fixture", l)
	}
}

func TestSauvolaLeavesACleanQRDecodable(t *testing.T) {
	// The rung must not destroy what already worked. A clean QR through the
	// transform still has to decode.
	const content = "https://example.com/sauvola-clean"
	out := sauvola(qrImage(t, content, 256))
	got, err := ScanDecodedImage(out)
	if err != nil {
		t.Fatalf("ScanDecodedImage: %v", err)
	}
	if len(got) != 1 || got[0] != content {
		t.Fatalf("a clean QR through sauvola decoded %q, want [%q]", got, content)
	}
}

// TestSauvolaHandlesEdgesAndTinyImages covers the window clamping. The rolling
// accumulators are the part most likely to be wrong at the frame, and being
// wrong there means inventing edges that no image has.
func TestSauvolaHandlesEdgesAndTinyImages(t *testing.T) {
	for _, d := range []int{1, 2, 3, 8, sauvolaWindow, sauvolaWindow + 1} {
		out := sauvola(flatGray(d, d, 200))
		if out.Bounds().Dx() != d || out.Bounds().Dy() != d {
			t.Fatalf("%dx%d in, %v out", d, d, out.Bounds())
		}
		// A flat field has no local structure, so every pixel sits above a
		// threshold pulled below its own mean: all background, no invented
		// edges at the frame.
		for i, v := range out.Pix {
			if v != 255 {
				t.Fatalf("%dx%d flat image: pixel %d = %d, want 255 (an edge was invented)", d, d, i, v)
			}
		}
	}
}

// TestSauvolaHonoursTheOffsetContract: the transforms are handed images whose
// bounds do not necessarily start at the origin, and a PixOffset mistake there
// silently shifts every pixel.
func TestSauvolaOnAnOffsetImage(t *testing.T) {
	full := checkerGray(64, 64, 8)
	sub := full.SubImage(image.Rect(16, 16, 48, 48)).(*image.Gray)
	out := sauvola(sub)
	if out.Bounds().Dx() != 32 || out.Bounds().Dy() != 32 {
		t.Fatalf("offset sub-image produced %v", out.Bounds())
	}
	if !twoValued(out) {
		t.Error("sauvola left mid-tones on an offset image")
	}
}
