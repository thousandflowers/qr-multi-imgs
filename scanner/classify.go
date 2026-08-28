package scanner

// Why an image ended up where it did.
//
// Before this, a scan answered one question — did a code come out? — and three
// different failures gave the same answer: a receipt whose QR is blown out by
// glare, a photo of a cat, and an image whose finder patterns are plainly
// visible but whose payload no strategy could recover. Telling a user "no QR
// found" about a code they can see with their own eyes is what makes the tool
// look broken when it is in fact working exactly as designed.
//
// The evidence needed to separate them is already computed. detectFinders runs
// before any decoding to give decodeRaster its stopping rule, and until now its
// geometry was discarded and only the count survived. Classification is that
// geometry, kept.

import (
	"image"
	"math"

	"github.com/makiuchi-d/gozxing/qrcode/detector"
)

// Classification is the reason behind a scan result.
//
// There are three, deliberately. A finer split built on partial finder
// centres was measured first and did not survive contact with the detector:
// those centres fire where there is no code at all — a uniform-noise image
// yields one — so a bucket built on them would promise a code the image does
// not contain. Three buckets each say something the evidence supports.
//
// What the evidence does support is the full triple: FindMulti locating three
// finder patterns while every decode strategy fails is exactly the case a
// person is looking at when they can see the code and the tool says there is
// none.
type Classification string

const (
	// Decoded — at least one payload came out. It says nothing about whether
	// every code in the image was read; an image holding one readable and one
	// unreadable code is Decoded, because it did decode.
	Decoded Classification = "DECODED"

	// QRDetectedDecodeFailed — a full finder-pattern triple was located and no
	// strategy could read a payload from it. This is the actionable one: the
	// tool saw what the user sees.
	QRDetectedDecodeFailed Classification = "QR_DETECTED_DECODE_FAILED"

	// NoQRFound — nothing decoded and no finder pattern was located by any
	// colour projection. It is not proof there is no code: a code invisible to
	// every projection is invisible to the detector too.
	NoQRFound Classification = "NO_QR_FOUND"
)

// Reason is a one-line human sentence for a classification, for a CLI that has
// one row per image. The web UI has its own translated copy; this is the
// English fallback and what the terminal shows.
func (c Classification) Reason() string {
	switch c {
	case Decoded:
		return "decoded"
	case QRDetectedDecodeFailed:
		return "QR detected, couldn't decode"
	case NoQRFound:
		return "no QR found"
	}
	return ""
}

// Box is an axis-aligned bounding box in ORIGINAL-image pixel coordinates —
// the same space hit.points live in, for the same reason. See the coordinate
// contract in hits.go. The detector that produces these runs on an unscaled,
// uncropped projection, so no mapping back is needed today; a future detector
// that scales must do the mapping at capture time.
type Box struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Detection is one located-but-unread QR code: where it is, how big its
// modules are, and which version it looks like.
type Detection struct {
	Box Box `json:"box"`
	// ModuleSize is the detector's estimate in pixels, averaged over the three
	// finder patterns.
	ModuleSize float64 `json:"module_size"`
	// Version is the estimated QR version, 1-40, or 0 when the geometry does
	// not yield a plausible one. It is an estimate from measured distances,
	// not a value read out of the code — the code did not decode.
	Version int `json:"version,omitempty"`
}

// Detail is a decode outcome together with the evidence behind it.
type Detail struct {
	Codes          []string       `json:"codes,omitempty"`
	Classification Classification `json:"classification"`
	// Detections is populated only for QRDetectedDecodeFailed. On a decoded
	// image the payload is the answer and a box adds nothing; on NoQRFound
	// there is nothing to box.
	Detections []Detection `json:"detections,omitempty"`
}

// classify picks the bucket. Order matters: any payload at all wins, because
// the user asked for payloads.
func classify(codes []string, infos []*detector.FinderPatternInfo) Classification {
	switch {
	case len(codes) > 0:
		return Decoded
	case len(infos) > 0:
		return QRDetectedDecodeFailed
	default:
		return NoQRFound
	}
}

// finderPadModules is how far a QR extends beyond its finder-pattern centres.
// A finder pattern is 7 modules across, so its centre sits 3.5 modules from the
// code's edge. Without this the box drawn from the three centres cuts the
// code's own corners off, which looks like a bug to anyone comparing it to the
// image.
const finderPadModules = 3.5

// detectionsFrom turns located finder-pattern triples into boxes, clamped to
// the image. bounds is the original image's rectangle.
func detectionsFrom(infos []*detector.FinderPatternInfo, bounds image.Rectangle) []Detection {
	var out []Detection
	for _, info := range infos {
		if d, ok := detectionOf(info, bounds); ok {
			out = append(out, d)
		}
	}
	return out
}

func detectionOf(info *detector.FinderPatternInfo, bounds image.Rectangle) (Detection, bool) {
	if info == nil {
		return Detection{}, false
	}
	tl, tr, bl := info.GetTopLeft(), info.GetTopRight(), info.GetBottomLeft()
	if tl == nil || tr == nil || bl == nil {
		return Detection{}, false
	}

	module := (tl.GetEstimatedModuleSize() + tr.GetEstimatedModuleSize() + bl.GetEstimatedModuleSize()) / 3
	if module <= 0 || math.IsNaN(module) || math.IsInf(module, 0) {
		return Detection{}, false
	}

	// The fourth corner is not detected — a QR has no finder pattern there —
	// so it is completed as a parallelogram. On a perspective-warped photo this
	// is an approximation, and an approximation that errs outward is the right
	// kind: the box is a hint about where to look, and a box slightly too big
	// still contains the code while one slightly too small does not.
	brX := tr.GetX() + bl.GetX() - tl.GetX()
	brY := tr.GetY() + bl.GetY() - tl.GetY()

	xs := []float64{tl.GetX(), tr.GetX(), bl.GetX(), brX}
	ys := []float64{tl.GetY(), tr.GetY(), bl.GetY(), brY}
	pad := finderPadModules * module

	minX, maxX := minMax(xs)
	minY, maxY := minMax(ys)
	x0 := clampInt(int(math.Floor(minX-pad)), bounds.Min.X, bounds.Max.X)
	y0 := clampInt(int(math.Floor(minY-pad)), bounds.Min.Y, bounds.Max.Y)
	x1 := clampInt(int(math.Ceil(maxX+pad)), bounds.Min.X, bounds.Max.X)
	y1 := clampInt(int(math.Ceil(maxY+pad)), bounds.Min.Y, bounds.Max.Y)
	if x1 <= x0 || y1 <= y0 {
		return Detection{}, false
	}

	return Detection{
		Box:        Box{X: x0, Y: y0, W: x1 - x0, H: y1 - y0},
		ModuleSize: module,
		Version:    estimateVersion(tl, tr, module),
	}, true
}

// estimateVersion recovers the QR version from measured geometry: the distance
// between the top-left and top-right finder centres spans (modules - 7) modules,
// since each centre sits 3.5 modules inside its edge, and a version-N QR is
// 17 + 4N modules across.
//
// It returns 0 rather than a guess when the arithmetic lands outside 1-40. The
// input is a code that failed to decode, so its geometry is by definition the
// geometry of something the decoder found confusing.
func estimateVersion(tl, tr *detector.FinderPattern, module float64) int {
	dx, dy := tr.GetX()-tl.GetX(), tr.GetY()-tl.GetY()
	modules := math.Hypot(dx, dy)/module + 7
	v := int(math.Round((modules - 17) / 4))
	if v < 1 || v > 40 {
		return 0
	}
	return v
}

func minMax(vs []float64) (float64, float64) {
	lo, hi := vs[0], vs[0]
	for _, v := range vs[1:] {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return lo, hi
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
