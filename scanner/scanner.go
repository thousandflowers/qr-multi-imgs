package scanner

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

	var results []ScanResult
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !supportedExtensions[ext] {
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())
		fi, err := entry.Info()
		if err != nil {
			continue
		}

		r := ScanResult{
			FilePath: fullPath,
			FileSize: fi.Size(),
		}

		contents, err := ScanImage(fullPath)
		if err != nil {
			r.Error = err.Error()
		} else if len(contents) > 0 {
			r.HasQR = true
			r.Contents = contents
		}

		results = append(results, r)
	}

	s := &Summary{
		Total:     len(results),
		StartTime: start,
		Duration:  time.Since(start),
		Results:   results,
	}
	for _, r := range results {
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

// ScanImage attempts to decode exactly one QR code from the image at path.
// Returns the decoded text(s) or nil if no QR was found.
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

	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return nil, fmt.Errorf("bitmap: %w", err)
	}

	reader := qrcode.NewQRCodeReader()
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}

	result, err := reader.Decode(bmp, hints)
	if err != nil || result == nil {
		return nil, nil
	}

	return []string{result.GetText()}, nil
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
