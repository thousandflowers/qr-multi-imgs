package scanner

// Per-image annotation: what the image looked like, and what was tried on it.
//
// This is measurement infrastructure for the retry ladder, and it lands before
// the ladder on purpose. A rung that cannot be shown to buy recall does not
// get to stay, and "which strategy actually read this code" is not recoverable
// after the fact from a payload. So the scan records it while it happens.
//
// THE HARD RULE, restated here because this is the file most likely to break
// it: metadata is annotation, never a filter. Nothing is skipped, downranked or
// abandoned because a number looks bad. A blurry image still runs every
// strategy. The moment a threshold gates work, the numbers stop describing the
// scan and start causing it, and every recall figure afterwards measures the
// threshold instead of the decoder.

import (
	"image"
	"math"
)

// StrategyAttempt is one decode stage that ran, and how many codes came out of
// it. Attempts are recorded in the order they ran.
//
// "The one that succeeded" is any attempt with Found > 0, and there can be
// more than one: the raster pass is a union, not a race, so two strategies
// finding the same code both report it. Recording a single winner would have
// to pick one arbitrarily and would hide exactly the overlap the ladder needs
// to see when it is reordered on measured hit rates.
//
// Found counts what the attempt returned on its own, before dedup against
// everything else. A code found by four strategies is four Founds and one
// payload.
type StrategyAttempt struct {
	Name  string `json:"name"`
	Found int    `json:"found"`
}

// Metadata is the annotation recorded for every image, whatever the outcome.
//
// Width and Height are zero when no raster was ever decoded — an unreadable
// file, or a HEIC that only Apple Vision could open. The image measures are
// zero then too, and a zero is not a measurement: consumers must read them
// beside the dimensions.
type Metadata struct {
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`

	// LaplacianVariance is the variance of a 3x3 Laplacian response over the
	// grayscale image, the standard cheap blur proxy: sharp edges produce a
	// wide spread of responses, a blurred image a narrow one. It is in squared
	// 8-bit intensity units and it is scale-dependent — the same photo at half
	// the resolution scores differently — so it compares images of comparable
	// size, not any two images.
	LaplacianVariance float64 `json:"laplacian_variance,omitempty"`

	// EdgeDensity is the mean gradient magnitude of the busiest
	// edgeWindow-sized block, normalised to 0-1. It is "local" in the sense
	// that matters here: a QR code in a corner of an otherwise empty photo
	// raises it, while an image-wide average would bury it.
	//
	// It carries no threshold and defines no bucket. HIGH_EDGE_DENSITY_REGION
	// was dropped for want of evidence that such a bucket is useful, and this
	// field is here to gather that evidence, not to pre-empt it.
	EdgeDensity float64 `json:"edge_density,omitempty"`

	// Strategies is every decode stage that ran, in order.
	Strategies []StrategyAttempt `json:"strategies,omitempty"`
}

// record appends one attempt. A nil Metadata is a no-op so callers on the
// error paths do not have to check.
func (m *Metadata) record(name string, found int) {
	if m == nil {
		return
	}
	m.Strategies = append(m.Strategies, StrategyAttempt{Name: name, Found: found})
}

// Names for the decode stages that are not raster strategies. They are stages
// in exactly the sense that matters to the benchmark: something that ran and
// either produced a payload or did not.
const (
	strategyNPYMask = "npy-mask"
	strategyVision  = "vision"
	strategyZbarimg = "zbarimg"
)

// name identifies a raster strategy in the annotation, as channel/binarizer/scale.
func (s decodeStrategy) name() string {
	bin := "hybrid"
	if s.global {
		bin = "global"
	}
	return s.channel + "/" + bin + "/" + itoa(s.scale) + "x"
}

// itoa avoids pulling strconv in for one digit. Scales are 1 and 2 today and a
// ladder rung will not plausibly exceed 9; anything larger falls back to a
// marker rather than lying about the number.
func itoa(n int) string {
	if n < 0 || n > 9 {
		return "?"
	}
	return string(rune('0' + n))
}

// edgeWindow is the block size, in pixels, over which edge density is measured.
// 64 is large enough that a single noisy pixel cannot carry a block and small
// enough that a QR code occupying a modest part of a photo still fills one.
const edgeWindow = 64

// measure computes the image annotation. One pass over the grayscale
// projection produces both numbers, because two passes over a 12-megapixel
// photo is the kind of cost that gets a measurement deleted.
//
// Border pixels are skipped: a 3x3 kernel has no defined response there, and
// clamping or wrapping would invent edges at the frame that no image has.
func measure(img image.Image) (laplacianVariance, edgeDensity float64) {
	gray := grayOf(img)
	b := gray.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 3 || h < 3 {
		return 0, 0
	}

	at := func(x, y int) float64 {
		return float64(gray.Pix[y*gray.Stride+x])
	}

	// Welford would be tidier; sum and sum-of-squares is enough here because
	// the values are bounded by construction (a Laplacian of 8-bit intensities
	// cannot exceed +-1020) and the pixel count is bounded by the image.
	var sum, sumSq float64
	var n float64

	// Blocks are indexed on the interior grid, so a block's index is derived
	// from the same coordinates the loop already has.
	blocksX := (w-2+edgeWindow-1)/edgeWindow + 1
	blockSum := make([]float64, blocksX*((h-2+edgeWindow-1)/edgeWindow+1))
	blockN := make([]float64, len(blockSum))

	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			c := at(x, y)
			up, down, left, right := at(x, y-1), at(x, y+1), at(x-1, y), at(x+1, y)

			lap := up + down + left + right - 4*c
			sum += lap
			sumSq += lap * lap
			n++

			// Central differences: the same four neighbours the Laplacian
			// already read, so the gradient costs no extra fetches.
			gx, gy := (right-left)/2, (down-up)/2
			bi := ((y-1)/edgeWindow)*blocksX + (x-1)/edgeWindow
			blockSum[bi] += math.Hypot(gx, gy)
			blockN[bi]++
		}
	}

	if n == 0 {
		return 0, 0
	}
	mean := sum / n
	laplacianVariance = sumSq/n - mean*mean
	if laplacianVariance < 0 {
		// Only reachable through floating-point cancellation on a flat image.
		laplacianVariance = 0
	}

	var best float64
	for i, cnt := range blockN {
		if cnt == 0 {
			continue
		}
		if v := blockSum[i] / cnt; v > best {
			best = v
		}
	}
	// A central-difference gradient magnitude tops out at 255 on a black/white
	// edge in both axes; dividing by that keeps the field in 0-1 without
	// clamping a real measurement.
	edgeDensity = best / 255
	if edgeDensity > 1 {
		edgeDensity = 1
	}
	return laplacianVariance, edgeDensity
}

// grayOf returns img as 8-bit grayscale, without copying when it already is.
//
// The type switches are not premature: At() returns a color.Color interface and
// converts through 16-bit channels, which on a 12-megapixel photo is tens of
// millions of interface calls and allocations-in-disguise. *image.RGBA is what
// the browser hands over and *image.YCbCr is what every JPEG decodes to, so
// those two cover nearly all real input; anything else falls back to At().
func grayOf(img image.Image) *image.Gray {
	if g, ok := img.(*image.Gray); ok {
		return g
	}
	b := img.Bounds()
	dst := image.NewGray(b)
	switch src := img.(type) {
	case *image.RGBA:
		for y := b.Min.Y; y < b.Max.Y; y++ {
			i := src.PixOffset(b.Min.X, y)
			o := dst.PixOffset(b.Min.X, y)
			for x := 0; x < b.Dx(); x++ {
				r, g, bl := src.Pix[i], src.Pix[i+1], src.Pix[i+2]
				// Rec. 601 luma, the same weighting image/color uses.
				dst.Pix[o] = uint8((19595*uint32(r) + 38470*uint32(g) + 7471*uint32(bl) + 1<<15) >> 16)
				i += 4
				o++
			}
		}
	case *image.YCbCr:
		for y := b.Min.Y; y < b.Max.Y; y++ {
			o := dst.PixOffset(b.Min.X, y)
			for x := b.Min.X; x < b.Max.X; x++ {
				dst.Pix[o] = src.Y[src.YOffset(x, y)]
				o++
			}
		}
	default:
		for y := b.Min.Y; y < b.Max.Y; y++ {
			o := dst.PixOffset(b.Min.X, y)
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, _ := img.At(x, y).RGBA()
				dst.Pix[o] = uint8((19595*(r>>8) + 38470*(g>>8) + 7471*(bl>>8) + 1<<15) >> 16)
				o++
			}
		}
	}
	return dst
}

// metadataOf annotates one raster decode: the image's own dimensions and
// measures, plus the strategies that ran on it.
//
// Every image gets one, including an image nothing was found in. That is the
// point - a bucket of failures with no numbers beside them is what the ladder
// cannot be built from.
func metadataOf(img image.Image, attempts []StrategyAttempt) *Metadata {
	b := img.Bounds()
	lap, edge := measure(img)
	return &Metadata{
		Width:             b.Dx(),
		Height:            b.Dy(),
		LaplacianVariance: lap,
		EdgeDensity:       edge,
		Strategies:        attempts,
	}
}
