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
	return decodeRaster(img)
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
