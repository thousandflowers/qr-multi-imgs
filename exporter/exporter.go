package exporter

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"

	"qr-multi-imgs/scanner"
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
	outPath := fmt.Sprintf("%s/qr_scan_%s.%s", sourceDir, ts, format)

	var err error
	switch format {
	case FormatJSON:
		err = writeJSON(results, outPath)
	case FormatCSV:
		err = writeCSV(results, outPath)
	case FormatTXT:
		err = writeTXT(results, outPath)
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
	defer w.Flush()

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

// WriteQRCode generates a 512x512 PNG QR code image at outputPath encoding content.
func WriteQRCode(content, outputPath string) error {
	writer := qrcode.NewQRCodeWriter()

	hints := map[gozxing.EncodeHintType]interface{}{
		gozxing.EncodeHintType_MARGIN: 2,
	}

	bitMatrix, err := writer.Encode(content, gozxing.BarcodeFormat_QR_CODE, 512, 512, hints)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}

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

	return png.Encode(f, img)
}
