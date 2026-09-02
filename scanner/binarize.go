package scanner

// Pixel preprocessing steps that run before gozxing binarizes.
//
// These are transforms, not Binarizer implementations, and that is the whole
// trick: a step that outputs a two-valued image has already made the decision a
// binarizer would make, so it can be handed to the cheapest binarizer in the
// library and pass through untouched. That keeps every new rung a pure function
// over pixels — testable on its own, with no gozxing interface to satisfy and
// no decoder state to thread.
//
// Each is named in the strategy table and recorded by name in the per-image
// metadata, so a rung that never wins can be found and removed.

import (
	"image"
	"math"
)

// transforms are the named preprocessing steps a strategy can ask for. The map
// is the registry: adding a rung means adding a function here and a row in the
// strategy table, and nothing else knows the difference.
var transforms = map[string]func(image.Image) *image.Gray{
	"sauvola": sauvola,
	"otsu":    otsu,
	"invert":  invert,
}

// invert flips light and dark.
//
// A QR code printed light-on-dark is not a decoding problem, it is a detection
// problem: the finder-pattern scan looks for a dark-light-dark-light-dark run
// of ratios 1:1:3:1:1, and on an inverted code every one of those runs is the
// wrong colour, so the detector reports nothing at all rather than reporting a
// code it cannot read. No binarizer choice recovers that, because binarization
// is not what went wrong. One subtraction per pixel does.
func invert(img image.Image) *image.Gray {
	src := grayOf(img)
	dst := image.NewGray(src.Bounds())
	for i, v := range src.Pix {
		dst.Pix[i] = 255 - v
	}
	return dst
}

// otsuBins is the histogram resolution. 256 is every distinct 8-bit value:
// Otsu's criterion is a search over thresholds, and quantising the search
// space is the one thing that would make it a different algorithm.
const otsuBins = 256

// otsu thresholds on the between-class variance maximum over the whole image.
//
// WHY THIS IS NOT WHAT gozxing ALREADY DOES. GlobalHistogramBinarizer is also
// a global threshold, and the resemblance stops there. It builds a 32-bucket
// histogram from four sampled rows — `height*y/5` for y in 1..4 — across the
// middle 60% of each, finds the tallest peak, finds a second peak weighted by
// squared distance, and picks a valley between them by a heuristic score. Four
// differences follow, and each is a case where the two disagree:
//
//  1. It samples four rows. A code that does not sit across those rows, or a
//     photograph whose background dominates them, is thresholded from a
//     histogram the code barely appears in. Otsu reads every pixel.
//  2. It quantises to 5 bits. Otsu searches all 256 levels.
//  3. Its criterion is a valley heuristic that assumes a clean bimodal
//     histogram. This dataset renders codes under vertical, radial and
//     horizontal colour gradients, which smear exactly that bimodality;
//     between-class variance still has a maximum when there is no visible
//     valley.
//  4. It gives up. When the two peaks land within two buckets of each other it
//     returns NotFound and nothing is thresholded at all. Otsu always returns a
//     threshold, which on a low-contrast image is the difference between a bad
//     attempt and no attempt.
//
// So it is a separate rung rather than a duplicate one. Whether it earns its
// place is a measurement, not an argument.
func otsu(img image.Image) *image.Gray {
	src := grayOf(img)
	var hist [otsuBins]int
	for _, v := range src.Pix {
		hist[v]++
	}

	total := len(src.Pix)
	if total == 0 {
		return src
	}
	var sum float64
	for i, n := range hist {
		sum += float64(i) * float64(n)
	}

	var sumB, wB float64
	var best float64
	threshold := 0
	for t := range otsuBins {
		wB += float64(hist[t])
		if wB == 0 {
			continue
		}
		wF := float64(total) - wB
		if wF == 0 {
			break
		}
		sumB += float64(t) * float64(hist[t])
		mB := sumB / wB
		mF := (sum - sumB) / wF
		// Between-class variance. Maximising it is equivalent to minimising
		// the weighted within-class variance, which is the form Otsu states.
		if v := wB * wF * (mB - mF) * (mB - mF); v > best {
			best = v
			threshold = t
		}
	}

	dst := image.NewGray(src.Bounds())
	for i, v := range src.Pix {
		if int(v) > threshold {
			dst.Pix[i] = 255
		}
	}
	return dst
}

// Sauvola parameters. The defaults are the ones from the paper, and they are
// defaults rather than tuned values: tuning them against this corpus would tune
// them against 256x256 renders, which is the distribution the roadmap says not
// to tune against.
const (
	// sauvolaWindow is the side of the local neighbourhood in pixels. At the
	// module sizes this corpus produces (median 4.6 px) it spans roughly three
	// modules, which is the range a local threshold wants: wide enough to hold
	// both colours, narrow enough that a gradient does not cross it.
	sauvolaWindow = 15
	// sauvolaK controls how far the threshold is pulled below the local mean in
	// low-variance neighbourhoods. Higher removes more background and eats thin
	// features.
	sauvolaK = 0.2
	// sauvolaR is the assumed dynamic range of the standard deviation, 128 for
	// 8-bit images.
	sauvolaR = 128.0
)

// sauvola thresholds each pixel against its own neighbourhood:
//
//	T(x,y) = m(x,y) * (1 + k * (s(x,y)/R - 1))
//
// where m and s are the local mean and standard deviation. It is the rung
// aimed at the images whose finder patterns were located and whose payload
// would not come out — glare across one corner, a gradient fill, a shadow — the
// cases where no single global threshold is right for the whole frame.
//
// It runs in one pass with two width-sized accumulators rather than the usual
// pair of integral images. That is not micro-optimisation: integral images of a
// 12-megapixel photo need a 48 MB sum and a 96 MB sum-of-squares, which is a
// cliff this transform would fall off exactly on the inputs it exists for. The
// rolling column sums are O(width) whatever the image.
//
// It assumes dark modules on a light ground, as every local threshold does.
// Inverted codes are the `invert` rung's problem, not this one's.
func sauvola(img image.Image) *image.Gray {
	src := grayOf(img)
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewGray(b)
	if w == 0 || h == 0 {
		return dst
	}

	const r = sauvolaWindow / 2
	// colSum[x] holds the sum of the pixels in column x over the rows currently
	// inside the vertical window; colSumSq the sum of their squares. Both are
	// int64 because a 12-megapixel column of squares overflows 32 bits.
	colSum := make([]int64, w)
	colSumSq := make([]int64, w)
	colRows := 0 // how many rows are currently accumulated, for the edge rows

	addRow := func(y int, sign int64) {
		if y < 0 || y >= h {
			return
		}
		off := src.PixOffset(b.Min.X, b.Min.Y+y)
		for x := range w {
			v := int64(src.Pix[off+x])
			colSum[x] += sign * v
			colSumSq[x] += sign * v * v
		}
		colRows += int(sign)
	}

	// Prime the window for row 0.
	for y := 0; y <= r; y++ {
		addRow(y, 1)
	}

	for y := range h {
		// Horizontal running sums across the column accumulators.
		var winSum, winSumSq int64
		var winCols int
		push := func(x int) {
			if x < 0 || x >= w {
				return
			}
			winSum += colSum[x]
			winSumSq += colSumSq[x]
			winCols++
		}
		pop := func(x int) {
			if x < 0 || x >= w {
				return
			}
			winSum -= colSum[x]
			winSumSq -= colSumSq[x]
			winCols--
		}
		for x := 0; x <= r; x++ {
			push(x)
		}

		dstOff := dst.PixOffset(b.Min.X, b.Min.Y+y)
		srcOff := src.PixOffset(b.Min.X, b.Min.Y+y)
		for x := range w {
			n := float64(winCols * colRows)
			if n > 0 {
				mean := float64(winSum) / n
				// Variance as E[x²] - E[x]², clamped: the two terms are large
				// and close on a flat neighbourhood, so cancellation can put
				// the difference slightly below zero.
				variance := float64(winSumSq)/n - mean*mean
				if variance < 0 {
					variance = 0
				}
				t := mean * (1 + sauvolaK*(math.Sqrt(variance)/sauvolaR-1))
				if float64(src.Pix[srcOff+x]) > t {
					dst.Pix[dstOff+x] = 255
				}
			} else {
				dst.Pix[dstOff+x] = src.Pix[srcOff+x]
			}
			pop(x - r)
			push(x + r + 1)
		}

		addRow(y-r, -1)
		addRow(y+r+1, 1)
	}
	return dst
}
