package scanner

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

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

// ponytail: parallel worker pool — numWorkers = min(numCPU, 6). M2 Pro has 8P+4E,
// but each worker does PNG encoding + zbarimg exec, so ~6 saturates.
var scanWorkers = func() int {
	n := runtime.NumCPU()
	if n > 6 {
		n = 6
	}
	return n
}()

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

	var list []ScanResult
	for r := range results {
		list = append(list, r)
	}

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

	sort.Slice(s.Results, func(i, j int) bool {
		if s.Results[i].HasQR != s.Results[j].HasQR {
			return s.Results[i].HasQR && !s.Results[j].HasQR
		}
		return s.Results[i].FilePath < s.Results[j].FilePath
	})

	return s, nil
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

// ponytail: mask catches dense codes — keep gozxing only for native/small-QR cases.
var strategies = [][2]int{
	{0, 1}, // hybrid, native
	{1, 2}, // global, 2x
}

// ponytail: renders companion .npy at 10x, pipes PNG via stdin to zbarimg.
// Avoids temp file I/O by using /dev/stdin pipe — ~30ms per image vs ~70ms with temp file.
func decodeFromMask(path string) string {
	npyPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".npy"
	if _, err := os.Stat(npyPath); os.IsNotExist(err) {
		return ""
	}
	mask, err := readBoolNPY(npyPath)
	if err != nil {
		return ""
	}

	img := renderMask(mask, 10)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}

	cmd := exec.Command("/opt/homebrew/bin/zbarimg", "-q", "--raw", "/dev/stdin")
	cmd.Stdin = &buf
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return ""
	}
	return strings.TrimRight(string(out), "\n\r")
}

// ScanImage tries multiple decode strategies in order until one succeeds.
func ScanImage(path string) ([]string, error) {
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
		text := decodeAttempt(img, global, scale)
		if text != "" {
			return []string{strings.TrimSpace(text)}, nil
		}
	}

	if text := decodeFromMask(path); text != "" {
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
