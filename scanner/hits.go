package scanner

// Geometry layer for multi-code decoding: what a decoded code is, how two
// sightings of the same physical code are recognised as one, and what order
// results come out in.

import (
	"math"
	"sort"

	"github.com/makiuchi-d/gozxing"
)

// hit is one decoded QR code together with where it was found.
//
// COORDINATE CONTRACT — read this before adding a decode strategy.
//
// points are ALWAYS expressed in the coordinate space of the image handed to
// decodeRaster: the original image, unscaled and uncropped. A strategy that
// upscales, crops or tiles the image before decoding MUST map its result
// points back into that space at capture time, inside decodeAttempt — never
// at compare time.
//
// This is not a style preference. Dedup compares geometry across strategies,
// so a strategy reporting a code in its own coordinate space simply stops
// matching the same code found by another strategy, and the duplicate is
// emitted as if it were a second physical code. Nothing errors and no test
// that only checks payloads will notice. The upcoming tiling cascade makes
// this acute: overlapping tiles find the same code repeatedly by design, and
// every tile must add its offset back before returning.
//
// points may be empty. Decoders that report no geometry at all (zbarimg) are
// merged by payload instead — see sameCode.
type hit struct {
	text   string
	points []gozxing.ResultPoint
}

// hasGeometry reports whether h carries enough points to be located and sized.
func (h hit) hasGeometry() bool { return len(h.points) >= 2 }

// centroid is the mean of h's result points, in original-image coordinates.
func (h hit) centroid() (x, y float64) {
	if len(h.points) == 0 {
		return 0, 0
	}
	for _, p := range h.points {
		x += p.GetX()
		y += p.GetY()
	}
	n := float64(len(h.points))
	return x / n, y / n
}

// size estimates how large the code is on the page: the widest distance
// between any two of its result points, which for a QR is the diagonal
// between finder patterns.
func (h hit) size() float64 {
	var max float64
	for i := range h.points {
		for j := i + 1; j < len(h.points); j++ {
			if d := gozxing.ResultPoint_Distance(h.points[i], h.points[j]); d > max {
				max = d
			}
		}
	}
	return max
}

// sameCodeFactor is how close two centroids must be, as a fraction of the
// smaller code's own size, to be judged the same physical code.
//
// Two flat printed QR codes cannot have centroids closer than roughly 1.0x
// their own extent — any closer and they physically overlap, which printed
// codes in one plane cannot do. Codes butted edge to edge sit at about 1.0.
// Half of that leaves a 2x margin below the physical floor, while the same
// code re-found by another strategy or an overlapping tile lands within a few
// pixels, orders of magnitude under the bound. Scaling by the code's own size
// rather than an absolute pixel count is what keeps this working when a small
// and a large code share one frame.
const sameCodeFactor = 0.5

// sameCode reports whether two hits are the same physical code.
//
// Geometry decides it whenever both hits have it. Payload is deliberately NOT
// used in that case: two physically distinct codes can legitimately carry
// identical content — two copies of the same receipt in one frame — and
// merging them by text would silently report one code where there are two.
//
// Payload equality is the fallback only when a hit carries no geometry,
// because a decoder that reports no position leaves nothing else to compare.
// That fallback can under-report N identical-payload codes; it is the price of
// accepting results from a decoder that does not say where it looked.
func sameCode(a, b hit) bool {
	if !a.hasGeometry() || !b.hasGeometry() {
		return a.text == b.text
	}
	ax, ay := a.centroid()
	bx, by := b.centroid()
	tolerance := sameCodeFactor * math.Min(a.size(), b.size())
	return math.Hypot(ax-bx, ay-by) < tolerance
}

// mergeHits returns base plus every candidate that is not already present.
// Order is preserved so the caller controls which sighting of a code wins.
func mergeHits(base, candidates []hit) []hit {
	for _, c := range candidates {
		dup := false
		for _, b := range base {
			if sameCode(b, c) {
				dup = true
				break
			}
		}
		if !dup {
			base = append(base, c)
		}
	}
	return base
}

// sortHits puts hits into reading order: top to bottom, then left to right
// within a row, with the payload as a final tiebreak.
//
// Rows are found by quantising the vertical centroid into bands one typical
// code-height tall, rather than by comparing y values pairwise. Pairwise
// banding is not transitive — a can share a row with b, and b with c, while a
// and c do not — which is not a valid ordering and would leave the result
// dependent on the input order. Quantising gives a genuine total order, so the
// manifest is stable no matter which strategy found what first.
//
// Hits with no geometry sort last, among themselves by payload: they have no
// position to place them by.
func sortHits(hits []hit) {
	band := typicalSize(hits)
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.hasGeometry() != b.hasGeometry() {
			return a.hasGeometry()
		}
		if !a.hasGeometry() {
			return a.text < b.text
		}
		ax, ay := a.centroid()
		bx, by := b.centroid()
		if ra, rb := int(math.Floor(ay/band)), int(math.Floor(by/band)); ra != rb {
			return ra < rb
		}
		if ax != bx {
			return ax < bx
		}
		return a.text < b.text
	})
}

// typicalSize is the median code size across hits, used as the row height for
// reading order. Median rather than mean so one huge or tiny code cannot drag
// the banding for everything else.
func typicalSize(hits []hit) float64 {
	var sizes []float64
	for _, h := range hits {
		if h.hasGeometry() {
			if s := h.size(); s > 0 {
				sizes = append(sizes, s)
			}
		}
	}
	if len(sizes) == 0 {
		return 1
	}
	sort.Float64s(sizes)
	return sizes[len(sizes)/2]
}

// hitTexts extracts the payloads, preserving order.
func hitTexts(hits []hit) []string {
	if len(hits) == 0 {
		return nil
	}
	texts := make([]string, len(hits))
	for i, h := range hits {
		texts[i] = h.text
	}
	return texts
}
