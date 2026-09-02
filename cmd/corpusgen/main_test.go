package main

import (
	"encoding/csv"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// writeQR renders a QR carrying content to path, creating parent directories.
func writeQR(t *testing.T, path, content string) {
	t.Helper()
	m, err := qrcode.NewQRCodeWriter().Encode(content, gozxing.BarcodeFormat_QR_CODE, 256, 256, nil)
	if err != nil {
		t.Fatalf("encode %q: %v", content, err)
	}
	w, h := m.GetWidth(), m.GetHeight()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := uint8(255)
			if m.Get(x, y) {
				v = 0
			}
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	writePNG(t, path, img)
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
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

// writeMultiQR renders several QRs side by side in one image.
func writeMultiQR(t *testing.T, path string, contents ...string) {
	t.Helper()
	const cell, gap = 300, 150
	w := len(contents)*cell + (len(contents)+1)*gap
	dst := image.NewGray(image.Rect(0, 0, w, cell+2*gap))
	for i := range dst.Pix {
		dst.Pix[i] = 255
	}
	for i, content := range contents {
		m, err := qrcode.NewQRCodeWriter().Encode(content, gozxing.BarcodeFormat_QR_CODE, cell, cell, nil)
		if err != nil {
			t.Fatalf("encode %q: %v", content, err)
		}
		ox := gap + i*(cell+gap)
		for y := range m.GetHeight() {
			for x := range m.GetWidth() {
				v := uint8(255)
				if m.Get(x, y) {
					v = 0
				}
				dst.SetGray(ox+x, gap+y, color.Gray{Y: v})
			}
		}
	}
	writePNG(t, path, dst)
}

// writeBlank writes an image with no QR in it.
func writeBlank(t *testing.T, path string) {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, 32, 32))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	writePNG(t, path, img)
}

// readManifest parses a generated manifest exactly the way TestCorpus does.
func readManifest(t *testing.T, path string) map[string][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open manifest: %v", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comment = '#'
	r.FieldsPerRecord = 2
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	got := make(map[string][]string, len(rows))
	for _, row := range rows {
		if row[0] == "path" && row[1] == "expected" {
			continue
		}
		got[row[0]] = append(got[row[0]], row[1])
	}
	return got
}

// The payloads a QR can legitimately carry that CSV has to survive.
var trickyPayloads = map[string]string{
	"comma.png":   "has,commas,in,it",
	"quote.png":   `he said "hello"`,
	"newline.png": "line one\nline two",
	"both.png":    `a,b "c" d` + "\ne",
}

func TestGenerateManifest(t *testing.T) {
	root := t.TempDir()

	// Same base name in two different directories: relative paths keep them apart.
	writeQR(t, filepath.Join(root, "a", "dup.png"), "payload-from-a")
	writeQR(t, filepath.Join(root, "b", "dup.png"), "payload-from-b")

	// Nested subdirectory.
	writeQR(t, filepath.Join(root, "deep", "deeper", "nested.png"), "nested-payload")

	// Payloads that CSV has to quote.
	for name, payload := range trickyPayloads {
		writeQR(t, filepath.Join(root, name), payload)
	}

	// A path beginning with '#', which csv.Reader treats as a comment unless
	// the writer quotes it.
	writeQR(t, filepath.Join(root, "#hash.png"), "hash-payload")

	// Several codes in one frame become several rows sharing a path, and two
	// codes carrying identical payloads must stay two rows.
	writeMultiQR(t, filepath.Join(root, "two-codes.png"), "CODE-LEFT", "CODE-RIGHT")
	writeMultiQR(t, filepath.Join(root, "twins.png"), "TWIN", "TWIN")

	// Images with no QR: these must be commented out, not dropped.
	writeBlank(t, filepath.Join(root, "blank.png"))
	writeBlank(t, filepath.Join(root, "a", "blank2.png"))

	// Non-image files must be skipped silently.
	for _, junk := range []string{"notes.txt", "data.npy", "README", "a/thing.json"} {
		p := filepath.Join(root, filepath.FromSlash(junk))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("not an image"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	out := filepath.Join(t.TempDir(), "corpus.csv")
	if err := run(root, out, false, 4, ""); err != nil {
		t.Fatalf("run: %v", err)
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	text := string(raw)

	// The premise must be stated in the file itself.
	if !strings.Contains(text, "NOT GROUND TRUTH") {
		t.Error("manifest header does not state that it is not ground truth")
	}

	got := readManifest(t, out)

	// Decoded rows, including every tricky payload, survive the round trip.
	want := map[string][]string{
		"a/dup.png":              {"payload-from-a"},
		"b/dup.png":              {"payload-from-b"},
		"deep/deeper/nested.png": {"nested-payload"},
		"#hash.png":              {"hash-payload"},
		"two-codes.png":          {"CODE-LEFT", "CODE-RIGHT"},
		"twins.png":              {"TWIN", "TWIN"},
	}
	for name, payload := range trickyPayloads {
		want[name] = []string{payload}
	}
	for path, wantPayloads := range want {
		gotPayloads, ok := got[path]
		if !ok {
			t.Errorf("missing live row for %q", path)
			continue
		}
		if len(gotPayloads) != len(wantPayloads) {
			t.Errorf("%s decoded %d payloads (%q), want %d (%q)", path, len(gotPayloads), gotPayloads, len(wantPayloads), wantPayloads)
			continue
		}
		for i := range wantPayloads {
			if gotPayloads[i] != wantPayloads[i] {
				t.Errorf("%s payload %d = %q, want %q", path, i, gotPayloads[i], wantPayloads[i])
			}
		}
	}

	// Undecoded images are absent from the parsed rows (they are commented
	// out) but present in the file, pre-labelled and ready to uncomment.
	for _, blank := range []string{"blank.png", "a/blank2.png"} {
		if _, live := got[blank]; live {
			t.Errorf("%s should be commented out, not a live row", blank)
		}
		if !strings.Contains(text, "#"+blank+","+expectedFail) {
			t.Errorf("manifest has no commented row for %s", blank)
		}
	}

	// Non-images never appear, in any form.
	for _, junk := range []string{"notes.txt", "data.npy", "README", "a/thing.json"} {
		if strings.Contains(text, junk) {
			t.Errorf("non-image %s leaked into the manifest", junk)
		}
	}

	if len(got) != len(want) {
		t.Errorf("live rows = %d, want %d: %v", len(got), len(want), got)
	}
}

// The manifest must not depend on how the concurrent decode interleaved.
func TestGenerateIsDeterministic(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"c.png", "a/x.png", "b/y.png", "a/b/z.png"} {
		writeQR(t, filepath.Join(root, filepath.FromSlash(n)), "payload-"+n)
	}

	var first string
	for i, jobs := range []int{1, 3, 8} {
		out := filepath.Join(t.TempDir(), "corpus.csv")
		if err := run(root, out, false, jobs, ""); err != nil {
			t.Fatalf("run(jobs=%d): %v", jobs, err)
		}
		raw, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		// The header names the root, which differs per run only via t.TempDir
		// for the output, not the input, so the whole file should match.
		if i == 0 {
			first = string(raw)
			continue
		}
		if string(raw) != first {
			t.Errorf("manifest differs at jobs=%d", jobs)
		}
	}
}

func TestRefusesToOverwrite(t *testing.T) {
	root := t.TempDir()
	writeQR(t, filepath.Join(root, "q.png"), "x")

	out := filepath.Join(t.TempDir(), "corpus.csv")
	if err := os.WriteFile(out, []byte("hand-labelled, do not clobber\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := run(root, out, false, 1, "")
	if err == nil {
		t.Fatal("run overwrote an existing manifest without -f")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %v, want it to mention the file exists", err)
	}
	raw, rerr := os.ReadFile(out)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if !strings.HasPrefix(string(raw), "hand-labelled") {
		t.Error("existing manifest was modified")
	}

	if err := run(root, out, true, 1, ""); err != nil {
		t.Fatalf("run with force: %v", err)
	}
	raw, rerr = os.ReadFile(out)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if strings.HasPrefix(string(raw), "hand-labelled") {
		t.Error("-f did not overwrite the manifest")
	}
}

func TestEmptyDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "corpus.csv")
	if err := run(root, out, false, 1, ""); err == nil {
		t.Fatal("expected an error when the directory holds no images")
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("an empty run should not create a manifest")
	}
}

// TestTruthSidecarsAreNotDecoded is the whole point of -truth: the payloads
// must come from the sidecar files and not from this decoder, so a manifest
// built this way measures accuracy rather than self-consistency.
//
// The image here is a QR whose real payload is "real", and its sidecar says
// "from-the-sidecar". If the manifest comes back saying "real", something
// decoded the image and the flag is not doing its job.
func TestTruthSidecarsAreNotDecoded(t *testing.T) {
	root := t.TempDir()
	writeQR(t, filepath.Join(root, "a.png"), "real")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("from-the-sidecar"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(root, "corpus.csv")
	if err := run(root, out, false, 1, ".txt"); err != nil {
		t.Fatalf("run: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, "from-the-sidecar") {
		t.Errorf("manifest does not carry the sidecar payload:\n%s", got)
	}
	if strings.Contains(got, ",real") {
		t.Errorf("manifest carries a decoded payload; -truth decoded the image:\n%s", got)
	}
	if !strings.Contains(got, "GROUND TRUTH") {
		t.Errorf("manifest still claims to be a bootstrap:\n%s", got)
	}
}

// TestTruthSkipsImagesWithNoSidecar: an image whose answer is unknown is left
// out, never labelled EXPECTED_FAIL. EXPECTED_FAIL is a claim that the image
// holds no QR, and a missing file makes no such claim.
func TestTruthSkipsImagesWithNoSidecar(t *testing.T) {
	root := t.TempDir()
	writeQR(t, filepath.Join(root, "known.png"), "known")
	writeQR(t, filepath.Join(root, "unknown.png"), "unknown")
	if err := os.WriteFile(filepath.Join(root, "known.txt"), []byte("answer"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(root, "corpus.csv")
	if err := run(root, out, false, 1, ".txt"); err != nil {
		t.Fatalf("run: %v", err)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	if !strings.Contains(got, "known.png") {
		t.Errorf("manifest lost the image that had a sidecar:\n%s", got)
	}
	if strings.Contains(got, "unknown.png") {
		t.Errorf("an image with no sidecar reached the manifest:\n%s", got)
	}
}

// TestTruthWithNoSidecarsAtAll fails loudly rather than writing an empty
// manifest, which would benchmark as a perfect score over nothing.
func TestTruthWithNoSidecarsAtAll(t *testing.T) {
	root := t.TempDir()
	writeQR(t, filepath.Join(root, "a.png"), "x")
	if err := run(root, filepath.Join(root, "corpus.csv"), false, 1, ".txt"); err == nil {
		t.Fatal("run with no sidecars anywhere succeeded; want an error")
	}
}
