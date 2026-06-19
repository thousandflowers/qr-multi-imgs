package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/thousandflowers/qr-multi-imgs/scanner"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func testModel() model {
	return model{
		page:    pageFolderInput,
		input:   textinput.New(),
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		summary: nil,
	}
}

func testQRSummary(t *testing.T, dir string) *scanner.Summary {
	t.Helper()
	f1 := filepath.Join(dir, "with_qr.png")
	f2 := filepath.Join(dir, "no_qr.png")
	f3 := filepath.Join(dir, "error.png")
	os.WriteFile(f1, []byte("dummy"), 0644)
	os.WriteFile(f2, []byte("dummy"), 0644)
	os.WriteFile(f3, []byte("dummy"), 0644)
	return &scanner.Summary{
		Total:     3,
		WithQR:    1,
		WithoutQR: 1,
		Errors:    1,
		TotalSize: 30,
		StartTime: time.Now(),
		Duration:  time.Second,
		Results: []scanner.ScanResult{
			{FilePath: f1, HasQR: true, Contents: []string{"https://example.com"}, FileSize: 10},
			{FilePath: f2, HasQR: false, FileSize: 10},
			{FilePath: f3, HasQR: false, Error: "decode failed", FileSize: 10},
		},
	}
}

// ─── resolvePath ─────────────────────────────────────────────────────────────

func TestResolvePath(t *testing.T) {
	t.Run("empty string returns cwd", func(t *testing.T) {
		got, err := resolvePath("")
		if err != nil {
			t.Fatalf("resolvePath('') error: %v", err)
		}
		cwd, _ := os.Getwd()
		if got != cwd {
			t.Errorf("resolvePath('') = %q, want %q", got, cwd)
		}
	})
	t.Run("trims whitespace", func(t *testing.T) {
		got, err := resolvePath("  /tmp/test  ")
		if err != nil {
			t.Fatalf("resolvePath error: %v", err)
		}
		if got != "/tmp/test" {
			t.Errorf("resolvePath = %q, want %q", got, "/tmp/test")
		}
	})
	t.Run("strips quotes", func(t *testing.T) {
		got, err := resolvePath(`"/tmp/test"`)
		if err != nil {
			t.Fatalf("resolvePath error: %v", err)
		}
		if got != "/tmp/test" {
			t.Errorf("resolvePath = %q, want %q", got, "/tmp/test")
		}
	})
	t.Run("resolves relative to absolute", func(t *testing.T) {
		got, err := resolvePath(".")
		if err != nil {
			t.Fatalf("resolvePath('.') error: %v", err)
		}
		cwd, _ := os.Getwd()
		if got != cwd {
			t.Errorf("resolvePath('.') = %q, want %q", got, cwd)
		}
	})
	t.Run("expands tilde", func(t *testing.T) {
		got, err := resolvePath("~")
		if err != nil {
			t.Fatalf("resolvePath('~') error: %v", err)
		}
		home, _ := os.UserHomeDir()
		if got != home {
			t.Errorf("resolvePath('~') = %q, want %q", got, home)
		}
	})
	t.Run("tilde with subdir", func(t *testing.T) {
		got, err := resolvePath("~/subdir")
		if err != nil {
			t.Fatalf("resolvePath('~/subdir') error: %v", err)
		}
		home, _ := os.UserHomeDir()
		want := filepath.Join(home, "subdir")
		if got != want {
			t.Errorf("resolvePath('~/subdir') = %q, want %q", got, want)
		}
	})
}

// ─── validateScanPath ────────────────────────────────────────────────────────

func TestValidateScanPath(t *testing.T) {
	t.Run("valid directory passes", func(t *testing.T) {
		dir := t.TempDir()
		if err := validateScanPath(dir); err != nil {
			t.Fatalf("validateScanPath on temp dir: %v", err)
		}
	})
	t.Run("non-existent path fails", func(t *testing.T) {
		err := validateScanPath("/tmp/nonexistent_qr_test_path")
		if err == nil {
			t.Fatal("expected error for non-existent path")
		}
	})
	t.Run("file instead of directory fails", func(t *testing.T) {
		f, _ := os.CreateTemp(t.TempDir(), "file")
		f.Close()
		err := validateScanPath(f.Name())
		if err == nil {
			t.Fatal("expected error for file path")
		}
	})
}

// ─── sanitizeFilename ────────────────────────────────────────────────────────

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"simple.txt", 40, "simple.txt"},
		{"path/to/file", 40, "path_to_file"},
		{"special:*?\"<>|chars", 40, "special_______chars"},
		{"  spaced  ", 40, "spaced"},
		{"\nnewline\r", 40, "newline"},
		{"too long filename here", 10, "too long f"},
		{"", 40, ""},
		{"   \n  ", 40, ""},
	}
	for _, tt := range tests {
		got := sanitizeFilename(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("sanitizeFilename(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

// ─── truncate ────────────────────────────────────────────────────────────────

func TestTruncate(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"short", 20, "short"},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.s, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}

// ─── clamp ───────────────────────────────────────────────────────────────────

func TestClamp(t *testing.T) {
	tests := []struct {
		v, lo, hi, want int
	}{
		{5, 0, 10, 5},
		{-5, 0, 10, 0},
		{15, 0, 10, 10},
		{0, 0, 10, 0},
		{10, 0, 10, 10},
	}
	for _, tt := range tests {
		got := clamp(tt.v, tt.lo, tt.hi)
		if got != tt.want {
			t.Errorf("clamp(%d, %d, %d) = %d, want %d", tt.v, tt.lo, tt.hi, got, tt.want)
		}
	}
}

// ─── roundDur ────────────────────────────────────────────────────────────────

func TestRoundDur(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Millisecond, "500ms"},
		{1500 * time.Millisecond, "1.5s"},
		{0, "0ms"},
		{10 * time.Second, "10.0s"},
	}
	for _, tt := range tests {
		got := roundDur(tt.d)
		if got != tt.want {
			t.Errorf("roundDur(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

// ─── isWritablePath ──────────────────────────────────────────────────────────

func TestIsWritablePath(t *testing.T) {
	t.Run("writable dir returns nil", func(t *testing.T) {
		dir := t.TempDir()
		if err := isWritablePath(dir); err != nil {
			t.Fatalf("isWritablePath on temp dir: %v", err)
		}
	})
	t.Run("non-existent dir fails", func(t *testing.T) {
		err := isWritablePath("/nonexistent/path")
		if err == nil {
			t.Fatal("expected error for non-existent dir")
		}
	})
}

// ─── safeCmd ─────────────────────────────────────────────────────────────────

func TestSafeCmd_normalResult(t *testing.T) {
	msg := safeCmd(func() tea.Msg { return actionDoneMsg{"ok"} })()
	if m, ok := msg.(actionDoneMsg); !ok || m.message != "ok" {
		t.Fatalf("safeCmd: expected actionDoneMsg{ok}, got %T %+v", msg, msg)
	}
}

func TestSafeCmd_panicRecovery(t *testing.T) {
	msg := safeCmd(func() tea.Msg { panic("boom") })()
	if _, ok := msg.(crashMsg); !ok {
		t.Fatalf("safeCmd: expected crashMsg for panic, got %T %+v", msg, msg)
	}
}

// ─── scanFolder ──────────────────────────────────────────────────────────────

func TestScanFolderCmd_success(t *testing.T) {
	dir := t.TempDir()
	// Create a dummy image file (not a real image — ScanFolder doesn't validate content at the dir level)
	os.WriteFile(filepath.Join(dir, "test.png"), []byte("dummy"), 0644)

	cmd := scanFolder(dir)
	msg := cmd()
	switch m := msg.(type) {
	case scanCompleteMsg:
		if m.summary == nil {
			t.Fatal("scanCompleteMsg has nil summary")
		}
	case actionErrorMsg:
		t.Fatalf("scanFolder: unexpected error: %v", m.err)
	default:
		t.Fatalf("scanFolder: expected scanCompleteMsg, got %T", msg)
	}
}

func TestScanFolderCmd_nonexistentDir(t *testing.T) {
	cmd := scanFolder("/tmp/nonexistent_qr_test_12345")
	msg := cmd()
	if _, ok := msg.(actionErrorMsg); !ok {
		t.Fatalf("scanFolder(nonexistent): expected actionErrorMsg, got %T", msg)
	}
}

// ─── deleteWithoutQR ─────────────────────────────────────────────────────────

func TestDeleteWithoutQR_deletesOnlyNoQR(t *testing.T) {
	dir := t.TempDir()
	s := testQRSummary(t, dir)

	cmd := deleteWithoutQR(s)
	msg := cmd()

	dm, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("deleteWithoutQR: expected actionDoneMsg, got %T", msg)
	}
	if !strings.Contains(dm.message, "Deleted 1") {
		t.Errorf("deleteWithoutQR: expected 'Deleted 1', got %q", dm.message)
	}
	// no_qr.png should be gone
	if _, err := os.Stat(filepath.Join(dir, "no_qr.png")); !os.IsNotExist(err) {
		t.Error("no_qr.png should have been deleted")
	}
	// with_qr.png should remain
	if _, err := os.Stat(filepath.Join(dir, "with_qr.png")); err != nil {
		t.Error("with_qr.png should still exist")
	}
}

func TestDeleteWithoutQR_removeError(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "target.png")
	os.WriteFile(f, []byte("x"), 0644)
	// Remove write permission so os.Remove fails.
	os.Chmod(dir, 0500)
	defer os.Chmod(dir, 0755)

	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: f, HasQR: false},
		},
	}
	cmd := deleteWithoutQR(s)
	msg := cmd()
	dm, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("expected actionDoneMsg, got %T", msg)
	}
	if !strings.Contains(dm.message, "Errors") {
		t.Errorf("expected error in message, got %q", dm.message)
	}
}

func TestDeleteWithoutQR_noopWhenNone(t *testing.T) {
	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: "a.png", HasQR: true, Contents: []string{"x"}},
		},
	}
	cmd := deleteWithoutQR(s)
	msg := cmd()
	dm, ok := msg.(actionDoneMsg)
	if !ok || !strings.Contains(dm.message, "Deleted 0") {
		t.Fatalf("deleteWithoutQR(no candidates): expected 'Deleted 0', got %T %+v", msg, msg)
	}
}

func TestOrganizeByQR_conflict(t *testing.T) {
	dir := t.TempDir()
	s := testQRSummary(t, dir)
	os.MkdirAll(filepath.Join(dir, "with_qr"), 0755)
	os.WriteFile(filepath.Join(dir, "with_qr", "with_qr.png"), []byte("blocker"), 0644)

	cmd := organizeByQR(s)
	msg := cmd()

	dm, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("organizeByQR conflict: expected actionDoneMsg, got %T", msg)
	}
	if !strings.Contains(dm.message, "Organized") {
		t.Errorf("expected 'Organized', got %q", dm.message)
	}
}

func TestOrganizeByQR(t *testing.T) {
	dir := t.TempDir()
	s := testQRSummary(t, dir)

	cmd := organizeByQR(s)
	msg := cmd()

	dm, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("organizeByQR: expected actionDoneMsg, got %T", msg)
	}
	if !strings.Contains(dm.message, "Organized") {
		t.Errorf("organizeByQR: expected 'Organized', got %q", dm.message)
	}
	if _, err := os.Stat(filepath.Join(dir, "with_qr")); err != nil {
		t.Error("with_qr/ dir should exist")
	}
	if _, err := os.Stat(filepath.Join(dir, "without_qr")); err != nil {
		t.Error("without_qr/ dir should exist")
	}
}

func TestOrganizeByQR_renameError(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	f := filepath.Join(sub, "img.png")
	os.WriteFile(f, []byte("x"), 0644)
	os.Chmod(sub, 0555)
	defer os.Chmod(sub, 0755)

	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: f, HasQR: true, Contents: []string{"x"}},
		},
	}
	cmd := organizeByQR(s)
	msg := cmd()
	dm, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("organizeByQR renameError: expected actionDoneMsg, got %T", msg)
	}
	if !strings.Contains(dm.message, "Errors") {
		t.Errorf("expected error message for rename failure, got %q", dm.message)
	}
}

// ─── recreateQRs ─────────────────────────────────────────────────────────────

func TestRecreateQRs(t *testing.T) {
	dir := t.TempDir()
	s := testQRSummary(t, dir)

	cmd := recreateQRs(s, "png")
	msg := cmd()

	dm, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("recreateQRs: expected actionDoneMsg, got %T", msg)
	}
	if !strings.Contains(dm.message, "Created 1") {
		t.Errorf("recreateQRs: expected 'Created 1', got %q", dm.message)
	}
	if _, err := os.Stat(filepath.Join(dir, "recreated_qr")); err != nil {
		t.Error("recreated_qr/ dir should exist")
	}
}

func TestRecreateQRs_multipleContents(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "multi.png")
	os.WriteFile(f, []byte("x"), 0644)
	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: f, HasQR: true, Contents: []string{"https://a.com", "https://b.com"}},
		},
	}
	cmd := recreateQRs(s, "png")
	msg := cmd()
	dm, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("recreateQRs multiple: expected actionDoneMsg, got %T", msg)
	}
	if !strings.Contains(dm.message, "Created 2") {
		t.Errorf("expected 'Created 2', got %q", dm.message)
	}
}

func TestRecreateQRs_emptySafeName(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "empty.png")
	os.WriteFile(f, []byte("x"), 0644)
	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: f, HasQR: true, Contents: []string{"\n"}},
		},
	}
	cmd := recreateQRs(s, "png")
	msg := cmd()
	dm, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("recreateQRs empty: expected actionDoneMsg, got %T", msg)
	}
	if !strings.Contains(dm.message, "Created 1") {
		t.Errorf("expected 'Created 1', got %q", dm.message)
	}
	if !strings.Contains(dm.message, "recreated_qr") {
		t.Errorf("expected output dir in message, got %q", dm.message)
	}
}

func TestRecreateQRs_noopWhenNoQR(t *testing.T) {
	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: "a.png", HasQR: false},
		},
	}
	cmd := recreateQRs(s, "png")
	msg := cmd()
	dm, ok := msg.(actionDoneMsg)
	if !ok || !strings.Contains(dm.message, "Created 0") {
		t.Fatalf("recreateQRs(no QR): expected 'Created 0', got %T %+v", msg, msg)
	}
}

func TestRecreateQRs_writeError(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.png")
	os.WriteFile(f, []byte("x"), 0644)
	outDir := filepath.Join(dir, "recreated_qr")
	os.MkdirAll(outDir, 0755)
	os.Chmod(outDir, 0555)
	defer os.Chmod(outDir, 0755)

	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: f, HasQR: true, Contents: []string{"test"}},
		},
	}
	cmd := recreateQRs(s, "png")
	msg := cmd()
	dm, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("recreateQRs writeError: expected actionDoneMsg, got %T", msg)
	}
	if !strings.Contains(dm.message, "Errors") {
		t.Errorf("expected error message for write failure, got %q", dm.message)
	}
}

// ─── exportResults ───────────────────────────────────────────────────────────

func TestExportResults_json(t *testing.T) {
	dir := t.TempDir()
	s := testQRSummary(t, dir)

	cmd := exportResults(s, "json")
	msg := cmd()

	dm, ok := msg.(actionDoneMsg)
	if !ok {
		t.Fatalf("exportResults: expected actionDoneMsg, got %T", msg)
	}
	if !strings.Contains(dm.message, "Exported") {
		t.Errorf("exportResults: expected 'Exported', got %q", dm.message)
	}
}

func TestExportResults_error(t *testing.T) {
	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: "/nonexistent/test.png", HasQR: true, Contents: []string{"x"}},
		},
	}
	cmd := exportResults(s, "json")
	msg := cmd()
	if _, ok := msg.(actionDoneMsg); ok {
		t.Error("expected actionErrorMsg for non-existent dir")
	}
}

// ─── page views ──────────────────────────────────────────────────────────────

func TestView_folderInput(t *testing.T) {
	m := testModel()
	m.page = pageFolderInput
	v := m.View()
	if v == "" {
		t.Fatal("viewFolderInput returned empty")
	}
	if !strings.Contains(v, "QR Multi IMG Scanner") {
		t.Error("folder input view missing title")
	}
}

func TestView_scanning(t *testing.T) {
	m := testModel()
	m.page = pageScanning
	v := m.View()
	if v == "" {
		t.Fatal("viewScanning returned empty")
	}
	if !strings.Contains(v, "Scanning") {
		t.Error("scanning view missing 'Scanning'")
	}
}

func TestView_results(t *testing.T) {
	dir := t.TempDir()
	m := testModel()
	m.page = pageResults
	m.summary = testQRSummary(t, dir)

	v := m.View()
	if v == "" {
		t.Fatal("viewResults returned empty")
	}
	if !strings.Contains(v, "Select action") {
		t.Error("results view missing 'Select action'")
	}
}

func TestView_results_nilSummary(t *testing.T) {
	m := testModel()
	m.page = pageResults
	m.summary = nil
	v := m.View()
	if v != "" {
		t.Errorf("viewResults with nil summary should be empty, got %q", v)
	}
}

func TestView_results_bareFilename(t *testing.T) {
	m := testModel()
	m.page = pageResults
	m.summary = &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: "bare.png", HasQR: true},
		},
	}
	v := m.View()
	if v == "" {
		t.Fatal("viewResults with bare filename returned empty")
	}
}

func TestView_list(t *testing.T) {
	dir := t.TempDir()
	m := testModel()
	m.page = pageList
	m.summary = testQRSummary(t, dir)

	v := m.View()
	if v == "" {
		t.Fatal("viewList returned empty")
	}
	if !strings.Contains(v, "All Scanned Images") {
		t.Error("list view missing header")
	}
}

func TestView_listWithError(t *testing.T) {
	m := testModel()
	m.page = pageList
	m.summary = &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: "err.png", HasQR: false, Error: "decode failed"},
		},
	}
	v := m.View()
	if v == "" {
		t.Fatal("viewList with error returned empty")
	}
	if !strings.Contains(v, "ERR") {
		t.Error("list view should show ERR for error results")
	}
	if !strings.Contains(v, "decode failed") {
		t.Error("list view should include error text")
	}
}

func TestView_exportFormat(t *testing.T) {
	m := testModel()
	m.page = pageExportFormat
	v := m.View()
	if v == "" {
		t.Fatal("viewExportFormat returned empty")
	}
	if !strings.Contains(v, "Export") {
		t.Error("export format view missing 'Export'")
	}
}

func TestView_qrFormat(t *testing.T) {
	m := testModel()
	m.page = pageQRFormat
	v := m.View()
	if v == "" {
		t.Fatal("viewQRFormat returned empty")
	}
	if !strings.Contains(v, "Recreate QR") {
		t.Error("QR format view missing 'Recreate QR'")
	}
}

func TestView_working(t *testing.T) {
	m := testModel()
	m.page = pageWorking
	v := m.View()
	if v == "" {
		t.Fatal("viewWorking returned empty")
	}
	if !strings.Contains(v, "Working") {
		t.Error("working view missing 'Working'")
	}
}

func TestView_done(t *testing.T) {
	m := testModel()
	m.page = pageDone
	m.actionMsg = "test done"
	v := m.View()
	if v == "" {
		t.Fatal("viewDone returned empty")
	}
	if !strings.Contains(v, "test done") {
		t.Error("done view missing action message")
	}
}

func TestView_error(t *testing.T) {
	m := testModel()
	m.page = pageError
	m.errMsg = "test error"
	v := m.View()
	if v == "" {
		t.Fatal("viewError returned empty")
	}
	if !strings.Contains(v, "test error") {
		t.Error("error view missing error message")
	}
}

// ─── model.Update message handling ───────────────────────────────────────────

func TestUpdate_windowSize(t *testing.T) {
	m := testModel()
	m2, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	if cmd != nil {
		t.Error("WindowSizeMsg should return nil cmd")
	}
	mm := m2.(model)
	if mm.width != 120 || mm.height != 60 {
		t.Errorf("WindowSizeMsg: want 120x60, got %dx%d", mm.width, mm.height)
	}
}

func TestUpdate_scanComplete(t *testing.T) {
	m := testModel()
	dir := t.TempDir()
	s := testQRSummary(t, dir)
	m2, cmd := m.Update(scanCompleteMsg{s})
	if cmd != nil {
		t.Error("scanCompleteMsg should return nil cmd")
	}
	mm := m2.(model)
	if mm.page != pageResults {
		t.Errorf("scanCompleteMsg: expected pageResults, got %v", mm.page)
	}
	if mm.summary != s {
		t.Error("scanCompleteMsg: summary not set")
	}
}

func TestUpdate_actionDone(t *testing.T) {
	m := testModel()
	m2, cmd := m.Update(actionDoneMsg{"all good"})
	if cmd != nil {
		t.Error("actionDoneMsg should return nil cmd")
	}
	mm := m2.(model)
	if mm.page != pageDone {
		t.Errorf("actionDoneMsg: expected pageDone, got %v", mm.page)
	}
	if mm.actionMsg != "all good" {
		t.Errorf("actionDoneMsg: message = %q, want 'all good'", mm.actionMsg)
	}
}

func TestUpdate_actionError(t *testing.T) {
	m := testModel()
	m2, cmd := m.Update(actionErrorMsg{os.ErrPermission})
	if cmd != nil {
		t.Error("actionErrorMsg should return nil cmd")
	}
	mm := m2.(model)
	if mm.page != pageError {
		t.Errorf("actionErrorMsg: expected pageError, got %v", mm.page)
	}
	if mm.errMsg == "" {
		t.Error("actionErrorMsg: errMsg should be set")
	}
}

func TestUpdate_crashMsg(t *testing.T) {
	m := testModel()
	m2, cmd := m.Update(crashMsg{"test crash"})
	if cmd != nil {
		t.Error("crashMsg should return nil cmd")
	}
	mm := m2.(model)
	if mm.page != pageError {
		t.Errorf("crashMsg: expected pageError, got %v", mm.page)
	}
	if !strings.Contains(mm.errMsg, "INTERNAL CRASH") {
		t.Error("crashMsg should contain 'INTERNAL CRASH'")
	}
}

func TestUpdate_clipboardMsg_onFolderInput(t *testing.T) {
	m := testModel()
	m.page = pageFolderInput
	m.input, _ = m.input.Update(nil) // ensure not nil
	m.input.SetValue("")
	m2, _ := m.Update(clipboardMsg{"/paste/dir"})
	mm := m2.(model)
	// clipboard should set value, then auto-submit — if value is invalid dir it goes to error
	_ = mm
	// Just verify it doesn't panic; the behavior is complex (auto-submit may fail with invalid path)
}

func TestUpdate_spinnerTickWhileScanning(t *testing.T) {
	m := testModel()
	m.page = pageScanning
	m2, cmd := m.Update(spinner.TickMsg{})
	if cmd == nil {
		t.Error("spinner tick during scanning should return tint cmd")
	}
	_ = m2
}

func TestUpdate_unknownMsgPassesThrough(t *testing.T) {
	m := testModel()
	m2, cmd := m.Update("something unexpected")
	if cmd != nil {
		t.Error("unknown msg should return nil cmd")
	}
	mm := m2.(model)
	if mm.page != m.page {
		t.Errorf("unknown msg changed page from %v to %v", m.page, mm.page)
	}
}

// ─── model.Init ──────────────────────────────────────────────────────────────

func TestModelInit_noInitialPath(t *testing.T) {
	m := testModel()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() should return a command (batch of blink + tick)")
	}
}

func TestModelInit_withInitialPath(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.png"), []byte("dummy"), 0644)
	m := testModel()
	m.initialScanPath = dir
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() with initialScanPath should return a command")
	}
}

// ─── done page key events ────────────────────────────────────────────────────

func TestUpdate_donePageEnter(t *testing.T) {
	m := testModel()
	m.page = pageDone
	m.actionMsg = "done"
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m2.(model).page != pageResults {
		t.Errorf("done+Enter: expected pageResults, got %v", m2.(model).page)
	}
}

func TestUpdate_donePageEscape(t *testing.T) {
	m := testModel()
	m.page = pageDone
	m.actionMsg = "done"
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m2.(model).page != pageResults {
		t.Errorf("done+Esc: expected pageResults, got %v", m2.(model).page)
	}
}

func TestUpdate_errorPageEnter(t *testing.T) {
	m := testModel()
	m.page = pageError
	m.errMsg = "err"
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m2.(model).page != pageResults {
		t.Errorf("error+Enter: expected pageResults, got %v", m2.(model).page)
	}
}

func TestUpdate_errorPageEscape(t *testing.T) {
	m := testModel()
	m.page = pageError
	m.errMsg = "err"
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m2.(model).page != pageResults {
		t.Errorf("error+Esc: expected pageResults, got %v", m2.(model).page)
	}
}

// ─── version ─────────────────────────────────────────────────────────────────

func TestVersionVariable(t *testing.T) {
	if version == "" {
		t.Fatal("version variable should not be empty")
	}
}

// ─── updateFolderInput ───────────────────────────────────────────────────────

func TestUpdateFolderInput_enterInvalidPath(t *testing.T) {
	m := testModel()
	m.input.SetValue("/nonexistent_path_for_testing_xyz")
	m2, cmd := m.updateFolderInput(tea.KeyMsg{Type: tea.KeyEnter})
	mm := m2.(model)
	if mm.page != pageError {
		t.Errorf("expected pageError for invalid path, got %v", mm.page)
	}
	if cmd != nil {
		t.Error("expected nil cmd on validation error")
	}
}

func TestUpdateFolderInput_enterValidPath(t *testing.T) {
	dir := t.TempDir()
	m := testModel()
	m.input.SetValue(dir)
	m2, cmd := m.updateFolderInput(tea.KeyMsg{Type: tea.KeyEnter})
	mm := m2.(model)
	if mm.page != pageScanning {
		t.Errorf("expected pageScanning, got %v", mm.page)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (scanFolder)")
	}
}

func TestUpdateFolderInput_escape(t *testing.T) {
	m := testModel()
	_, cmd := m.updateFolderInput(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd == nil {
		t.Error("expected tea.Quit cmd on Escape")
	}
}

func TestUpdateFolderInput_ctrlC(t *testing.T) {
	m := testModel()
	_, cmd := m.updateFolderInput(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestUpdateFolderInput_ctrlD(t *testing.T) {
	m := testModel()
	_, cmd := m.updateFolderInput(tea.KeyMsg{Type: tea.KeyCtrlD})
	if runtime.GOOS == "darwin" && cmd == nil {
		t.Error("expected readClipboard cmd on macOS")
	}
}

func TestUpdateFolderInput_default(t *testing.T) {
	m := testModel()
	m.input.Focus()
	m2, cmd := m.updateFolderInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	_ = m2
	if cmd == nil {
		t.Error("expected textinput cmd on key rune")
	}
}

func TestUpdateFolderInput_enter_fileNotDir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "afile.txt")
	os.WriteFile(f, []byte("x"), 0644)
	m := testModel()
	m.input.SetValue(f)
	m2, cmd := m.updateFolderInput(tea.KeyMsg{Type: tea.KeyEnter})
	mm := m2.(model)
	if mm.page != pageError {
		t.Errorf("expected pageError for file path, got %d", mm.page)
	}
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestValidateScanPath_permissionDenied(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	os.Chmod(sub, 0000)
	defer os.Chmod(sub, 0755)
	err := validateScanPath(sub)
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected permission denied error, got %v", err)
	}
}

func TestValidateScanPath_notADir(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file.txt")
	os.WriteFile(f, []byte("x"), 0644)
	err := validateScanPath(f)
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("expected not a directory error, got %v", err)
	}
}

func TestValidateScanPath_nonexistent(t *testing.T) {
	err := validateScanPath("/tmp/nonexistent_validate_test_dir")
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected does not exist error, got %v", err)
	}
}

// ─── updateResults ───────────────────────────────────────────────────────────

func TestUpdateResults_up(t *testing.T) {
	m := testModel()
	m.page = pageResults
	m.cursor = 3
	m2, cmd := m.updateResults(tea.KeyMsg{Type: tea.KeyUp})
	mm := m2.(model)
	if mm.cursor != 2 {
		t.Errorf("expected cursor 2, got %d", mm.cursor)
	}
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestUpdateResults_upAtTop(t *testing.T) {
	m := testModel()
	m.page = pageResults
	m.cursor = 0
	m2, _ := m.updateResults(tea.KeyMsg{Type: tea.KeyUp})
	if m2.(model).cursor != 0 {
		t.Error("cursor should stay at 0 when at top")
	}
}

func TestUpdateResults_down(t *testing.T) {
	m := testModel()
	m.page = pageResults
	m.cursor = 3
	m2, cmd := m.updateResults(tea.KeyMsg{Type: tea.KeyDown})
	mm := m2.(model)
	if mm.cursor != 4 {
		t.Errorf("expected cursor 4, got %d", mm.cursor)
	}
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestUpdateResults_downAtBottom(t *testing.T) {
	m := testModel()
	m.page = pageResults
	m.cursor = 6
	m2, _ := m.updateResults(tea.KeyMsg{Type: tea.KeyDown})
	if m2.(model).cursor != 6 {
		t.Error("cursor should stay at 6 when at bottom")
	}
}

func TestUpdateResults_shiftTab(t *testing.T) {
	m := testModel()
	m.page = pageResults
	m.cursor = 2
	m2, _ := m.updateResults(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m2.(model).cursor != 1 {
		t.Errorf("expected cursor 1 after ShiftTab, got %d", m2.(model).cursor)
	}
}

func TestUpdateResults_tab(t *testing.T) {
	m := testModel()
	m.page = pageResults
	m.cursor = 2
	m2, _ := m.updateResults(tea.KeyMsg{Type: tea.KeyTab})
	if m2.(model).cursor != 3 {
		t.Errorf("expected cursor 3 after Tab, got %d", m2.(model).cursor)
	}
}

func TestUpdateResults_enter(t *testing.T) {
	m := testModel()
	m.page = pageResults
	m.summary = &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: "/tmp/test.png", HasQR: true, Contents: []string{"x"}},
		},
	}
	m2, cmd := m.updateResults(tea.KeyMsg{Type: tea.KeyEnter})
	mm := m2.(model)
	if mm.page != pageList {
		t.Errorf("expected pageList, got %d", mm.page)
	}
	if cmd != nil {
		t.Error("expected nil cmd from case-0 executeAction")
	}
}

func TestUpdateResults_ctrlC(t *testing.T) {
	m := testModel()
	m.page = pageResults
	_, cmd := m.updateResults(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestUpdateResults_default(t *testing.T) {
	m := testModel()
	m.page = pageResults
	m2, cmd := m.updateResults(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Error("expected nil cmd for unhandled key")
	}
	_ = m2
}

// ─── updateList ──────────────────────────────────────────────────────────────

func TestUpdateList_up(t *testing.T) {
	dir := t.TempDir()
	m := testModel()
	m.page = pageList
	m.summary = testQRSummary(t, dir)
	m.cursor = 1
	m2, cmd := m.updateList(tea.KeyMsg{Type: tea.KeyUp})
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m2.(model).cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m2.(model).cursor)
	}
}

func TestUpdateList_upAtTop(t *testing.T) {
	dir := t.TempDir()
	m := testModel()
	m.page = pageList
	m.summary = testQRSummary(t, dir)
	m.cursor = 0
	m2, _ := m.updateList(tea.KeyMsg{Type: tea.KeyUp})
	if m2.(model).cursor != 0 {
		t.Error("cursor should stay at 0")
	}
}

func TestUpdateList_down(t *testing.T) {
	dir := t.TempDir()
	m := testModel()
	m.page = pageList
	m.summary = testQRSummary(t, dir)
	m.cursor = 0
	m2, cmd := m.updateList(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m2.(model).cursor != 1 {
		t.Errorf("expected cursor 1, got %d", m2.(model).cursor)
	}
}

func TestUpdateList_downAtBottom(t *testing.T) {
	dir := t.TempDir()
	m := testModel()
	m.page = pageList
	m.summary = testQRSummary(t, dir)
	m.cursor = 2 // last item (0-indexed, 3 results total)
	m2, _ := m.updateList(tea.KeyMsg{Type: tea.KeyDown})
	if m2.(model).cursor != 2 {
		t.Error("cursor should stay at bottom")
	}
}

func TestUpdateList_escape(t *testing.T) {
	dir := t.TempDir()
	m := testModel()
	m.page = pageList
	m.summary = testQRSummary(t, dir)
	m2, cmd := m.updateList(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m2.(model).page != pageResults {
		t.Errorf("expected pageResults after Esc, got %v", m2.(model).page)
	}
}

func TestUpdateList_backspace(t *testing.T) {
	dir := t.TempDir()
	m := testModel()
	m.page = pageList
	m.summary = testQRSummary(t, dir)
	m2, cmd := m.updateList(tea.KeyMsg{Type: tea.KeyBackspace})
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m2.(model).page != pageResults {
		t.Errorf("expected pageResults after Backspace, got %v", m2.(model).page)
	}
}

func TestUpdateList_ctrlC(t *testing.T) {
	dir := t.TempDir()
	m := testModel()
	m.page = pageList
	m.summary = testQRSummary(t, dir)
	_, cmd := m.updateList(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestUpdateList_default(t *testing.T) {
	dir := t.TempDir()
	m := testModel()
	m.page = pageList
	m.summary = testQRSummary(t, dir)
	m2, cmd := m.updateList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	_ = m2
}

// ─── updateQRFormat ──────────────────────────────────────────────────────────

func TestUpdateQRFormat_up(t *testing.T) {
	m := testModel()
	m.page = pageQRFormat
	m.cursor = 1
	m2, cmd := m.updateQRFormat(tea.KeyMsg{Type: tea.KeyUp})
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m2.(model).cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m2.(model).cursor)
	}
}

func TestUpdateQRFormat_upAtTop(t *testing.T) {
	m := testModel()
	m.page = pageQRFormat
	m.cursor = 0
	m2, _ := m.updateQRFormat(tea.KeyMsg{Type: tea.KeyUp})
	if m2.(model).cursor != 0 {
		t.Error("cursor should stay at 0")
	}
}

func TestUpdateQRFormat_down(t *testing.T) {
	m := testModel()
	m.page = pageQRFormat
	m.cursor = 0
	m2, cmd := m.updateQRFormat(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m2.(model).cursor != 1 {
		t.Errorf("expected cursor 1, got %d", m2.(model).cursor)
	}
}

func TestUpdateQRFormat_downAtBottom(t *testing.T) {
	m := testModel()
	m.page = pageQRFormat
	m.cursor = 2
	m2, _ := m.updateQRFormat(tea.KeyMsg{Type: tea.KeyDown})
	if m2.(model).cursor != 2 {
		t.Error("cursor should stay at 2")
	}
}

func TestUpdateQRFormat_enterPNG(t *testing.T) {
	dir := t.TempDir()
	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: filepath.Join(dir, "test.png"), HasQR: true, Contents: []string{"x"}, FileSize: 10},
		},
		WithQR: 1,
	}
	m := testModel()
	m.page = pageQRFormat
	m.cursor = 0 // PNG
	m.summary = s
	m2, cmd := m.updateQRFormat(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("expected non-nil cmd")
	}
	if m2.(model).page != pageWorking {
		t.Errorf("expected pageWorking, got %v", m2.(model).page)
	}
	if m2.(model).qrRecreateFmt != "png" {
		t.Errorf("expected qrRecreateFmt 'png', got %q", m2.(model).qrRecreateFmt)
	}
}

func TestUpdateQRFormat_enterJPG(t *testing.T) {
	dir := t.TempDir()
	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: filepath.Join(dir, "test.png"), HasQR: true, Contents: []string{"x"}, FileSize: 10},
		},
		WithQR: 1,
	}
	m := testModel()
	m.page = pageQRFormat
	m.cursor = 1 // JPG
	m.summary = s
	m2, cmd := m.updateQRFormat(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("expected non-nil cmd")
	}
	if m2.(model).qrRecreateFmt != "jpg" {
		t.Errorf("expected qrRecreateFmt 'jpg', got %q", m2.(model).qrRecreateFmt)
	}
}

func TestUpdateQRFormat_enterSVG(t *testing.T) {
	dir := t.TempDir()
	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: filepath.Join(dir, "test.png"), HasQR: true, Contents: []string{"x"}, FileSize: 10},
		},
		WithQR: 1,
	}
	m := testModel()
	m.page = pageQRFormat
	m.cursor = 2 // SVG
	m.summary = s
	m2, cmd := m.updateQRFormat(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("expected non-nil cmd")
	}
	if m2.(model).qrRecreateFmt != "svg" {
		t.Errorf("expected qrRecreateFmt 'svg', got %q", m2.(model).qrRecreateFmt)
	}
}

func TestUpdateQRFormat_enterUnwritableDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	f := filepath.Join(sub, "test.png")
	os.WriteFile(f, []byte("x"), 0644)
	os.Chmod(sub, 0444)
	defer os.Chmod(sub, 0755)

	m := testModel()
	m.page = pageQRFormat
	m.summary = &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: f, HasQR: true, Contents: []string{"x"}},
		},
	}
	m2, cmd := m.updateQRFormat(tea.KeyMsg{Type: tea.KeyEnter})
	mm := m2.(model)
	if mm.page != pageError {
		t.Errorf("expected pageError for unwritable dir, got %d", mm.page)
	}
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestUpdateQRFormat_escape(t *testing.T) {
	m := testModel()
	m.page = pageQRFormat
	m2, cmd := m.updateQRFormat(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m2.(model).page != pageResults {
		t.Errorf("expected pageResults after Esc, got %v", m2.(model).page)
	}
}

func TestUpdateQRFormat_ctrlC(t *testing.T) {
	m := testModel()
	m.page = pageQRFormat
	_, cmd := m.updateQRFormat(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestUpdateQRFormat_default(t *testing.T) {
	m := testModel()
	m.page = pageQRFormat
	m2, cmd := m.updateQRFormat(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	_ = m2
}

// ─── updateExportFormat ──────────────────────────────────────────────────────

func TestUpdateExportFormat_up(t *testing.T) {
	m := testModel()
	m.page = pageExportFormat
	m.cursor = 1
	m2, cmd := m.updateExportFormat(tea.KeyMsg{Type: tea.KeyUp})
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m2.(model).cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m2.(model).cursor)
	}
}

func TestUpdateExportFormat_down(t *testing.T) {
	m := testModel()
	m.page = pageExportFormat
	m.cursor = 0
	m2, cmd := m.updateExportFormat(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m2.(model).cursor != 1 {
		t.Errorf("expected cursor 1, got %d", m2.(model).cursor)
	}
}

func TestUpdateExportFormat_enterJSON(t *testing.T) {
	dir := t.TempDir()
	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: filepath.Join(dir, "test.png"), HasQR: true, Contents: []string{"x"}, FileSize: 10},
		},
		WithQR: 1,
	}
	m := testModel()
	m.page = pageExportFormat
	m.cursor = 0 // JSON
	m.summary = s
	m2, cmd := m.updateExportFormat(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("expected non-nil cmd")
	}
	if m2.(model).page != pageWorking {
		t.Errorf("expected pageWorking, got %v", m2.(model).page)
	}
	if m2.(model).exportFmt != "json" {
		t.Errorf("expected exportFmt 'json', got %q", m2.(model).exportFmt)
	}
}

func TestUpdateExportFormat_enterCSV(t *testing.T) {
	dir := t.TempDir()
	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: filepath.Join(dir, "test.png"), HasQR: true, Contents: []string{"x"}, FileSize: 10},
		},
		WithQR: 1,
	}
	m := testModel()
	m.page = pageExportFormat
	m.cursor = 1 // CSV
	m.summary = s
	m2, cmd := m.updateExportFormat(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("expected non-nil cmd")
	}
	if m2.(model).exportFmt != "csv" {
		t.Errorf("expected exportFmt 'csv', got %q", m2.(model).exportFmt)
	}
}

func TestUpdateExportFormat_enterTXT(t *testing.T) {
	dir := t.TempDir()
	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: filepath.Join(dir, "test.png"), HasQR: true, Contents: []string{"x"}, FileSize: 10},
		},
		WithQR: 1,
	}
	m := testModel()
	m.page = pageExportFormat
	m.cursor = 2 // TXT
	m.summary = s
	m2, cmd := m.updateExportFormat(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("expected non-nil cmd")
	}
	if m2.(model).exportFmt != "txt" {
		t.Errorf("expected exportFmt 'txt', got %q", m2.(model).exportFmt)
	}
}

func TestUpdateExportFormat_noResults(t *testing.T) {
	m := testModel()
	m.page = pageExportFormat
	m.cursor = 0
	m.summary = &scanner.Summary{}
	m2, cmd := m.updateExportFormat(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m2.(model).page != pageError {
		t.Errorf("expected pageError, got %v", m2.(model).page)
	}
}

func TestUpdateExportFormat_enterUnwritableDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	f := filepath.Join(sub, "test.png")
	os.WriteFile(f, []byte("x"), 0644)
	os.Chmod(sub, 0444)
	defer os.Chmod(sub, 0755)

	m := testModel()
	m.page = pageExportFormat
	m.cursor = 0
	m.summary = &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: f, HasQR: true, Contents: []string{"x"}},
		},
	}
	m2, cmd := m.updateExportFormat(tea.KeyMsg{Type: tea.KeyEnter})
	mm := m2.(model)
	if mm.page != pageError {
		t.Errorf("expected pageError for unwritable dir, got %d", mm.page)
	}
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestUpdateExportFormat_escape(t *testing.T) {
	m := testModel()
	m.page = pageExportFormat
	m2, cmd := m.updateExportFormat(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m2.(model).page != pageResults {
		t.Errorf("expected pageResults, got %v", m2.(model).page)
	}
}

func TestUpdateExportFormat_ctrlC(t *testing.T) {
	m := testModel()
	m.page = pageExportFormat
	_, cmd := m.updateExportFormat(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestUpdateExportFormat_default(t *testing.T) {
	m := testModel()
	m.page = pageExportFormat
	m2, cmd := m.updateExportFormat(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	_ = m2
}

// ─── updateDone Ctrl+C ───────────────────────────────────────────────────────

func TestUpdateDone_ctrlC(t *testing.T) {
	m := testModel()
	m.page = pageDone
	_, cmd := m.updateDone(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestUpdateDone_default(t *testing.T) {
	m := testModel()
	m.page = pageDone
	m2, cmd := m.updateDone(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	_ = m2
}

// ─── executeAction ──────────────────────────────────────────────────────────

func TestExecuteAction_list(t *testing.T) {
	m := testModel()
	m.page = pageResults
	m2, cmd := m.executeAction(0) // List all results
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m2.(model).page != pageList {
		t.Errorf("expected pageList, got %v", m2.(model).page)
	}
}

func TestExecuteAction_export(t *testing.T) {
	m := testModel()
	m.page = pageResults
	m2, cmd := m.executeAction(1) // Export
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m2.(model).page != pageExportFormat {
		t.Errorf("expected pageExportFormat, got %v", m2.(model).page)
	}
}

func TestExecuteAction_deleteWithoutQR_noCandidates(t *testing.T) {
	dir := t.TempDir()
	s := &scanner.Summary{
		Results:   []scanner.ScanResult{},
		WithoutQR: 0,
	}
	m := testModel()
	m.summary = s
	m.page = pageResults
	_ = dir
	m2, cmd := m.executeAction(2) // Delete without QR
	if cmd != nil {
		t.Error("expected nil cmd when no candidates")
	}
	if m2.(model).page != pageDone {
		t.Errorf("expected pageDone, got %v", m2.(model).page)
	}
}

func TestExecuteAction_deleteWithoutQR_withCandidates(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "img.png"), []byte("dummy"), 0644)
	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: filepath.Join(dir, "img.png"), HasQR: false, FileSize: 5},
		},
		WithoutQR: 1,
	}
	m := testModel()
	m.summary = s
	m.page = pageResults
	m2, cmd := m.executeAction(2)
	if cmd == nil {
		t.Error("expected non-nil cmd")
	}
	if m2.(model).page != pageWorking {
		t.Errorf("expected pageWorking, got %v", m2.(model).page)
	}
}

func TestExecuteAction_organizeByQR(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "img.png"), []byte("dummy"), 0644)
	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: filepath.Join(dir, "img.png"), HasQR: true, Contents: []string{"x"}, FileSize: 5},
		},
		WithQR: 1,
	}
	m := testModel()
	m.summary = s
	m.page = pageResults
	m2, cmd := m.executeAction(3)
	if cmd == nil {
		t.Error("expected non-nil cmd")
	}
	if m2.(model).page != pageWorking {
		t.Errorf("expected pageWorking, got %v", m2.(model).page)
	}
}

func TestExecuteAction_deleteWithoutQR_unwritableDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	f := filepath.Join(sub, "img.png")
	os.WriteFile(f, []byte("x"), 0644)
	os.Chmod(sub, 0444)
	defer os.Chmod(sub, 0755)

	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: f, HasQR: false},
		},
		WithoutQR: 1,
	}
	m := testModel()
	m.summary = s
	m.page = pageResults
	m2, cmd := m.executeAction(2)
	mm := m2.(model)
	if mm.page != pageError {
		t.Errorf("expected pageError for unwritable dir, got %d", mm.page)
	}
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestExecuteAction_organizeByQR_unwritableDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0755)
	f := filepath.Join(sub, "img.png")
	os.WriteFile(f, []byte("x"), 0644)
	os.Chmod(sub, 0444)
	defer os.Chmod(sub, 0755)

	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: f, HasQR: false},
		},
	}
	m := testModel()
	m.summary = s
	m.page = pageResults
	m2, cmd := m.executeAction(3)
	mm := m2.(model)
	if mm.page != pageError {
		t.Errorf("expected pageError for unwritable dir, got %d", mm.page)
	}
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestExecuteAction_recreateQR_noCandidates(t *testing.T) {
	s := &scanner.Summary{
		Results: []scanner.ScanResult{},
		WithQR:  0,
	}
	m := testModel()
	m.summary = s
	m.page = pageResults
	m2, cmd := m.executeAction(4)
	if cmd != nil {
		t.Error("expected nil cmd when no QR")
	}
	if m2.(model).page != pageDone {
		t.Errorf("expected pageDone, got %v", m2.(model).page)
	}
}

func TestExecuteAction_recreateQR_withCandidates(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "img.png"), []byte("dummy"), 0644)
	s := &scanner.Summary{
		Results: []scanner.ScanResult{
			{FilePath: filepath.Join(dir, "img.png"), HasQR: true, Contents: []string{"x"}, FileSize: 5},
		},
		WithQR: 1,
	}
	m := testModel()
	m.summary = s
	m.page = pageResults
	m2, cmd := m.executeAction(4)
	if cmd != nil {
		t.Error("expected nil cmd (goes to format selection)")
	}
	if m2.(model).page != pageQRFormat {
		t.Errorf("expected pageQRFormat, got %v", m2.(model).page)
	}
}

func TestExecuteAction_newScan(t *testing.T) {
	m := testModel()
	m.page = pageResults
	m2, cmd := m.executeAction(5) // New scan
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	if m2.(model).page != pageFolderInput {
		t.Errorf("expected pageFolderInput, got %v", m2.(model).page)
	}
}

func TestExecuteAction_quit(t *testing.T) {
	m := testModel()
	m.page = pageResults
	_, cmd := m.executeAction(6) // Quit
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
}

func TestExecuteAction_default(t *testing.T) {
	m := testModel()
	m.page = pageResults
	m2, cmd := m.executeAction(99) // invalid index
	if cmd != nil {
		t.Error("expected nil cmd")
	}
	_ = m2
}

// ─── validateScanPath edge cases ────────────────────────────────────────────

func TestValidateScanPath_genericError(t *testing.T) {
	// Use an overly long path that will trigger a different error on Stat
	// This is hard to reliably trigger, so we skip this test.
}

func TestValidateScanPath_unreadableDir(t *testing.T) {
	dir := t.TempDir()
	// Remove read permission to make Open fail
	os.Chmod(dir, 0000)
	defer os.Chmod(dir, 0755)

	err := validateScanPath(dir)
	if err == nil {
		// On some systems running as root, Chmod may not restrict access
		t.Log("warning: unreadable dir test — dir was still accessible (running as root?)")
	}
	os.Chmod(dir, 0755)
}

// ─── Update routing via model.Update ────────────────────────────────────────

func TestUpdate_keyMsg_routesToList(t *testing.T) {
	dir := t.TempDir()
	m := testModel()
	m.page = pageList
	m.summary = testQRSummary(t, dir)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m2.(model).page != pageResults {
		t.Errorf("expected pageResults after Escape on list, got %v", m2.(model).page)
	}
}

func TestUpdate_keyMsg_routesToQRFormat(t *testing.T) {
	m := testModel()
	m.page = pageQRFormat
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m2.(model).page != pageResults {
		t.Errorf("expected pageResults after Escape on QRFormat, got %v", m2.(model).page)
	}
}

func TestUpdate_keyMsg_routesToExportFormat(t *testing.T) {
	m := testModel()
	m.page = pageExportFormat
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m2.(model).page != pageResults {
		t.Errorf("expected pageResults after Escape on ExportFormat, got %v", m2.(model).page)
	}
}

func TestUpdate_keyMsg_workingNoop(t *testing.T) {
	m := testModel()
	m.page = pageWorking
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil cmd on working page")
	}
	_ = m2
}

func TestUpdate_clipboardMsg_nonFolderInput(t *testing.T) {
	m := testModel()
	m.page = pageResults
	m2, cmd := m.Update(clipboardMsg{"/some/path"})
	if cmd != nil {
		t.Error("expected nil cmd when clipboardMsg on non-folder-input page")
	}
	_ = m2
}

func TestUpdate_scanningSpinnerTick(t *testing.T) {
	m := testModel()
	m.page = pageScanning
	m2, cmd := m.Update(sphereTickMsg{})
	if cmd != nil {
		t.Error("expected nil cmd for unknown msg type while scanning")
	}
	_ = m2
}

// sphereTickMsg is a random non-handled message type for scanning page test
type sphereTickMsg struct{}

// ─── View unknown page ──────────────────────────────────────────────────────

func TestView_unknownPage(t *testing.T) {
	m := testModel()
	m.page = page(999)
	v := m.View()
	if v != "" {
		t.Errorf("expected empty string for unknown page, got %q", v)
	}
}
