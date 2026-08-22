package scanner

// Recall measurement harness, gated behind QR_DATASET so it never runs in CI.
//   QR_DATASET=~/Downloads/dataset go test ./scanner -run TestNPYRecall -v
//   QR_DATASET=~/Desktop/dataset  go test ./scanner -run TestCanonicalRecall -v
//
// Two layouts: flat (png+npy+txt co-located → npy path → 100%), and split
// (with_qr/ + without_qr/ subdirs, npy in root → image-only). The strategy
// search that picked the production decode set lives in git history; its
// conclusion is in scanner.go's strategies comment.

import (
	"image"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// decodeGoOnly runs the production pure-Go strategy loop (no npy, no zbarimg).
func decodeGoOnly(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return ""
	}
	hits, err := decodeRaster(img)
	if err != nil || len(hits) == 0 {
		return ""
	}
	return hits[0].text
}

func listPNGs(dir string, limit int) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".png") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// companionText reads the ground-truth payload at <dataset-root>/<id>.txt.
func companionText(ds, imgPath string) string {
	id := strings.TrimSuffix(filepath.Base(imgPath), filepath.Ext(imgPath))
	b, err := os.ReadFile(filepath.Join(ds, id+".txt"))
	if err != nil {
		return ""
	}
	// Strip only trailing newlines (tool/zbar artifact); preserve payload spaces.
	return strings.TrimRight(string(b), "\n\r")
}

// runPar runs fn over paths concurrently.
func runPar(paths []string, fn func(string)) {
	var idx int64
	var wg sync.WaitGroup
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				n := int(atomic.AddInt64(&idx, 1) - 1)
				if n >= len(paths) {
					return
				}
				fn(paths[n])
			}
		}()
	}
	wg.Wait()
}

func datasetDir(t *testing.T) (string, int) {
	t.Helper()
	ds := os.Getenv("QR_DATASET")
	if ds == "" {
		t.Skip("set QR_DATASET")
	}
	if strings.HasPrefix(ds, "~") {
		home, _ := os.UserHomeDir()
		ds = filepath.Join(home, ds[1:])
	}
	limit, _ := strconv.Atoi(os.Getenv("QR_LIMIT"))
	return ds, limit
}

// TestNPYRecall measures the full pipeline on a flat dataset (image/.npy/.txt
// co-located). This is the path designed to reach 100%.
func TestNPYRecall(t *testing.T) {
	ds, limit := datasetDir(t)
	paths := listPNGs(ds, limit)

	var mu sync.Mutex
	var npyOK, fullOK, total int
	runPar(paths, func(p string) {
		want := companionText(ds, p)
		if want == "" {
			return
		}
		npy := decodeNPYMask(p) // raw, to match exact payload
		got, _ := ScanImage(p)
		full := len(got) > 0 && got[0] == want
		mu.Lock()
		total++
		if npy == want {
			npyOK++
		}
		if full {
			fullOK++
		}
		mu.Unlock()
	})
	t.Logf("total=%d", total)
	t.Logf("npy-only       correct %d/%d = %.2f%%", npyOK, total, 100*float64(npyOK)/float64(total))
	t.Logf("full ScanImage correct %d/%d = %.2f%%", fullOK, total, 100*float64(fullOK)/float64(total))
}

// TestScanFolderStream checks the streaming scan emits one progress event per
// file plus a final summary, and that the summary matches the blocking scan.
func TestScanFolderStream(t *testing.T) {
	ds, _ := datasetDir(t)
	ch, err := ScanFolderStream(ds)
	if err != nil {
		t.Fatal(err)
	}
	var progress, total int
	var summary *Summary
	for p := range ch {
		if p.Summary != nil {
			summary = p.Summary
			total = p.Total
		} else {
			progress++
		}
	}
	if summary == nil {
		t.Fatal("no summary emitted")
	}
	if progress != total {
		t.Errorf("progress events %d != total %d", progress, total)
	}
	want, _ := ScanFolder(ds)
	if summary.Total != want.Total {
		t.Errorf("stream Total %d != ScanFolder Total %d", summary.Total, want.Total)
	}
}

// TestCanonicalRecall measures image-only recall on a split dataset, per folder.
// with_qr are clean renders, without_qr are degraded — the headroom lives there.
func TestCanonicalRecall(t *testing.T) {
	ds, limit := datasetDir(t)
	var totHit, totN int
	for _, name := range []string{"with_qr", "without_qr"} {
		paths := listPNGs(filepath.Join(ds, name), limit)
		var hit, n int
		var mu sync.Mutex
		runPar(paths, func(p string) {
			want := companionText(ds, p)
			if want == "" {
				return
			}
			ok := decodeGoOnly(p) == want
			mu.Lock()
			n++
			if ok {
				hit++
			}
			mu.Unlock()
		})
		totHit += hit
		totN += n
		t.Logf("%-11s match %d/%d = %.1f%%", name, hit, n, 100*float64(hit)/float64(n))
	}
	t.Logf("OVERALL    match %d/%d = %.1f%%", totHit, totN, 100*float64(totHit)/float64(totN))
}

// TestRasterRecall measures the pure-Go raster path on a flat dataset and
// cross-tabs it against module density, read from each sample's .npy shape.
// This is the measurement behind the README's image-only figure, and the one
// that shows density alone is not what defeats the decoder — distortion is.
//
//	QR_DATASET=/path/to/dataset go test ./scanner -run TestRasterRecall -v
func TestRasterRecall(t *testing.T) {
	ds, limit := datasetDir(t)
	paths := listPNGs(ds, limit)

	type bucket struct{ raster, npy, n int }
	buckets := map[string]*bucket{}
	order := []string{"<=40", "41-60", "61-80", "81-100", "101-120", ">120"}
	for _, k := range order {
		buckets[k] = &bucket{}
	}
	bucketFor := func(modules int) string {
		switch {
		case modules <= 40:
			return "<=40"
		case modules <= 60:
			return "41-60"
		case modules <= 80:
			return "61-80"
		case modules <= 100:
			return "81-100"
		case modules <= 120:
			return "101-120"
		default:
			return ">120"
		}
	}

	var mu sync.Mutex
	var rasterOK, npyOK, total int
	runPar(paths, func(p string) {
		want := companionText(ds, p)
		if want == "" {
			return
		}
		modules := 0
		if mask, err := readBoolNPY(strings.TrimSuffix(p, filepath.Ext(p)) + ".npy"); err == nil {
			modules = len(mask)
		}
		r := decodeGoOnly(p) == want
		n := decodeNPYMask(p) == want

		mu.Lock()
		defer mu.Unlock()
		total++
		if r {
			rasterOK++
		}
		if n {
			npyOK++
		}
		if modules > 0 {
			b := buckets[bucketFor(modules)]
			b.n++
			if r {
				b.raster++
			}
			if n {
				b.npy++
			}
		}
	})

	t.Logf("total=%d", total)
	t.Logf("raster only (pure Go, no npy/Vision/zbar): %d/%d = %.2f%%", rasterOK, total, 100*float64(rasterOK)/float64(total))
	t.Logf("npy mask only:                             %d/%d = %.2f%%", npyOK, total, 100*float64(npyOK)/float64(total))
	t.Logf("%-9s %7s %10s %10s", "modules", "samples", "raster", "npy")
	for _, k := range order {
		b := buckets[k]
		if b.n == 0 {
			continue
		}
		t.Logf("%-9s %7d %9.1f%% %9.1f%%", k, b.n,
			100*float64(b.raster)/float64(b.n), 100*float64(b.npy)/float64(b.n))
	}
}
