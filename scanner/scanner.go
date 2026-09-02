package scanner

import (
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"github.com/makiuchi-d/gozxing"
	multiqr "github.com/makiuchi-d/gozxing/multi/qrcode"
	multidetector "github.com/makiuchi-d/gozxing/multi/qrcode/detector"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/makiuchi-d/gozxing/qrcode/detector"
)

// ScanResult holds the outcome of scanning a single image file.
//
// Classification and Detections are additive: every field that was here before
// still means what it did, so an existing consumer of the JSON export reads the
// same rows it always has. What is new is why a row without a code has no code.
type ScanResult struct {
	FilePath string   `json:"file_path"`
	HasQR    bool     `json:"has_qr"`
	Contents []string `json:"qr_contents,omitempty"`
	Error    string   `json:"error,omitempty"`
	FileSize int64    `json:"file_size"`
	// Classification is empty for a file that could not be read at all — see
	// ScanImageDetail. Every other row carries one.
	Classification Classification `json:"classification,omitempty"`
	// Detections is populated only for QRDetectedDecodeFailed.
	Detections []Detection `json:"detections,omitempty"`
	// Metadata describes the image and what was tried on it. It is annotation
	// and never a filter - see metadata.go. Absent for a file that could not be
	// read at all.
	Metadata *Metadata `json:"metadata,omitempty"`
}

// Summary aggregates scan results across all files in a folder.
type Summary struct {
	Total     int
	WithQR    int
	WithoutQR int
	Errors    int
	// Detected counts the images inside WithoutQR that hold a QR code the
	// decoder located and could not read. It is a subset of WithoutQR, not a
	// fourth bucket beside it: the file still has no payload, which is what
	// every existing count and action means by "without".
	Detected  int
	TotalSize int64
	Results   []ScanResult
	StartTime time.Time
	Duration  time.Duration
}

// supportedExtensions is the set of image file extensions we attempt to scan.
//
// HEIC/HEIF are listed even though Go's image.Decode cannot read them. Every
// iPhone has shot HEIC by default since 2017, so skipping the extension would
// silently ignore most of a modern camera roll. On macOS, Apple Vision reads
// them straight from the path and they decode normally; elsewhere they need a
// build with -tags heic, and without it they report an unreadable format, which
// is the honest answer and far better than pretending the files were not there.
//
// NEF is a MACOS-ONLY path and is listed on the same reasoning. Core Image
// reads Nikon raw directly, so those files scan on macOS; nothing in the pure
// Go path decodes them and -tags heic does not help, since raw demosaicing and
// per-camera colour are a different problem from a HEIF container. On Linux and
// Windows a .nef reports an unreadable format. That is expected, not a bug.
var supportedExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true,
	".gif": true, ".bmp": true, ".webp": true,
	".tif": true, ".tiff": true,
	".heic": true, ".heif": true,
	".nef": true,
}

// zbarimgPath is resolved at init so decodeWithZbarimg works regardless
// of where zbarimg lives in PATH (not hardcoded to Homebrew).
var zbarimgPath string

// ponytail: parallel worker pool — numWorkers = min(numCPU, 6). M2 Pro has 8P+4E,
// but each worker does PNG encoding + zbarimg exec, so ~6 saturates.
var scanWorkers = func() int {
	n := runtime.NumCPU()
	if n > 6 {
		n = 6
	}
	return n
}()

func init() {
	zbarimgPath, _ = exec.LookPath("zbarimg")

	// On macOS the system decodes more than the portable codecs do — every
	// camera raw ImageIO knows, and whatever a future OS adds. Merging what it
	// reports beats maintaining a list that goes stale silently.
	for _, ext := range systemExtensions() {
		supportedExtensions["."+ext] = true
	}
}

// SupportsExtension reports whether a file with this name would be picked up by
// a folder scan. The answer depends on the platform and, on macOS, on what the
// running system can decode, so callers must ask rather than keep their own
// copy of the list.
func SupportsExtension(name string) bool {
	return supportedExtensions[strings.ToLower(filepath.Ext(name))]
}

// ScanFolder scans all supported image files in dir and returns a Summary.
// It is ScanPaths over one directory, kept because a folder is still the common
// case and because it rejects a file path, which ScanPaths deliberately allows.
func ScanFolder(dir string) (*Summary, error) {
	if err := mustBeDir(dir); err != nil {
		return nil, err
	}
	return ScanPaths([]string{dir})
}

// tallySummary computes summary counts from the result list.
func tallySummary(list []ScanResult, start time.Time) *Summary {
	s := &Summary{
		Total:     len(list),
		StartTime: start,
		Duration:  time.Since(start),
		Results:   list,
	}
	for _, r := range list {
		switch {
		case r.HasQR:
			s.WithQR++
		case r.Error != "":
			s.Errors++
		default:
			s.WithoutQR++
			if r.Classification == QRDetectedDecodeFailed {
				s.Detected++
			}
		}
		s.TotalSize += r.FileSize
	}
	return s
}

// sortResults puts the images that decoded first, then everything else, and
// orders by file path within each group.
//
// This is deliberately NOT the order the web UI uses — see rowRank in
// web/order.js, which leads with the images holding a code nobody could read.
// The two surfaces want different things and each pins its own order in its
// own test:
//
//   - In the terminal nothing is per-row. Every action the list offers (e
//     export, d delete, o organize, r recreate) works on the whole set, never
//     on the row under the cursor, so leading with the failures buys a reader
//     nothing they can use.
//   - It also costs them the answer. viewList renders 25 rows from the cursor,
//     and on a real camera roll — a couple of hundred photos, most with no code
//     in them — ranking NO_QR_FOUND above the decoded rows pushes every single
//     payload past the first screen. The summary box has already given the
//     counts by then; the list is where the payloads are read.
//
// In the browser the row IS the unit of work — it carries the thumbnail, the
// rename preview and the filter tab that selects it — which is why triage
// order is right there and wrong here.
func sortResults(s *Summary) {
	sort.Slice(s.Results, func(i, j int) bool {
		a, b := s.Results[i], s.Results[j]
		if ra, rb := resultRank(a), resultRank(b); ra != rb {
			return ra < rb
		}
		return a.FilePath < b.FilePath
	})
}

// resultRank orders the buckets for the terminal list. See sortResults for why
// this is two buckets and not four, and why it is not what the web does.
//
// Classification still reaches the row — viewList marks a located-but-unread
// image [QR?] and names the reason — it just does not move it. Saying what a
// row is and deciding where it sits are separate jobs.
func resultRank(r ScanResult) int {
	if r.HasQR {
		return 0
	}
	return 1
}

// ─── multi-strategy decoding ─────────────────────────────────────────────────

// Progress is one folder-scan progress event. Summary is non-nil only on the
// final event; before that, File is the path just finished (Done of Total).
type Progress struct {
	Done, Total int
	File        string
	Summary     *Summary
}

// ScanFolderStream scans like ScanFolder but emits a Progress per finished file
// on the returned channel, ending with one event carrying the Summary. Path
// validation is synchronous so errors surface before scanning starts.
func ScanFolderStream(dir string) (<-chan Progress, error) {
	if err := mustBeDir(dir); err != nil {
		return nil, err
	}
	return ScanPathsStream([]string{dir})
}

// decodeStrategy is one decode configuration: a color-channel projection, a
// binarizer choice, and an integer upscale. Colored QRs hide their contrast in
// a single channel that luminance averages away, so we try several.
type decodeStrategy struct {
	channel string // lum, r, g, b, min, max
	global  bool   // global-histogram binarizer vs hybrid (local) default
	scale   int    // integer nearest-neighbor upscale
	// prep names a pixel transform from the transforms registry in
	// binarize.go, applied after the channel projection and before any
	// upscale. Empty means the pixels go to gozxing as they are.
	//
	// Before the upscale rather than after, for two reasons: a local threshold
	// wants its window measured in source modules, not in interpolated copies
	// of them, and running the transform on a quarter of the pixels is a
	// quarter of the work.
	prep string
}

// decodeAttempt runs one strategy and returns every code it found.
//
// Result points come back in the coordinate space of the transformed image, so
// an upscaled strategy reports them at scale x their true position. They are
// divided back here, at capture time, to honour hit's coordinate contract.
func decodeAttempt(img image.Image, s decodeStrategy) []hit {
	src := projectChannel(img, s.channel)
	if s.prep != "" {
		t, ok := transforms[s.prep]
		if !ok {
			// A strategy naming a transform that does not exist is a
			// programming error, and skipping the attempt is how it costs
			// recall instead of the scan. TestStrategyPrepsExist catches it
			// before it can get here.
			return nil
		}
		src = t(src)
	}
	if s.scale > 1 {
		src = nearestNeighborScale(src, s.scale)
	}

	var bmp *gozxing.BinaryBitmap
	if s.global {
		lum := gozxing.NewLuminanceSourceFromImage(src)
		bmp, _ = gozxing.NewBinaryBitmap(gozxing.NewGlobalHistgramBinarizer(lum))
	} else {
		bmp, _ = gozxing.NewBinaryBitmapFromImage(src)
	}
	if bmp == nil {
		return nil
	}

	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}

	results, err := multiqr.NewQRCodeMultiReader().DecodeMultiple(bmp, hints)
	if err != nil {
		return nil
	}

	hits := make([]hit, 0, len(results))
	for _, r := range results {
		if r == nil {
			continue
		}
		// Payload is returned exactly as decoded — trimming would corrupt QRs
		// whose payload is or ends with whitespace.
		hits = append(hits, hit{text: r.GetText(), points: rescalePoints(r.GetResultPoints(), s.scale)})
	}
	return hits
}

// rescalePoints maps result points from an upscaled image back to original
// coordinates. See hit's coordinate contract.
func rescalePoints(points []gozxing.ResultPoint, scale int) []gozxing.ResultPoint {
	if scale <= 1 || len(points) == 0 {
		return points
	}
	f := float64(scale)
	out := make([]gozxing.ResultPoint, len(points))
	for i, p := range points {
		out[i] = gozxing.NewResultPoint(p.GetX()/f, p.GetY()/f)
	}
	return out
}

// projectChannel returns img unchanged for "lum" (gozxing computes luminance),
// otherwise a grayscale image holding the chosen color channel. Single-channel
// projection recovers colored QRs whose contrast luminance would wash out.
func projectChannel(img image.Image, channel string) image.Image {
	if channel == "lum" {
		return img
	}
	b := img.Bounds()
	dst := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			r, g, bl = r>>8, g>>8, bl>>8
			var v uint32
			switch channel {
			case "r":
				v = r
			case "g":
				v = g
			case "b":
				v = bl
			case "max":
				v = maxu(maxu(r, g), bl)
			case "min":
				v = minu(minu(r, g), bl)
			}
			dst.SetGray(x, y, color.Gray{Y: uint8(v)})
		}
	}
	return dst
}

func maxu(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

func minu(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// nearest-neighbor preserves hard QR edges; bilinear/bicubic would blur small modules.
func nearestNeighborScale(src image.Image, factor int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx()*factor, b.Dy()*factor
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, src.At(x/factor, y/factor))
		}
	}
	return dst
}

// strategies are tried in order until one decodes. Order from a greedy cover
// over a colored-QR benchmark: luminance hybrid+global crack the clean codes,
// per-channel global-histogram recovers gradient-colored ones. No PURE_BARCODE
// (useless on noisy renders) and no upscale past 2x (no marginal recall, ~3x
// slower). Measured: 25.7% -> 27.3% on the hard set, easy set stays ~100%.
var strategies = []decodeStrategy{
	{channel: "lum", scale: 1},               // hybrid, native — clean QRs
	{channel: "lum", scale: 1, global: true}, // global, native — biggest colored-set winner
	{channel: "lum", scale: 2},               // hybrid, 2x
	{channel: "lum", scale: 2, global: true}, // global, 2x
	{channel: "r", scale: 1, global: true},   // red channel
	{channel: "g", scale: 1, global: true},   // green channel
	{channel: "b", scale: 1, global: true},   // blue channel
	{channel: "min", scale: 1, global: true}, // darkest channel
	{channel: "max", scale: 1, global: true}, // brightest channel
	{channel: "r", scale: 2},                 // red channel, hybrid 2x

	// The rungs below were added after the ten above, and they run last on
	// purpose. The ten are a measured greedy cover; appending means an image
	// that already decoded takes exactly the path it took before, so a recall
	// delta is attributable to the new rungs and nothing else. Their own order
	// among themselves is not measured and is not a ranking.
	//
	// A transform outputs a two-valued image, so the binarizer after it has
	// nothing left to decide and the cheapest one is the right one.
	{channel: "lum", scale: 1, global: true, prep: "sauvola"}, // local threshold — glare, gradients, shadow
	{channel: "lum", scale: 1, global: true, prep: "otsu"},    // variance-optimal global threshold
	{channel: "lum", scale: 1, prep: "invert"},                // light-on-dark codes the detector cannot see
}

// projectionChannels are the distinct colour projections the strategies use,
// derived from the strategy list so the two cannot drift apart.
func projectionChannels() []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range strategies {
		if !seen[s.channel] {
			seen[s.channel] = true
			out = append(out, s.channel)
		}
	}
	return out
}

// detectFinders locates QR finder patterns without decoding anything. Finder
// patterns are cheap to find and a payload is expensive to recover, which is
// what makes an evidence based stopping rule affordable.
//
// It looks across every colour projection the strategies use, not just
// luminance, and keeps the triples from whichever projection shows the most.
// That breadth is the whole point: a code whose contrast lives in one channel
// can be nearly absent from the luminance image — red on green of the same
// brightness is the extreme case — so a count taken from luminance alone would
// miss it, and a decode loop trusting that count would stop before the channel
// strategy that could read it ever ran. Taking the largest set means a
// projection that hides a code can never lower a target another projection has
// already raised.
//
// The geometry, not only the count, is returned. An image that failed to decode
// with finder patterns plainly located is a different answer from an image with
// nothing in it, and these triples are what let a caller tell the two apart —
// see classify.go. Until this returned a bare count, and every box was thrown
// away the moment it was computed.
//
// Returns nil when nothing is detected anywhere, which disables early exit
// entirely rather than ending the search.
//
// Result points are in original-image coordinates: every projection here is
// unscaled and uncropped, so nothing needs mapping back. A projection that
// scaled would have to map at capture time — see the contract in hits.go.
func detectFinders(img image.Image) []*detector.FinderPatternInfo {
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}
	var best []*detector.FinderPatternInfo
	for _, channel := range projectionChannels() {
		src := projectChannel(img, channel)
		// Global histogram: cheapest binarizer, and the most forgiving on the
		// flat single-channel projections this pass is built around.
		bmp, err := gozxing.NewBinaryBitmap(gozxing.NewGlobalHistgramBinarizer(
			gozxing.NewLuminanceSourceFromImage(src)))
		if err != nil || bmp == nil {
			continue
		}
		matrix, merr := bmp.GetBlackMatrix()
		if merr != nil || matrix == nil {
			continue
		}
		infos, ferr := multidetector.NewMultiFinderPatternFinder(matrix, nil).FindMulti(hints)
		if ferr != nil {
			// A detector error must cost time, never recall: leaving the set
			// where it is only fails to stop the loop early.
			//
			// The partial centres behind this error are deliberately not used.
			// FindMulti does populate them before failing — GetPossibleCenters
			// returns what it had — so they are there for the taking. Measured,
			// they also fire where there is nothing: a uniform-noise image with
			// no code in it yields one. A bucket built on them would promise a
			// code the image does not contain, which is the failure this whole
			// change exists to stop, pointed the other way.
			continue
		}
		if len(infos) > len(best) {
			best = infos
		}
	}
	return best
}

// decodeRaster runs the pure-Go strategies over a decoded image and returns
// the union of what they found, deduped and in reading order.
//
// The strategy list is a greedy cover, not a ranking: strategies are
// complementary, so one finding nothing says nothing about the next. Stopping
// at the first success would return whichever subset the first working strategy
// happened to see — a silent under-report, indistinguishable from a frame that
// really held one code. Running all ten instead costs roughly thirty times the
// old best case on real photos, which is why the loop stops on evidence.
//
// detectFinders says how many codes are visible before any decoding starts;
// once that many have been decoded there is nothing left to look for. The
// guards are that at least one strategy always runs, and that a count of zero
// disables early exit rather than triggering it, so uncertainty costs time
// instead of recall.
//
// This is evidence about what the detector can see, not proof about what is on
// the page: a code invisible to every projection is invisible to the count too.
// It is strictly better than stopping at the first success, and it is what
// makes the union affordable.
func decodeRaster(img image.Image) ([]hit, error) {
	hits, _, _, err := decodeRasterDetail(img)
	return hits, err
}

// decodeRasterDetail is decodeRaster plus the finder-pattern triples the pass
// already located. Two entry points rather than one because decodeRaster's
// shape is load-bearing across the package and its tests, and a caller that
// only wants payloads should not have to name a value it will discard.
func decodeRasterDetail(img image.Image) ([]hit, []*detector.FinderPatternInfo, []StrategyAttempt, error) {
	infos := detectFinders(img)
	target := len(infos)
	var hits []hit
	// Attempts are recorded whether or not they found anything: a rung that ran
	// and found nothing is the measurement that gets it removed later, and it
	// is unrecoverable after the fact. Early exit means the list is also a
	// record of where the loop stopped.
	attempts := make([]StrategyAttempt, 0, len(strategies))
	for _, s := range strategies {
		found := decodeAttempt(img, s)
		attempts = append(attempts, StrategyAttempt{Name: s.name(), Found: len(found)})
		hits = mergeHits(hits, found)
		if target > 0 && len(hits) >= target {
			break
		}
	}
	sortHits(hits)
	return hits, infos, attempts, nil
}

// maskRenderScale upscales each QR module to this many pixels before decoding.
// 10px/module gives gozxing's PURE_BARCODE extractor crisp, unambiguous cells.
const maskRenderScale = 10

// npyMaskEnabled gates the companion-mask short circuit below.
//
// It exists for one caller: the corpus harness, which defaults to ignoring
// masks. A mask is the generator's own answer key sitting beside the image, so
// a benchmark that reads it measures a path on which no decoding happens - the
// dataset this project quotes ships one beside all 3332 images, and the
// headline throughput taken over them is the speed of parsing answers, not of
// reading codes. Ignoring masks is what makes that number a decoder number.
//
// It is a package variable rather than a parameter because the alternative is a
// third ScanMode or a changed public signature, for a switch only an in-package
// test ever flips. Not safe to change while a scan is running; the harness is
// sequential and sets it once.
var npyMaskEnabled = true

// decodeNPYMask decodes the companion .npy module bitmap — a pristine,
// pixel-exact QR shipped alongside each image — purely in Go. It recovers the
// dense codes whose modules are sub-pixel in the source raster. Returns "" when
// no mask is present or it cannot be decoded.
func decodeNPYMask(path string) string {
	if !npyMaskEnabled {
		return ""
	}
	npyPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".npy"
	mask, err := readBoolNPY(npyPath)
	if err != nil {
		return ""
	}
	return decodeMaskImage(renderMask(mask, maskRenderScale))
}

// decodeWithZbarimg shells out to zbarimg on the original image — a
// best-effort fallback for real photos that ship no mask, used only when
// zbarimg is installed. The pure-Go paths mean it is never required.
//
// zbarimg prints one line per code, so every line is returned. It reports no
// coordinates with --raw, so its hits carry no geometry and are merged by
// payload; see sameCode.
func decodeWithZbarimg(path string) []hit {
	if zbarimgPath == "" {
		return nil
	}
	out, err := exec.Command(zbarimgPath, "-q", "--raw", path).Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	// A trailing newline terminates the last code rather than starting an
	// empty one, so it is stripped before splitting.
	text := strings.TrimRight(string(out), "\n\r")
	if text == "" {
		return nil
	}
	var hits []hit
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimRight(line, "\r"); line != "" {
			hits = append(hits, hit{text: line})
		}
	}
	return hits
}

// decodeMaskImage decodes a clean, axis-aligned QR render using gozxing. It
// tries the normal detector first, then the PURE_BARCODE extractor, which
// cracks the high-density grids the normal finder-pattern search misses.
func decodeMaskImage(img image.Image) string {
	for _, pure := range []bool{false, true} {
		bmp, err := gozxing.NewBinaryBitmapFromImage(img)
		if err != nil || bmp == nil {
			continue
		}
		hints := map[gozxing.DecodeHintType]interface{}{
			gozxing.DecodeHintType_TRY_HARDER: true,
		}
		if pure {
			hints[gozxing.DecodeHintType_PURE_BARCODE] = true
		}
		if res, derr := qrcode.NewQRCodeReader().Decode(bmp, hints); derr == nil && res != nil {
			return res.GetText() // exact payload; do not trim whitespace
		}
	}
	return ""
}

// ScanDecodedImage decodes an already-decoded image and returns every QR
// payload in it, in reading order, or nil if there are none.
//
// It is the path-free half of ScanImage. The companion .npy mask, Apple Vision,
// and zbarimg all need a file path, so none of them run here — expect lower
// recall than ScanImage on real photos, where on macOS Vision is what rescues
// the warped and low-contrast ones. This is the entry point for callers holding
// pixels but no file, such as a wasm build.
//
// Every raster strategy always runs unless the finder-pattern count says every
// visible code has been decoded; see decodeRaster.
//
// This is also the answer for HEIC in a browser. gen2brain/heic does run under
// GOOS=js, but at roughly 4.5 minutes per 12MP photo against 0.1s native, so a
// wasm build should not decode HEIC in Go at all: let the browser do it with
// createImageBitmap and pass the pixels here. That is this seam's first
// concrete payoff — a caller holding pixels and no file, exactly as intended.
//
// The returned error is always nil today. It is in the signature because every
// decode strategy lives in decodeRaster, which is where allocation-heavy work
// belongs; under GOOS=js that can fail, and adding an error to an already
// released public function would be a breaking change. Do not remove it.
func ScanDecodedImage(img image.Image) ([]string, error) {
	hits, err := decodeRaster(img)
	return hitTexts(hits), err
}

// ScanDecodedImageDetail is ScanDecodedImage plus the reason behind the answer:
// which of the three buckets this image fell into, and where the codes are when
// they were located but could not be read.
//
// It exists because "no QR found" is the wrong thing to tell someone looking at
// a photo of a QR code, and the evidence to say something truer was already
// being computed and discarded. This is the entry point a UI wants; the plain
// one stays for callers that only need payloads.
func ScanDecodedImageDetail(img image.Image) (Detail, error) {
	hits, infos, attempts, err := decodeRasterDetail(img)
	codes := hitTexts(hits)
	d := Detail{Codes: codes, Classification: classify(codes, infos)}
	// Boxes for both outcomes here, unlike the path-based scan: this is the
	// seam a viewfinder decodes through, and it needs to draw every code it can
	// see, in one colour for the ones it read and another for the ones it only
	// found.
	if d.Classification == QRDetectedDecodeFailed {
		d.Detections = detectionsFrom(infos, img.Bounds())
	} else {
		d.Detections = detectionsFromHits(hits, img.Bounds())
	}
	d.Metadata = metadataOf(img, attempts)
	return d, err
}

// ScanMode selects how much work ScanImageMode does before giving up.
//
// There are exactly two modes and neither carries an expected number of codes.
// That omission is deliberate: telling the decoder how many codes to find and
// then measuring how many it found is circular, and would let a benchmark
// report a recall figure it was handed rather than one it earned. Keeping the
// count out of the type makes that mistake unavailable rather than discouraged.
type ScanMode int

const (
	// ScanFast unions every raster strategy, then falls back to Apple Vision
	// and zbarimg only when the raster path found nothing at all. This is what
	// the CLI uses: it keeps folder scans quick on the common case of one code
	// per image.
	ScanFast ScanMode = iota

	// ScanExhaustive runs every stage unconditionally and unions all of them,
	// with no early exit anywhere. Much slower — Vision alone costs about a
	// second per image — and meant for building ground truth, where being
	// right matters more than being quick.
	ScanExhaustive
)

// ScanImage decodes every QR code in the image at path, in reading order,
// using ScanFast. It returns nil with no error when the image holds no code.
func ScanImage(path string) ([]string, error) {
	return ScanImageMode(path, ScanFast)
}

// ScanImageMode decodes every QR code in the image at path under the given
// mode. See ScanMode for what the modes cost.
func ScanImageMode(path string, mode ScanMode) ([]string, error) {
	d, err := ScanImageDetail(path, mode)
	return d.Codes, err
}

// ScanImageDetail is ScanImageMode plus the reason behind the answer. See
// Classification: the interesting case is an image whose finder patterns were
// located and whose payload no strategy could recover, which until now was
// reported as an image with no code in it.
//
// A file that could not be read at all returns an error and no Detail, exactly
// as before. An unreadable file has no classification — "no QR found" would be
// a claim about pixels nobody ever saw.
func ScanImageDetail(path string, mode ScanMode) (Detail, error) {
	exhaustive := mode == ScanExhaustive

	// A companion .npy mask, when present, is the pixel-exact source QR, and it
	// decodes faster and more reliably than recovering sub-pixel modules from
	// the raster. One mask describes one code, so in ScanFast it still short
	// circuits — that is what keeps dense-code scans quick. ScanExhaustive
	// folds it into the union instead and keeps going.
	var hits []hit
	meta := &Metadata{}
	if text := decodeNPYMask(path); text != "" {
		hits = append(hits, hit{text: text})
		meta.record(strategyNPYMask, 1)
		if !exhaustive {
			// No raster was ever decoded on this path, so the image measures
			// stay zero. They are reported beside dimensions of zero, which is
			// what says "not measured" rather than "measured as flat".
			return Detail{Codes: hitTexts(hits), Classification: Decoded, Metadata: meta}, nil
		}
	}

	// The file has to be readable at all; nothing downstream can work otherwise.
	f, err := os.Open(path)
	if err != nil {
		return Detail{}, err
	}
	img, _, decodeErr := image.Decode(f)
	f.Close()

	// Neither a raster decode failure nor a raster capacity failure is a "no QR"
	// answer, so neither may collapse the cascade: the error is remembered and
	// the path-based decoders still get their turn, because one of them may well
	// succeed where the raster could not even start. It is reported only if
	// nothing decodes at all.
	//
	// This matters well beyond exotic files. Go's image decoders do not cover
	// HEIC, which every iPhone has shot by default since 2017, so image.Decode
	// fails on most of a modern camera roll. Apple Vision reads those straight
	// from the path, so on macOS they decode fine — but only if a failure here
	// lets the scan continue instead of ending it.
	//
	// It is also the contract the allocation-heavy cascade stages (CLAHE,
	// multiscale tiling) will land into: a strategy that runs out of room must
	// cost recall, not the whole scan.
	var rasterErr error
	var infos []*detector.FinderPatternInfo
	if decodeErr != nil {
		rasterErr = fmt.Errorf("decode: %w", decodeErr)
	} else {
		var rasterHits []hit
		var attempts []StrategyAttempt
		rasterHits, infos, attempts, rasterErr = decodeRasterDetail(img)
		hits = mergeHits(hits, rasterHits)
		b := img.Bounds()
		meta.Width, meta.Height = b.Dx(), b.Dy()
		meta.LaplacianVariance, meta.EdgeDensity = measure(img)
		meta.Strategies = append(meta.Strategies, attempts...)
	}

	// Real photos (warped/low-contrast/small-module QRs) defeat gozxing. On
	// macOS, Apple Vision recovers many of them and ships with the OS; it is a
	// no-op elsewhere. Try it before zbarimg, which is weaker and can segfault.
	//
	// TODO(cascade): this gate is where ScanFast trades recall for speed, and
	// the gap between it and ScanExhaustive is exactly what Vision recovers.
	// Closing it provably rather than by guesswork is what finder-pattern
	// accounting is for: a cheap MultiFinderPatternFinder pass over the image
	// says how many codes are present, so escalation can stop when the decoded
	// count matches the detected count instead of when a stage happens to
	// return something.
	visionReadable := false
	if exhaustive || len(hits) == 0 {
		var visionHits []hit
		visionHits, visionReadable = decodeWithVision(path)
		// Only recorded where it exists. A stage logged as "ran, found 0" on a
		// build that compiled it out reads as a stage that was tried and
		// failed, which is a different and false claim.
		if visionAvailable {
			meta.record(strategyVision, len(visionHits))
		}
		hits = mergeHits(hits, visionHits)
	}

	// Last resort for real photos with no mask: zbarimg, if installed. It
	// reports no geometry, so it can only be merged by payload.
	if exhaustive || len(hits) == 0 {
		zbarHits := decodeWithZbarimg(path)
		if zbarimgPath != "" {
			meta.record(strategyZbarimg, len(zbarHits))
		}
		hits = mergeHits(hits, zbarHits)
	}

	// Report the raster failure only if nothing decoded AND no other decoder
	// managed to read the file. When Vision read it and simply found no code,
	// "no QR here" is the honest answer — calling that an error would turn a
	// whole HEIC camera roll into a wall of failures.
	if len(hits) == 0 && rasterErr != nil && !visionReadable {
		return Detail{}, rasterErr
	}
	sortHits(hits)

	codes := hitTexts(hits)
	d := Detail{Codes: codes, Classification: classify(codes, infos), Metadata: meta}
	// Vision and zbarimg report no finder patterns, so an image they read has
	// no geometry to box even when it decoded. Boxes are only drawn for the
	// bucket that has nothing else to show.
	if d.Classification == QRDetectedDecodeFailed && img != nil {
		d.Detections = detectionsFrom(infos, img.Bounds())
	}
	return d, nil
}

// FormatBytes returns a human-readable string for a byte count.
func FormatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
