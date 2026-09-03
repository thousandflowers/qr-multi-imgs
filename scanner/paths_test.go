package scanner

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/tiff"
)

// A scan may now cover several folders and loose files at once, so the work
// list is what these tests are about: what gets in, what gets in twice, and
// what refuses to be silently dropped.

func TestCollectJobs_mixedTargets(t *testing.T) {
	dirA, dirB := t.TempDir(), t.TempDir()
	writeQRToFile(t, filepath.Join(dirA, "a.png"), "alpha")
	writeQRToFile(t, filepath.Join(dirB, "b.png"), "beta")
	loose := filepath.Join(t.TempDir(), "loose.png")
	writeQRToFile(t, loose, "gamma")

	jobs, err := collectJobs([]string{dirA, dirB, loose})
	if err != nil {
		t.Fatalf("collectJobs: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs across two folders and a file, got %d", len(jobs))
	}
}

func TestCollectJobs_dedupesFileInsideListedFolder(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.png")
	writeQRToFile(t, file, "alpha")

	jobs, err := collectJobs([]string{dir, file})
	if err != nil {
		t.Fatalf("collectJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("naming a folder and a file inside it should scan it once, got %d", len(jobs))
	}
}

// A named file is scanned whatever its extension, the caller asked for it by
// name, so guessing from the suffix would be second-guessing them. A folder
// walk still filters, because there nobody named the file.
func TestCollectJobs_namedFileIgnoresExtensionFilter(t *testing.T) {
	dir := t.TempDir()
	odd := filepath.Join(dir, "screenshot.qrz")
	writeQRToFile(t, odd, "alpha")

	fromFolder, err := collectJobs([]string{dir})
	if err != nil {
		t.Fatalf("collectJobs(folder): %v", err)
	}
	if len(fromFolder) != 0 {
		t.Fatalf("folder walk should skip an unsupported extension, got %d", len(fromFolder))
	}

	named, err := collectJobs([]string{odd})
	if err != nil {
		t.Fatalf("collectJobs(file): %v", err)
	}
	if len(named) != 1 {
		t.Fatalf("a named file should be scanned regardless of extension, got %d", len(named))
	}
}

func TestCollectJobs_missingPathIsAnError(t *testing.T) {
	dir := t.TempDir()
	writeQRToFile(t, filepath.Join(dir, "a.png"), "alpha")

	if _, err := collectJobs([]string{dir, filepath.Join(dir, "nope.png")}); err == nil {
		t.Fatal("a path the caller named but which does not exist must be an error, not a skip")
	}
}

func TestScanPaths_oneSummaryAcrossFolders(t *testing.T) {
	dirA, dirB := t.TempDir(), t.TempDir()
	writeQRToFile(t, filepath.Join(dirA, "a.png"), "alpha")
	writeQRToFile(t, filepath.Join(dirB, "b.png"), "beta")

	s, err := ScanPaths([]string{dirA, dirB})
	if err != nil {
		t.Fatalf("ScanPaths: %v", err)
	}
	if s.Total != 2 || s.WithQR != 2 {
		t.Fatalf("expected 2 files both with QR, got total=%d withQR=%d", s.Total, s.WithQR)
	}

	found := map[string]bool{}
	for _, r := range s.Results {
		for _, c := range r.Contents {
			found[c] = true
		}
	}
	if !found["alpha"] || !found["beta"] {
		t.Fatalf("both folders should contribute payloads, got %v", found)
	}
}

func TestScanPathsStream_countsEveryTarget(t *testing.T) {
	dir := t.TempDir()
	writeQRToFile(t, filepath.Join(dir, "a.png"), "alpha")
	loose := filepath.Join(t.TempDir(), "loose.png")
	writeQRToFile(t, loose, "beta")

	ch, err := ScanPathsStream([]string{dir, loose})
	if err != nil {
		t.Fatalf("ScanPathsStream: %v", err)
	}
	var last Progress
	events := 0
	for p := range ch {
		events++
		last = p
	}
	if last.Summary == nil {
		t.Fatal("stream must end with the summary event")
	}
	if last.Summary.Total != 2 {
		t.Fatalf("expected 2 files scanned, got %d", last.Summary.Total)
	}
	if events != 3 {
		t.Fatalf("expected 2 progress events plus the summary, got %d", events)
	}
}

// ScanFolder still rejects a file, which is the one behaviour ScanPaths drops.
func TestScanFolder_stillRejectsAFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "a.png")
	writeQRToFile(t, f, "alpha")

	if _, err := ScanFolder(f); err == nil {
		t.Fatal("ScanFolder on a file should error")
	}
	if _, err := ScanFolderStream(f); err == nil {
		t.Fatal("ScanFolderStream on a file should error")
	}
}

func TestScanPaths_emptyTargetList(t *testing.T) {
	s, err := ScanPaths(nil)
	if err != nil {
		t.Fatalf("ScanPaths(nil): %v", err)
	}
	if s.Total != 0 {
		t.Fatalf("expected an empty summary, got %d", s.Total)
	}
	if s.Results != nil {
		t.Fatal("an empty scan should leave Results nil so the export shape does not change")
	}
}

// TIFF is a listed extension, so a folder holding one must scan it rather than
// walk past it.
func TestScanPaths_tiffIsScanned(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.png")
	writeQRToFile(t, src, "tiffpayload")

	f, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}

	out, err := os.Create(filepath.Join(dir, "src.tiff"))
	if err != nil {
		t.Fatal(err)
	}
	if err := tiff.Encode(out, img, nil); err != nil {
		out.Close()
		t.Fatal(err)
	}
	out.Close()

	s, err := ScanPaths([]string{dir})
	if err != nil {
		t.Fatalf("ScanPaths: %v", err)
	}
	if s.Total != 1 {
		t.Fatalf("expected the TIFF to be picked up by the folder walk, got %d files", s.Total)
	}
	if s.WithQR != 1 || len(s.Results[0].Contents) == 0 || s.Results[0].Contents[0] != "tiffpayload" {
		t.Fatalf("expected the TIFF to decode, got %+v", s.Results[0])
	}
}
