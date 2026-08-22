//go:build darwin && cgo

package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

// A PDF is not a raster format: Go's image.Decode cannot open one and ImageIO
// does not list it, so the pages are rendered by Core Graphics and read from
// there. The fixture holds one QR per page, payloads "page-one" and "page-two".
const twoPagePDF = "testdata/vision_qr_2pages.pdf"

func TestPDF_isScannable(t *testing.T) {
	if !SupportsExtension("invoice.pdf") {
		t.Fatal("a folder scan should pick up a PDF on macOS")
	}
}

func TestPDF_everyPageIsRead(t *testing.T) {
	got, err := ScanImage(twoPagePDF)
	if err != nil {
		t.Fatalf("ScanImage(pdf): %v", err)
	}
	want := map[string]bool{"page-one": true, "page-two": true}
	for _, c := range got {
		delete(want, c)
	}
	if len(want) != 0 {
		t.Fatalf("stopping at the first page under-reports: got %v, still missing %v", got, want)
	}
}

// The whole point of rendering onto white: a QR in a PDF is black vector art on
// no background, and drawn onto an uninitialised bitmap it would be black on
// black. If that regressed the page would decode as nothing at all.
func TestPDF_decodesThroughAScan(t *testing.T) {
	s, err := ScanPaths([]string{twoPagePDF})
	if err != nil {
		t.Fatalf("ScanPaths: %v", err)
	}
	if s.Total != 1 {
		t.Fatalf("expected the one named PDF, got %d", s.Total)
	}
	if !s.Results[0].HasQR {
		t.Fatalf("the PDF should report codes, got %+v", s.Results[0])
	}
	if len(s.Results[0].Contents) != 2 {
		t.Fatalf("expected both pages' codes, got %v", s.Results[0].Contents)
	}
}

func TestPDF_unreadableFileStillErrors(t *testing.T) {
	broken := filepath.Join(t.TempDir(), "broken.pdf")
	if err := os.WriteFile(broken, []byte("%PDF-1.4 this is not a pdf"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ScanImage(broken); err == nil {
		t.Fatal("a file that no decoder can open must still report an error")
	}
}
