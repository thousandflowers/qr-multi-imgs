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

func TestScanFolder_withImages(t *testing.T) {
	dir := t.TempDir()
	qrContent := "https://example.com"
	qrPath := filepath.Join(dir, "qr.png")
	writeQRToFile(t, qrPath, qrContent)
	plain := image.NewGray(image.Rect(0, 0, 50, 50))
	plainPath := filepath.Join(dir, "plain.png")
	f, _ := os.Create(plainPath)
	png.Encode(f, plain)
	f.Close()
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0644)

	s, err := ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder: %v", err)
	}
	if s.Total != 2 {
		t.Errorf("expected 2 images scanned, got %d", s.Total)
	}
	if s.WithQR != 1 {
		t.Errorf("expected 1 with QR, got %d", s.WithQR)
	}
	if s.WithoutQR != 1 {
		t.Errorf("expected 1 without QR, got %d", s.WithoutQR)
	}
	if s.TotalSize <= 0 {
		t.Error("expected TotalSize > 0")
	}
	if s.Results[0].HasQR && !s.Results[0].HasQR {
		t.Error("first result should be the QR file")
	}
}

func writeQRToFile(t *testing.T, path, content string) {
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
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
}

func TestScanFolder_nonexistentDir(t *testing.T) {
	_, err := ScanFolder("/tmp/nonexistent_scan_folder_test")
	if err == nil {
		t.Fatal("expected error for non-existent dir")
	}
}

func TestScanFolder_fileInsteadOfDir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "afile")
	os.WriteFile(f, []byte("x"), 0644)
	_, err := ScanFolder(f)
	if err == nil {
		t.Fatal("expected error when path is a file")
	}
}

func TestScanFolder_emptyDir(t *testing.T) {
	dir := t.TempDir()
	s, err := ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder on empty dir: %v", err)
	}
	if s.Total != 0 {
		t.Errorf("expected 0 images, got %d", s.Total)
	}
}

func TestScanFolder_skipsSubdirs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "subdir", "test.png"), []byte("dummy"), 0644)
	s, err := ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder: %v", err)
	}
	if s.Total != 0 {
		t.Errorf("expected 0 (subdir skipped), got %d", s.Total)
	}
}

func TestScanImage_nonImageFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "notanimage.txt")
	os.WriteFile(f, []byte("this is not an image"), 0644)
	_, err := ScanImage(f)
	if err == nil {
		t.Fatal("expected decode error for non-image file")
	}
}

func corruptPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, 1, 1))
	img.SetGray(0, 0, color.Gray{Y: 128})
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestScanFolder_corruptImage(t *testing.T) {
	dir := t.TempDir()
	corruptPNG(t, filepath.Join(dir, "blank.png"))
	s, err := ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder: %v", err)
	}
	if s.Total != 1 {
		t.Errorf("expected 1 file, got %d", s.Total)
	}
	if s.WithoutQR != 1 {
		t.Errorf("expected 1 WithoutQR, got %d", s.WithoutQR)
	}
}

func TestScanFolder_unreadableDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0000); err != nil {
		t.Skipf("chmod failed: %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0755) })

	_, err := ScanFolder(dir)
	if err == nil {
		t.Fatal("expected error for unreadable directory")
	}
}

func TestScanFolder_scanImageError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "garbage.png"), []byte("not a real png at all"), 0644)
	s, err := ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder: %v", err)
	}
	if s.Total != 1 {
		t.Errorf("expected 1 file, got %d", s.Total)
	}
	if s.Errors != 1 {
		t.Errorf("expected 1 error result, got %d", s.Errors)
	}
	if s.Results[0].Error == "" {
		t.Error("expected non-empty Error on result")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		b    int64
		want string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1610612736, "1.5 GB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		got := FormatBytes(tt.b)
		if got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.b, got, tt.want)
		}
	}
}
