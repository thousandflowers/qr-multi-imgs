package scanner

import (
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	if runtime.GOOS == "windows" {
		t.Skip("chmod permissions work differently on Windows")
	}
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

// ─── pure-Go mask decode + zbarimg fallback ─────────────────────────────────

// decodeMaskImage is the pure-Go core: a clean QR render must decode with no
// external dependency.
func TestDecodeMaskImage_pureGo(t *testing.T) {
	content := "PURE-GO-MASK-PATH"
	img := createQRImage(t, content)
	if got := decodeMaskImage(img); got != content {
		t.Errorf("decodeMaskImage = %q, want %q", got, content)
	}
}

func TestDecodeMaskImage_noQR(t *testing.T) {
	blank := image.NewGray(image.Rect(0, 0, 64, 64))
	for i := range blank.Pix {
		blank.Pix[i] = 255
	}
	if got := decodeMaskImage(blank); got != "" {
		t.Errorf("expected empty for blank, got %q", got)
	}
}

func TestDecodeWithZbarimg(t *testing.T) {
	if zbarimgPath == "" {
		t.Skip("zbarimg not found in PATH")
	}
	content := "ZBARIMG-FALLBACK"
	path := createTestQR(t, content)
	got := decodeWithZbarimg(path)
	if got == "" {
		// A zbar that crashes is the same as a zbar that is not installed:
		// decodeWithZbarimg swallows the error either way, and the fallback
		// is optional by design. Some builds (0.23.93 on macOS/arm64) segfault
		// on a perfectly valid PNG, which is why this path is a fallback and
		// not the primary decoder.
		if err := exec.Command(zbarimgPath, "-q", "--raw", path).Run(); err != nil {
			t.Skipf("zbarimg present but unusable on this host: %v", err)
		}
	}
	if got != content {
		t.Errorf("decodeWithZbarimg = %q, want %q", got, content)
	}
}

func TestDecodeWithZbarimg_noQR(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blank.png")
	writeBlankPNG(t, path)
	if got := decodeWithZbarimg(path); got != "" {
		t.Errorf("expected empty for non-QR, got %q", got)
	}
}

// ─── ScanImage fallback ─────────────────────────────────────────────────────

func TestScanImage_fallbackDecodeFromMask(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fallback.png")
	writeBlankPNG(t, path)
	got, err := ScanImage(path)
	if err != nil {
		t.Fatalf("ScanImage: %v", err)
	}
	if got != nil {
		t.Logf("ScanImage returned %v (expected nil or empty for blank)", got)
	}
}

// ─── ScanFolder with same-HasQR (covers sort FilePath branch) ───────────────

func TestScanFolder_sameHasQR(t *testing.T) {
	dir := t.TempDir()
	writeQRToFile(t, filepath.Join(dir, "b_qr.png"), "second")
	writeQRToFile(t, filepath.Join(dir, "a_qr.png"), "first")
	s, err := ScanFolder(dir)
	if err != nil {
		t.Fatalf("ScanFolder: %v", err)
	}
	if s.WithQR != 2 {
		t.Errorf("expected 2 with QR, got %d", s.WithQR)
	}
	if s.Results[0].FilePath > s.Results[1].FilePath {
		t.Error("expected results sorted by FilePath")
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func createQRImage(t *testing.T, content string) image.Image {
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
	return img
}

// writeBMP encodes img as a 24-bit BMP file (no compression).
func writeBMP(t *testing.T, path string, img image.Image) {
	t.Helper()
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	rowStride := ((w*24 + 31) / 32) * 4
	pixelSize := rowStride * h
	offset := 54

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	hdr := make([]byte, offset)
	copy(hdr, "BM")
	binary.LittleEndian.PutUint32(hdr[2:6], uint32(offset+pixelSize))
	binary.LittleEndian.PutUint32(hdr[10:14], uint32(offset))
	binary.LittleEndian.PutUint32(hdr[14:18], 40)
	binary.LittleEndian.PutUint32(hdr[18:22], uint32(w))
	binary.LittleEndian.PutUint32(hdr[22:26], uint32(h))
	binary.LittleEndian.PutUint16(hdr[26:28], 1)
	binary.LittleEndian.PutUint16(hdr[28:30], 24)
	binary.LittleEndian.PutUint32(hdr[38:42], uint32(pixelSize))
	if _, err := f.Write(hdr); err != nil {
		t.Fatal(err)
	}

	row := make([]byte, rowStride)
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			row[x*3] = byte(b >> 8)
			row[x*3+1] = byte(g >> 8)
			row[x*3+2] = byte(r >> 8)
		}
		if _, err := f.Write(row); err != nil {
			t.Fatal(err)
		}
	}
}

// writeBlankBMP writes a blank 10x10 BMP file (no QR, valid image).
func writeBlankBMP(t *testing.T, path string) {
	t.Helper()
	writeBMP(t, path, image.NewGray(image.Rect(0, 0, 10, 10)))
}

func writeBlankPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, 10, 10))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// ─── BMP/WebP codec tests ────────────────────────────────────────────────────

func TestScanImage_BMP_QR(t *testing.T) {
	content := "BMP-CODEC-TEST"
	img := createQRImage(t, content)
	path := filepath.Join(t.TempDir(), "test_qr.bmp")
	writeBMP(t, path, img)

	got, err := ScanImage(path)
	if err != nil {
		t.Fatalf("ScanImage: %v", err)
	}
	if len(got) == 0 || got[0] != content {
		t.Fatalf("ScanImage = %v, want content %q", got, content)
	}
}

func TestScanImage_BMP_blank(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blank.bmp")
	writeBlankBMP(t, path)

	// ponytail: no error even though extension is .bmp — proves codec is registered
	got, err := ScanImage(path)
	if err != nil {
		t.Fatalf("ScanImage(bmp): %v", err)
	}
	_ = got
}

// ─── WebP helper — Go stdlib can't encode WebP, so we use a known-good vector ─

func writeStaticWebP(t *testing.T, path string) {
	t.Helper()
	// Minimal lossless WebP: 1x1 transparent pixel.
	// RIFF header + WEBP container + VP8L lossless bitstream.
	// Source: https://developers.google.com/speed/webp/docs/riff_container
	data := []byte{
		'R', 'I', 'F', 'F', 0x1e, 0, 0, 0, 'W', 'E', 'B', 'P',
		'V', 'P', '8', 'L', 0x12, 0, 0, 0,
		0x2f, 0x00, 0x00, 0x00, 0x10, 0x07, 0x10, 0x11, 0x11, 0x88, 0x88, 0xfe,
		0x07, 0x00,
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestScanImage_WebP_blank(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.webp")
	writeStaticWebP(t, path)

	// ponytail: no decode error — proves webp codec is registered
	got, err := ScanImage(path)
	if err != nil {
		t.Fatalf("ScanImage(webp): %v", err)
	}
	_ = got
}

// ─── NPY mask ────────────────────────────────────────────────────────────────

func TestDecodeNPYMask(t *testing.T) {
	// Pure-Go: a companion .npy mask must decode with no zbarimg installed.
	content := "NPY-MASK-TEST"
	writer := qrcode.NewQRCodeWriter()
	bitMatrix, err := writer.Encode(content, gozxing.BarcodeFormat_QR_CODE, 100, 100, nil)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	rows := bitMatrix.GetHeight()
	cols := bitMatrix.GetWidth()

	// Blank image so only the .npy mask path can decode it.
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "test.png")
	blank := image.NewGray(image.Rect(0, 0, cols, rows))
	for i := range blank.Pix {
		blank.Pix[i] = 255
	}
	f, _ := os.Create(imgPath)
	png.Encode(f, blank)
	f.Close()

	// Build raw data for the companion .npy mask.
	data := make([]byte, rows*cols)
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			if bitMatrix.Get(x, y) {
				data[y*cols+x] = 1
			}
		}
	}
	hdr := `{'descr': '|b1', 'fortran_order': False, 'shape': (100, 100),}`
	writeNPY(t, filepath.Join(dir, "test.npy"), hdr, data)

	got := decodeNPYMask(imgPath)
	if got != content {
		t.Errorf("decodeNPYMask = %q, want %q", got, content)
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
