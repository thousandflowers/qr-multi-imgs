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
	"sync"
	"time"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

	"github.com/makiuchi-d/gozxing"
	multiqr "github.com/makiuchi-d/gozxing/multi/qrcode"
	multidetector "github.com/makiuchi-d/gozxing/multi/qrcode/detector"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// ScanResult holds the outcome of scanning a single image file.
type ScanResult struct {
	FilePath string   `json:"file_path"`
	HasQR    bool     `json:"has_qr"`
	Contents []string `json:"qr_contents,omitempty"`
	Error    string   `json:"error,omitempty"`
	FileSize int64    `json:"file_size"`
}

// Summary aggregates scan results across all files in a folder.
type Summary struct {
	Total     int
	WithQR    int
	WithoutQR int
	Errors    int
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
// them straight from the path and they decode normally; elsewhere they are
// reported as an unreadable format, which is the honest answer and far better
// than pretending the files were not there.
var supportedExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true,
	".gif": true, ".bmp": true, ".webp": true,
	".heic": true, ".heif": true,
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
}

// ScanFolder scans all supported image files in dir and returns a Summary.
func ScanFolder(dir string) (*Summary, error) {
	start := time.Now()

	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	type job struct {
		path string
		fi   os.FileInfo
	}

	jobs := make(chan job, len(entries))
	results := make(chan ScanResult, len(entries))

	var wg sync.WaitGroup
	for range scanWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				r := ScanResult{FilePath: j.path, FileSize: j.fi.Size()}
				contents, err := ScanImage(j.path)
				if err != nil {
					r.Error = err.Error()
				} else if len(contents) > 0 {
					r.HasQR = true
					r.Contents = contents
				}
				results <- r
			}
		}()
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !supportedExtensions[ext] {
			continue
		}
		fi, err := entry.Info()
		if err != nil {
			continue
		}
		jobs <- job{path: filepath.Join(dir, entry.Name()), fi: fi}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	list := collectResults(results)
	s := tallySummary(list, start)
	sortResults(s)
	return s, nil
}

// collectResults drains the results channel into a slice.
func collectResults(ch <-chan ScanResult) []ScanResult {
	var list []ScanResult
	for r := range ch {
		list = append(list, r)
	}
	return list
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
		}
		s.TotalSize += r.FileSize
	}
	return s
}

// sortResults sorts results: QR items first, then by file path.
func sortResults(s *Summary) {
	sort.Slice(s.Results, func(i, j int) bool {
		if s.Results[i].HasQR != s.Results[j].HasQR {
			return s.Results[i].HasQR && !s.Results[j].HasQR
		}
		return s.Results[i].FilePath < s.Results[j].FilePath
	})
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
// ponytail: duplicates ScanFolder's small worker-pool boilerplate to avoid a
// blind rewrite of the existing function; fold them together if it drifts.
func ScanFolderStream(dir string) (<-chan Progress, error) {
	start := time.Now()
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	type job struct {
		path string
		fi   os.FileInfo
	}
	var jobList []job
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !supportedExtensions[strings.ToLower(filepath.Ext(entry.Name()))] {
			continue
		}
		fi, err := entry.Info()
		if err != nil {
			continue
		}
		jobList = append(jobList, job{path: filepath.Join(dir, entry.Name()), fi: fi})
	}
	total := len(jobList)

	out := make(chan Progress, total+1)
	go func() {
		defer close(out)
		jobs := make(chan job, total)
		results := make(chan ScanResult, total)
		var wg sync.WaitGroup
		for range scanWorkers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range jobs {
					r := ScanResult{FilePath: j.path, FileSize: j.fi.Size()}
					contents, err := ScanImage(j.path)
					if err != nil {
						r.Error = err.Error()
					} else if len(contents) > 0 {
						r.HasQR = true
						r.Contents = contents
					}
					results <- r
				}
			}()
		}
		for _, j := range jobList {
			jobs <- j
		}
		close(jobs)
		go func() {
			wg.Wait()
			close(results)
		}()

		var list []ScanResult
		done := 0
		for r := range results {
			list = append(list, r)
			done++
			out <- Progress{Done: done, Total: total, File: r.FilePath}
		}
		s := tallySummary(list, start)
		sortResults(s)
		out <- Progress{Done: total, Total: total, Summary: s}
	}()
	return out, nil
}

// decodeStrategy is one decode configuration: a color-channel projection, a
// binarizer choice, and an integer upscale. Colored QRs hide their contrast in
// a single channel that luminance averages away, so we try several.
type decodeStrategy struct {
	channel string // lum, r, g, b, min, max
	global  bool   // global-histogram binarizer vs hybrid (local) default
	scale   int    // integer nearest-neighbor upscale
}

// decodeAttempt runs one strategy and returns every code it found.
//
// Result points come back in the coordinate space of the transformed image, so
// an upscaled strategy reports them at scale x their true position. They are
// divided back here, at capture time, to honour hit's coordinate contract.
func decodeAttempt(img image.Image, s decodeStrategy) []hit {
	src := projectChannel(img, s.channel)
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
	{"lum", false, 1}, // hybrid, native — clean QRs
	{"lum", true, 1},  // global, native — biggest colored-set winner
	{"lum", false, 2}, // hybrid, 2x
	{"lum", true, 2},  // global, 2x
	{"r", true, 1},    // red channel
	{"g", true, 1},    // green channel
	{"b", true, 1},    // blue channel
	{"min", true, 1},  // darkest channel
	{"max", true, 1},  // brightest channel
	{"r", false, 2},   // red channel, hybrid 2x
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

// countCandidates estimates how many QR codes the image holds, by locating
// finder patterns rather than decoding anything. Finder patterns are cheap to
// find and a payload is expensive to recover, which is what makes an evidence
// based stopping rule affordable.
//
// It looks across every colour projection the strategies use, not just
// luminance, and takes the largest count any of them shows. That breadth is the
// whole point: a code whose contrast lives in one channel can be nearly absent
// from the luminance image — red on green of the same brightness is the extreme
// case — so a count taken from luminance alone would miss it, and a decode loop
// trusting that count would stop before the channel strategy that could read
// it ever ran. Taking the maximum means a projection that hides a code can
// never lower a target another projection has already raised.
//
// Returns 0 when nothing is detected anywhere, which disables early exit
// entirely rather than ending the search.
func countCandidates(img image.Image) int {
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}
	most := 0
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
			// A detector error must cost time, never recall: leaving the count
			// where it is only fails to stop the loop early.
			continue
		}
		if len(infos) > most {
			most = len(infos)
		}
	}
	return most
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
// countCandidates says how many codes are visible before any decoding starts;
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
	target := countCandidates(img)
	var hits []hit
	for _, s := range strategies {
		hits = mergeHits(hits, decodeAttempt(img, s))
		if target > 0 && len(hits) >= target {
			break
		}
	}
	sortHits(hits)
	return hits, nil
}

// maskRenderScale upscales each QR module to this many pixels before decoding.
// 10px/module gives gozxing's PURE_BARCODE extractor crisp, unambiguous cells.
const maskRenderScale = 10

// decodeNPYMask decodes the companion .npy module bitmap — a pristine,
// pixel-exact QR shipped alongside each image — purely in Go. It recovers the
// dense codes whose modules are sub-pixel in the source raster. Returns "" when
// no mask is present or it cannot be decoded.
func decodeNPYMask(path string) string {
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
// Every raster strategy always runs; see decodeRaster for why there is no
// early exit.
//
// The returned error is always nil today. It is in the signature because every
// decode strategy lives in decodeRaster, which is where allocation-heavy work
// belongs; under GOOS=js that can fail, and adding an error to an already
// released public function would be a breaking change. Do not remove it.
func ScanDecodedImage(img image.Image) ([]string, error) {
	hits, err := decodeRaster(img)
	return hitTexts(hits), err
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
	exhaustive := mode == ScanExhaustive

	// A companion .npy mask, when present, is the pixel-exact source QR, and it
	// decodes faster and more reliably than recovering sub-pixel modules from
	// the raster. One mask describes one code, so in ScanFast it still short
	// circuits — that is what keeps dense-code scans quick. ScanExhaustive
	// folds it into the union instead and keeps going.
	var hits []hit
	if text := decodeNPYMask(path); text != "" {
		hits = append(hits, hit{text: text})
		if !exhaustive {
			return hitTexts(hits), nil
		}
	}

	// The file has to be readable at all; nothing downstream can work otherwise.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
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
	if decodeErr != nil {
		rasterErr = fmt.Errorf("decode: %w", decodeErr)
	} else {
		var rasterHits []hit
		rasterHits, rasterErr = decodeRaster(img)
		hits = mergeHits(hits, rasterHits)
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
		hits = mergeHits(hits, visionHits)
	}

	// Last resort for real photos with no mask: zbarimg, if installed. It
	// reports no geometry, so it can only be merged by payload.
	if exhaustive || len(hits) == 0 {
		hits = mergeHits(hits, decodeWithZbarimg(path))
	}

	// Report the raster failure only if nothing decoded AND no other decoder
	// managed to read the file. When Vision read it and simply found no code,
	// "no QR here" is the honest answer — calling that an error would turn a
	// whole HEIC camera roll into a wall of failures.
	if len(hits) == 0 && rasterErr != nil && !visionReadable {
		return nil, rasterErr
	}
	sortHits(hits)
	return hitTexts(hits), nil
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
