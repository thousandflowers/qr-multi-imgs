package scanner

// Headroom probe: how many raster failures are merely mirrored?
// The dataset generator applies Flip(p=0.1) and Transpose(p=0.1); both produce
// a mirrored QR, which a conforming decoder must reject. Mirroring is not
// damage — it is recoverable by retrying the flipped image.
//
//	QR_DATASET=/path/to/dataset go test ./scanner -run TestMirrorHeadroom -v

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func loadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	return img
}

func mirrorH(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(b.Max.X-1-(x-b.Min.X), y, src.At(x, y))
		}
	}
	return dst
}

func mirrorV(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(x, b.Max.Y-1-(y-b.Min.Y), src.At(x, y))
		}
	}
	return dst
}

func transposeImg(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(y-b.Min.Y, x-b.Min.X, src.At(x, y))
		}
	}
	return dst
}

func TestMirrorHeadroom(t *testing.T) {
	ds, limit := datasetDir(t)
	paths := listPNGs(ds, limit)

	var mu sync.Mutex
	var total, baseOK, failed int
	recovered := map[string]int{}
	var anyRecovered int

	runPar(paths, func(p string) {
		want := companionText(ds, p)
		if want == "" {
			return
		}
		base := decodeGoOnly(p) == want
		mu.Lock()
		total++
		if base {
			baseOK++
			mu.Unlock()
			return
		}
		failed++
		mu.Unlock()

		img := loadImage(p)
		if img == nil {
			return
		}
		hits := map[string]bool{}
		for name, variant := range map[string]image.Image{
			"mirrorH":   mirrorH(img),
			"mirrorV":   mirrorV(img),
			"transpose": transposeImg(img),
		} {
			if decodedContains(variant, want) {
				hits[name] = true
			}
		}
		if len(hits) == 0 {
			return
		}
		mu.Lock()
		anyRecovered++
		for k := range hits {
			recovered[k]++
		}
		mu.Unlock()
	})

	t.Logf("total=%d  baseline raster OK=%d (%.2f%%)  failures=%d",
		total, baseOK, 100*float64(baseOK)/float64(total), failed)
	for _, k := range []string{"mirrorH", "mirrorV", "transpose"} {
		t.Logf("  recovered by %-10s %d", k, recovered[k])
	}
	t.Logf("recovered by ANY mirror: %d of %d failures (%.1f%% of failures)",
		anyRecovered, failed, 100*float64(anyRecovered)/float64(failed))
	t.Logf("projected raster recall with mirror retry: %.2f%%",
		100*float64(baseOK+anyRecovered)/float64(total))
	_ = strings.TrimSpace
	_ = filepath.Base
}

// decodedContains reports whether want is among the payloads decoded from img.
func decodedContains(img image.Image, want string) bool {
	hits, err := decodeRaster(img)
	if err != nil {
		return false
	}
	for _, h := range hits {
		if h.text == want {
			return true
		}
	}
	return false
}
