package scanner

import (
	"encoding/binary"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

// writeNPY creates a v1.0 .npy file with the given header dict and data bytes.
func writeNPY(t *testing.T, path, hdr string, data []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	f.Write([]byte("\x93NUMPY"))
	f.Write([]byte{1, 0}) // version 1.0
	binary.Write(f, binary.LittleEndian, uint16(len(hdr)))
	f.WriteString(hdr)
	f.Write(data)
}

func TestReadBoolNPY_simple2x3(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.npy")
	data := []byte{1, 0, 1, 0, 1, 0}
	hdr := `{'descr': '|b1', 'fortran_order': False, 'shape': (2, 3)}`
	writeNPY(t, path, hdr, data)

	got, err := readBoolNPY(path)
	if err != nil {
		t.Fatalf("readBoolNPY: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
	want := [][]bool{{true, false, true}, {false, true, false}}
	for i := range want {
		for j := range want[i] {
			if got[i][j] != want[i][j] {
				t.Errorf("got[%d][%d] = %v, want %v", i, j, got[i][j], want[i][j])
			}
		}
	}
}

func TestReadBoolNPY_trailingComma(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "comma.npy")
	data := []byte{1, 0, 1, 0, 1, 0}
	hdr := `{'descr': '|b1', 'fortran_order': False, 'shape': (2, 3),}`
	writeNPY(t, path, hdr, data)

	got, err := readBoolNPY(path)
	if err != nil {
		t.Fatalf("readBoolNPY with trailing comma: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
	if !got[0][0] {
		t.Error("expected got[0][0] = true")
	}
}

func TestReadBoolNPY_fortranOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f_order.npy")
	data := []byte{1, 0, 1, 0, 1, 0}
	hdr := `{'descr': '|b1', 'fortran_order': True, 'shape': (2, 3)}`
	writeNPY(t, path, hdr, data)

	got, err := readBoolNPY(path)
	if err != nil {
		t.Fatalf("readBoolNPY fortran: %v", err)
	}
	want := [][]bool{{true, true, true}, {false, false, false}}
	for i := range want {
		for j := range want[i] {
			if got[i][j] != want[i][j] {
				t.Errorf("F-order got[%d][%d] = %v, want %v", i, j, got[i][j], want[i][j])
			}
		}
	}
}

func TestReadBoolNPY_nonExistent(t *testing.T) {
	_, err := readBoolNPY("/tmp/nonexistent_npy_file_test")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestReadBoolNPY_truncated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trunc.npy")
	os.WriteFile(path, []byte("short"), 0644)
	_, err := readBoolNPY(path)
	if err == nil {
		t.Fatal("expected error for truncated file")
	}
}

func TestReadBoolNPY_badMagic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "badmagic.npy")
	os.WriteFile(path, []byte("NOTNUMPY\x01\x00\x0a\x00..."), 0644)
	_, err := readBoolNPY(path)
	if err == nil {
		t.Fatal("expected error for bad magic")
	}
}

func TestReadBoolNPY_unsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "v2.npy")
	f, _ := os.Create(path)
	f.Write([]byte("\x93NUMPY"))
	f.Write([]byte{2, 0}) // version 2, unsupported
	f.Write([]byte{0, 0}) // header len
	f.Close()
	_, err := readBoolNPY(path)
	if err == nil {
		t.Fatal("expected error for v2")
	}
}

func TestReadBoolNPY_headerTruncated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hdr_trunc.npy")
	f, _ := os.Create(path)
	f.Write([]byte("\x93NUMPY"))
	f.Write([]byte{1, 0})
	binary.Write(f, binary.LittleEndian, uint16(999)) // claims 999 bytes header
	f.WriteString("short")
	f.Close()
	_, err := readBoolNPY(path)
	if err == nil {
		t.Fatal("expected error for truncated header")
	}
}

func TestReadBoolNPY_badJSONHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "badjson.npy")
	hdr := `{'descr': '|b1', 'fortran_order': False, 'shape': (bad,)}`
	writeNPY(t, path, hdr, []byte{1, 0, 1})
	_, err := readBoolNPY(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON header")
	}
}

func TestReadBoolNPY_1Dshape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "1d.npy")
	data := []byte{1, 0, 1}
	hdr := `{'descr': '|b1', 'fortran_order': False, 'shape': (3,)}`
	writeNPY(t, path, hdr, data)
	_, err := readBoolNPY(path)
	if err == nil {
		t.Fatal("expected error for 1D array")
	}
}

// ─── pyDictToJSON ────────────────────────────────────────────────────────────

func TestPyDictToJSON_basic(t *testing.T) {
	input := `{'descr': '|b1', 'fortran_order': False, 'shape': (109, 109)}`
	got := pyDictToJSON(input)
	want := `{"descr": "|b1", "fortran_order": false, "shape": [109, 109]}`
	if got != want {
		t.Errorf("pyDictToJSON\n  got:  %s\n  want: %s", got, want)
	}
}

func TestPyDictToJSON_trailingCommaTrimmed(t *testing.T) {
	input := `{'descr': '|b1', 'fortran_order': False, 'shape': (2, 3),}`
	got := pyDictToJSON(input)
	want := `{"descr": "|b1", "fortran_order": false, "shape": [2, 3]}`
	if got != want {
		t.Errorf("pyDictToJSON trailing comma\n  got:  %s\n  want: %s", got, want)
	}
}

func TestPyDictToJSON_whitespaceTrimmed(t *testing.T) {
	input := `{'descr': '|b1', 'fortran_order': False, 'shape': (2, 3),}  `
	got := pyDictToJSON(input)
	if got[len(got)-1] != '}' {
		t.Errorf("expected trailing whitespace trimmed, got: %q", got)
	}
}

func TestPyDictToJSON_TrueLiteral(t *testing.T) {
	input := `{'descr': '|b1', 'fortran_order': True, 'shape': (5, 5),}`
	got := pyDictToJSON(input)
	if !stringsContains(got, "true") {
		t.Errorf("expected 'true' in output, got: %s", got)
	}
}

func TestPyDictToJSON_noComma(t *testing.T) {
	input := `{'descr': '|b1'}`
	got := pyDictToJSON(input)
	want := `{"descr": "|b1"}`
	if got != want {
		t.Errorf("pyDictToJSON no comma\n  got:  %s\n  want: %s", got, want)
	}
}

func TestPyDictToJSON_doubleQuotesInside(t *testing.T) {
	input := `{'descr': '"|b1"', 'fortran_order': False, 'shape': (1, 1)}`
	got := pyDictToJSON(input)
	if !stringsContains(got, `"descr"`) {
		t.Errorf("expected double-quoted keys, got: %s", got)
	}
}

// stringsContains because we're in package scanner, not scanner_test
func stringsContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ─── renderMask ──────────────────────────────────────────────────────────────

func TestRenderMask_basic(t *testing.T) {
	mask := [][]bool{
		{true, false},
		{false, true},
	}
	img := renderMask(mask, 2)
	if img == nil {
		t.Fatal("renderMask returned nil")
	}
	wantW := 2*2 + 2*4*2 // module_w*scale + 2*pad = 4 + 16 = 20
	wantH := 2*2 + 2*4*2
	b := img.Bounds()
	if b.Dx() != wantW || b.Dy() != wantH {
		t.Errorf("renderMask size = %dx%d, want %dx%d", b.Dx(), b.Dy(), wantW, wantH)
	}
	// Quiet zone (pad) should be white (255)
	if img.GrayAt(0, 0).Y != 255 {
		t.Error("quiet zone should be white")
	}
	// Center of first module (pad=8, scale=2 → x=8..10, y=8..10)
	if img.GrayAt(8+1, 8+1).Y != 0 {
		t.Error("true module should be black (0)")
	}
	// Area where false module is
	if img.GrayAt(8+2*2+1, 8+1).Y != 255 {
		t.Error("false module should be white (255)")
	}
}

func TestRenderMask_singlePixel(t *testing.T) {
	mask := [][]bool{{true}}
	img := renderMask(mask, 1)
	if img == nil {
		t.Fatal("renderMask returned nil")
	}
}

func TestRenderMask_colorModel(t *testing.T) {
	mask := [][]bool{{false}}
	img := renderMask(mask, 1)
	if img.ColorModel() != color.GrayModel {
		t.Error("renderMask should use Gray color model")
	}
}
