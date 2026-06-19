package exporter

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"

	"github.com/thousandflowers/qr-multi-imgs/scanner"
)

// ExportFormat represents the output format for export.
type ExportFormat string

const (
	FormatJSON ExportFormat = "json"
	FormatCSV  ExportFormat = "csv"
	FormatTXT  ExportFormat = "txt"
)

// Export writes scan results to a file in the given format alongside the source directory.
// Returns the output file path.
func Export(results []scanner.ScanResult, format ExportFormat, sourceDir string) (string, error) {
	ts := time.Now().Format("20060102_150405")
	outPath := filepath.Join(sourceDir, fmt.Sprintf("qr_scan_%s.%s", ts, format))

	var err error
	switch format {
	case FormatJSON:
		err = writeJSON(results, outPath)
	case FormatCSV:
		err = writeCSV(results, outPath)
	case FormatTXT:
		err = writeTXT(results, outPath)
	default:
		return "", fmt.Errorf("unsupported export format %q", format)
	}
	if err != nil {
		return "", err
	}
	return outPath, nil
}

func writeJSON(results []scanner.ScanResult, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

func writeCSV(results []scanner.ScanResult, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)

	w.Write([]string{"file_path", "has_qr", "qr_contents", "error", "file_size"})
	for _, r := range results {
		w.Write([]string{
			r.FilePath,
			strconv.FormatBool(r.HasQR),
			strings.Join(r.Contents, " | "),
			r.Error,
			strconv.FormatInt(r.FileSize, 10),
		})
	}
	// Flush before reading w.Error(): a deferred Flush would run after the
	// return value is evaluated, silently dropping buffered-write failures.
	w.Flush()
	return w.Error()
}

func writeTXT(results []scanner.ScanResult, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, r := range results {
		var status string
		switch {
		case r.HasQR:
			status = "QR OK"
		case r.Error != "":
			status = "ERROR"
		default:
			status = "NO QR"
		}
		fmt.Fprintf(f, "[%s] %s\n", status, r.FilePath)
		if len(r.Contents) > 0 {
			for _, c := range r.Contents {
				fmt.Fprintf(f, "  content: %s\n", c)
			}
		}
		if r.Error != "" {
			fmt.Fprintf(f, "  error: %s\n", r.Error)
		}
	}
	return nil
}

// QRCodeFormat is the image format for generated QR code images.
type QRCodeFormat string

const (
	QRFormatPNG QRCodeFormat = "png"
	QRFormatJPG QRCodeFormat = "jpg"
	QRFormatSVG QRCodeFormat = "svg"
)

// WriteQRCode generates a 512x512 PNG QR code image at outputPath encoding content.
func WriteQRCode(content, outputPath string) error {
	return WriteQRCodeFormat(content, outputPath, QRFormatPNG)
}

func WriteQRCodeFormat(content, outputPath string, format QRCodeFormat) error {
	switch format {
	case QRFormatPNG, QRFormatJPG, QRFormatSVG:
	default:
		return fmt.Errorf("unsupported QR format %q", format)
	}

	writer := qrcode.NewQRCodeWriter()

	hints := map[gozxing.EncodeHintType]interface{}{
		gozxing.EncodeHintType_MARGIN: 2,
	}

	bitMatrix, err := writer.Encode(content, gozxing.BarcodeFormat_QR_CODE, 512, 512, hints)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	if format == QRFormatSVG {
		return writeQRCodeSVG(bitMatrix, content, outputPath)
	}

	return writeQRCodeImage(bitMatrix, outputPath, format)
}

func writeQRCodeImage(bitMatrix *gozxing.BitMatrix, outputPath string, format QRCodeFormat) error {
	w := bitMatrix.GetWidth()
	h := bitMatrix.GetHeight()
	img := image.NewGray(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if bitMatrix.Get(x, y) {
				img.SetGray(x, y, color.Gray{Y: 0})
			} else {
				img.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	switch format {
	case QRFormatJPG:
		return jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
	default:
		return png.Encode(f, img)
	}
}

func writeQRCodeSVG(bitMatrix *gozxing.BitMatrix, content, outputPath string) error {
	w := bitMatrix.GetWidth()
	h := bitMatrix.GetHeight()

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">
  <rect width="%d" height="%d" fill="#fff"/>
`, w, h, w, h, w, h))

	moduleSize := 1
	for y := 0; y < h; y += moduleSize {
		for x := 0; x < w; x += moduleSize {
			if bitMatrix.Get(x, y) {
				buf.WriteString(fmt.Sprintf(`  <rect x="%d" y="%d" width="%d" height="%d" fill="#000"/>
`, x, y, moduleSize, moduleSize))
			}
		}
	}
	buf.WriteString(`</svg>`)

	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}
