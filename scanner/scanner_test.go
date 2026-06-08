package scanner

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// createTestQR generates a temporary PNG with the given content.
func createTestQR(t *testing.T, content string) string {
	t.Helper()
	writer := qrcode.NewQRCodeWriter()
	bitMatrix, err := writer.Encode(content, gozxing.BarcodeFormat_QR_CODE, 256, 256, nil)
	if err != nil {
		t.Fatalf("encode %q: %v", content, err)
	}

	w, h := bitMatrix.GetWidth(), bitMatrix.GetHeight()
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

	path := filepath.Join(t.TempDir(), "test_qr.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return path
}

func TestScanImage(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"simple URL", "https://example.com"},
		{"text with numbers", "HELLO-WORLD-12345"},
		{"WiFi config", "WIFI:T:WPA;S:test;P:pass;;"},
		{"long text", "Two roads diverged in a yellow wood,\nAnd sorry I could not travel both"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := createTestQR(t, tt.content)
			got, err := ScanImage(path)
			if err != nil {
				t.Fatalf("ScanImage(%s) error: %v", path, err)
			}
			if len(got) == 0 {
				t.Fatal("ScanImage: no QR code found")
			}
			if got[0] != tt.content {
				t.Errorf("ScanImage = %q, want %q", got[0], tt.content)
			}
		})
	}
}

func TestScanImageNoQR(t *testing.T) {
	plain := image.NewGray(image.Rect(0, 0, 50, 50))
	path := filepath.Join(t.TempDir(), "no_qr.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, plain); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	got, err := ScanImage(path)
	if err != nil {
		t.Fatalf("ScanImage error on non-QR image: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil result for non-QR image, got %v", got)
	}
}

func TestScanImageNonExistent(t *testing.T) {
	got, err := ScanImage("/tmp/nonexistent_qr_file.png")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
	if got != nil {
		t.Errorf("expected nil result, got %v", got)
	}
}
