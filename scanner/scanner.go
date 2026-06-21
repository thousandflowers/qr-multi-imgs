package scanner

import (
	"fmt"
	"image"
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
var supportedExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true,
	".gif": true, ".bmp": true, ".webp": true,
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

func decodeAttempt(img image.Image, useGlobal bool, scale int) string {
	srcImg := img
	if scale > 1 {
		srcImg = nearestNeighborScale(img, scale)
	}

	var bmp *gozxing.BinaryBitmap
	if useGlobal {
		src := gozxing.NewLuminanceSourceFromImage(srcImg)
		bin := gozxing.NewGlobalHistgramBinarizer(src)
		bmp, _ = gozxing.NewBinaryBitmap(bin)
	} else {
		bmp, _ = gozxing.NewBinaryBitmapFromImage(srcImg)
	}
	if bmp == nil {
		return ""
	}

	reader := qrcode.NewQRCodeReader()
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}
	result, err := reader.Decode(bmp, hints)
	if err != nil || result == nil {
		return ""
	}
	return result.GetText()
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

// ponytail: 8 combos — each binarizer × 1x-4x. Small 256×256 QR modules often
// need upscaling. Hybrid first (better for typical QR), lower scale first.
var strategies = [][2]int{
	{0, 1}, // hybrid, native
	{0, 2}, // hybrid, 2x
	{0, 3}, // hybrid, 3x
	{0, 4}, // hybrid, 4x
	{1, 1}, // global, native
	{1, 2}, // global, 2x
	{1, 3}, // global, 3x
	{1, 4}, // global, 4x
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

// decodeWithZbarimg shells out to zbarimg on the original image — a best-effort
// fallback for real photos that ship no mask, used only when zbarimg is
// installed. The pure-Go paths above mean it is never required.
func decodeWithZbarimg(path string) string {
	if zbarimgPath == "" {
		return ""
	}
	if out, err := exec.Command(zbarimgPath, "-q", "--raw", path).Output(); err == nil && len(out) > 0 {
		return strings.TrimRight(string(out), "\n\r")
	}
	return ""
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
			return strings.TrimSpace(res.GetText())
		}
	}
	return ""
}

// ScanImage tries multiple decode strategies in order until one succeeds.
func ScanImage(path string) ([]string, error) {
	// A companion .npy mask, when present, is the pixel-exact source QR. It
	// decodes faster and more reliably than recovering sub-pixel modules from
	// the raster, so try it first — this is what keeps dense-code scans quick.
	if text := decodeNPYMask(path); text != "" {
		return []string{text}, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	for _, s := range strategies {
		global := s[0] == 1
		scale := s[1]
		if text := decodeAttempt(img, global, scale); text != "" {
			return []string{strings.TrimSpace(text)}, nil
		}
	}

	// Last resort for real photos with no mask: zbarimg, if installed.
	if text := decodeWithZbarimg(path); text != "" {
		return []string{text}, nil
	}

	return nil, nil
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
