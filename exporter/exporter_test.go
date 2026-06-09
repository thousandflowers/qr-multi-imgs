package exporter

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thousandflowers/qr-multi-imgs/scanner"
)

func sampleResults() []scanner.ScanResult {
	return []scanner.ScanResult{
		{FilePath: "a.png", HasQR: true, Contents: []string{"https://example.com"}, FileSize: 100},
		{FilePath: "b.png", HasQR: false, FileSize: 50},
		{FilePath: "c.png", HasQR: false, Error: "decode failed", FileSize: 10},
	}
}

func TestExportJSON_roundTrip(t *testing.T) {
	dir := t.TempDir()

	path, err := Export(sampleResults(), FormatJSON, dir)
	if err != nil {
		t.Fatalf("Export(json) error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("output file not readable: %v", err)
	}
	var got []scanner.ScanResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 results, got %d", len(got))
	}
	if got[0].Contents[0] != "https://example.com" {
		t.Errorf("contents lost in round trip: %+v", got[0])
	}
}

func TestExportCSV_headerAndRows(t *testing.T) {
	dir := t.TempDir()

	path, err := Export(sampleResults(), FormatCSV, dir)
	if err != nil {
		t.Fatalf("Export(csv) error: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("output file not readable: %v", err)
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("output is not valid CSV: %v", err)
	}
	if len(rows) != 4 { // header + 3 rows
		t.Fatalf("want 4 rows, got %d", len(rows))
	}
	wantHeader := []string{"file_path", "has_qr", "qr_contents", "error", "file_size"}
	for i, h := range wantHeader {
		if rows[0][i] != h {
			t.Errorf("header[%d] = %q, want %q", i, rows[0][i], h)
		}
	}
	if rows[3][3] != "decode failed" {
		t.Errorf("error column = %q, want %q", rows[3][3], "decode failed")
	}
}

func TestExportTXT_statusLines(t *testing.T) {
	dir := t.TempDir()

	path, err := Export(sampleResults(), FormatTXT, dir)
	if err != nil {
		t.Fatalf("Export(txt) error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("output file not readable: %v", err)
	}
	text := string(data)
	for _, want := range []string{"[QR OK] a.png", "[NO QR] b.png", "[ERROR] c.png",
		"content: https://example.com", "error: decode failed"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q\n---\n%s", want, text)
		}
	}
}

func TestExport_unknownFormat_returnsError(t *testing.T) {
	dir := t.TempDir()

	path, err := Export(sampleResults(), ExportFormat("xml"), dir)
	if err == nil {
		t.Fatalf("Export(xml) = %q, nil — want error for unsupported format", path)
	}
}

func TestExport_unwritableDir_returnsError(t *testing.T) {
	if _, err := Export(sampleResults(), FormatJSON, filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("Export into non-existent dir: want error, got nil")
	}
}

func TestWriteQRCode_roundTripThroughScanner(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "qr.png")
	const content = "hello-qr-round-trip"

	if err := WriteQRCode(content, out); err != nil {
		t.Fatalf("WriteQRCode error: %v", err)
	}

	decoded, err := scanner.ScanImage(out)
	if err != nil {
		t.Fatalf("generated QR not scannable: %v", err)
	}
	if len(decoded) != 1 || decoded[0] != content {
		t.Fatalf("round trip = %v, want [%s]", decoded, content)
	}
}
