package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thousandflowers/qr-multi-imgs/exporter"
	"github.com/thousandflowers/qr-multi-imgs/scanner"
)

// The server is reachable by every page the browser has open, so these tests
// are the load-bearing ones: they describe what a hostile origin cannot do.

func testServer(t *testing.T) *apiServer {
	t.Helper()
	s, err := newAPIServer()
	if err != nil {
		t.Fatalf("newAPIServer: %v", err)
	}
	return s
}

func do(t *testing.T, s *apiServer, method, path, origin, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if token != "" {
		req.Header.Set(tokenHeader, token)
	}
	rec := httptest.NewRecorder()
	s.handler().ServeHTTP(rec, req)
	return rec
}

// jsonBody marshals a request body properly. Concatenating a path into JSON
// looks fine until a Windows temp dir arrives with backslashes in it.
func jsonBody(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	return string(b)
}

func TestAPI_rejectsMissingToken(t *testing.T) {
	s := testServer(t)
	rec := do(t, s, "POST", "/api/scan", allowedOrigin, "", `{"paths":["."]}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", rec.Code)
	}
}

func TestAPI_rejectsWrongToken(t *testing.T) {
	s := testServer(t)
	rec := do(t, s, "POST", "/api/scan", allowedOrigin, s.token+"x", `{"paths":["."]}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token = %d, want 401", rec.Code)
	}
}

func TestAPI_rejectsForeignOrigin(t *testing.T) {
	s := testServer(t)
	// Correct token, hostile page. Origin alone must stop it, because a token
	// can leak and an attacker page cannot forge Origin.
	rec := do(t, s, "POST", "/api/scan", "https://evil.example", s.token, `{"paths":["."]}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign origin = %d, want 403", rec.Code)
	}
}

func TestAPI_allowsLocalhostOrigins(t *testing.T) {
	s := testServer(t)
	for _, o := range []string{allowedOrigin, "http://127.0.0.1:8787", "http://localhost:8787"} {
		rec := do(t, s, "GET", "/api/health", o, s.token, "")
		if rec.Code != http.StatusOK {
			t.Errorf("origin %s = %d, want 200", o, rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != o {
			t.Errorf("origin %s echoed as %q", o, got)
		}
	}
}

func TestAPI_preflightAnswersPrivateNetworkAccess(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest("OPTIONS", "/api/scan", nil)
	req.Header.Set("Origin", allowedOrigin)
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Private-Network", "true")
	rec := httptest.NewRecorder()
	s.handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight = %d, want 204", rec.Code)
	}
	// Without this header Chrome refuses to let a public page reach 127.0.0.1.
	if rec.Header().Get("Access-Control-Allow-Private-Network") != "true" {
		t.Error("preflight missing Access-Control-Allow-Private-Network")
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), tokenHeader) {
		t.Errorf("preflight does not allow %s", tokenHeader)
	}
}

// The whole safety argument: a destructive call names a session, never a path.
func TestAPI_deleteRejectsUnknownSession(t *testing.T) {
	s := testServer(t)
	rec := do(t, s, "POST", "/api/delete", allowedOrigin, s.token, `{"session":"nope"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown session = %d, want 404", rec.Code)
	}
}

func TestAPI_deleteTouchesOnlyThatSessionsFiles(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "not_in_session.png")
	if err := os.WriteFile(victim, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	doomed := filepath.Join(dir, "no_qr.png")
	if err := os.WriteFile(doomed, []byte("no qr here"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := testServer(t)
	// A session that saw only `doomed`, as a real scan of it would produce.
	sess := s.putSession(&scanner.Summary{
		Total:     1,
		WithoutQR: 1,
		Results:   []scanner.ScanResult{{FilePath: doomed, HasQR: false}},
	})

	rec := do(t, s, "POST", "/api/delete", allowedOrigin, s.token, `{"session":"`+sess+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete = %d (%s), want 200", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(doomed); !os.IsNotExist(err) {
		t.Error("file without QR should have been deleted")
	}
	if _, err := os.Stat(victim); err != nil {
		t.Error("a file the session never saw must survive")
	}
}

func TestAPI_scanRejectsPathThatDoesNotExist(t *testing.T) {
	s := testServer(t)
	rec := do(t, s, "POST", "/api/scan", allowedOrigin, s.token,
		`{"paths":["/definitely/not/here/xyz"]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing path = %d, want 400", rec.Code)
	}
}

func TestAPI_healthReportsVersion(t *testing.T) {
	s := testServer(t)
	rec := do(t, s, "GET", "/api/health", allowedOrigin, s.token, "")
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("health body %q: %v", rec.Body.String(), err)
	}
	if got["version"] != versionString() {
		t.Errorf("health version = %v, want %s", got["version"], versionString())
	}
}

func TestNewAPIServer_tokenIsUnguessable(t *testing.T) {
	a, b := testServer(t), testServer(t)
	if a.token == b.token {
		t.Fatal("two servers produced the same token")
	}
	if len(a.token) < 32 {
		t.Errorf("token is %d chars, too short to resist guessing", len(a.token))
	}
}

// Binding beyond the loopback would expose the delete endpoint to the network.
func TestListenAddr_isLoopbackOnly(t *testing.T) {
	for _, in := range []string{"", ":8787", "8787", "127.0.0.1:9000"} {
		got, err := loopbackAddr(in)
		if err != nil {
			t.Fatalf("loopbackAddr(%q): %v", in, err)
		}
		if !strings.HasPrefix(got, "127.0.0.1:") {
			t.Errorf("loopbackAddr(%q) = %q, must bind 127.0.0.1", in, got)
		}
	}
	for _, bad := range []string{"0.0.0.0:8787", ":::8787", "192.168.1.5:8787"} {
		if _, err := loopbackAddr(bad); err == nil {
			t.Errorf("loopbackAddr(%q) should refuse a non-loopback bind", bad)
		}
	}
}

func TestServeAddr(t *testing.T) {
	cases := []struct {
		args []string
		want string
		on   bool
	}{
		{[]string{}, "", false},
		{[]string{"photos/"}, "", false},
		{[]string{"--serve"}, "", true},
		{[]string{"--serve=9000"}, "9000", true},
		{[]string{"--serve=127.0.0.1:9000"}, "127.0.0.1:9000", true},
		// A folder after a bare --serve is a scan target, never a bind address.
		{[]string{"--serve", "photos/"}, "", true},
	}
	for _, c := range cases {
		got, on := serveAddr(c.args)
		if got != c.want || on != c.on {
			t.Errorf("serveAddr(%v) = (%q, %v), want (%q, %v)", c.args, got, on, c.want, c.on)
		}
	}
}

func TestAPI_scanReturnsAUsableSession(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plain.txt"), []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := testServer(t)
	rec := do(t, s, "POST", "/api/scan", allowedOrigin, s.token,
		jsonBody(t, map[string]any{"paths": []string{dir}}))
	if rec.Code != http.StatusOK {
		t.Fatalf("scan = %d (%s)", rec.Code, rec.Body.String())
	}

	// The stream is newline-delimited JSON; the last line carries the session.
	lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
	var last struct {
		Session string `json:"session"`
		Summary *struct {
			Total int `json:"total"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatalf("last line %q: %v", lines[len(lines)-1], err)
	}
	if last.Summary == nil {
		t.Fatal("stream ended without a summary")
	}
	if _, ok := s.session(last.Session); !ok {
		t.Errorf("session %q from the stream is not resolvable", last.Session)
	}
}

func TestPutSession_forgetsTheOldest(t *testing.T) {
	s := testServer(t)
	ids := make([]string, 0, maxSessions+2)
	for range maxSessions + 2 {
		ids = append(ids, s.putSession(&scanner.Summary{}))
	}
	if _, ok := s.session(ids[0]); ok {
		t.Error("the oldest session should have been dropped")
	}
	if _, ok := s.session(ids[len(ids)-1]); !ok {
		t.Error("the newest session must still resolve")
	}
}

// organize, recreate and export all write to disk, so one test drives all three
// against a throwaway directory and checks what actually landed there.
func TestAPI_organizeRecreateExportWriteWhereTheyShould(t *testing.T) {
	dir := t.TempDir()
	withQR := filepath.Join(dir, "code.png")
	if err := os.WriteFile(withQR, mustQRPNG(t, "https://example.com/api-test"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := testServer(t)
	rec := do(t, s, "POST", "/api/scan", allowedOrigin, s.token,
		jsonBody(t, map[string]any{"paths": []string{dir}}))
	if rec.Code != http.StatusOK {
		t.Fatalf("scan = %d (%s)", rec.Code, rec.Body.String())
	}
	lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
	var last struct {
		Session string `json:"session"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatal(err)
	}
	body := func(format string) string {
		return jsonBody(t, map[string]any{"session": last.Session, "format": format})
	}

	if rec := do(t, s, "POST", "/api/export", allowedOrigin, s.token,
		body("json")); rec.Code != http.StatusOK {
		t.Fatalf("export = %d", rec.Code)
	}
	if rec := do(t, s, "POST", "/api/recreate", allowedOrigin, s.token,
		body("png")); rec.Code != http.StatusOK {
		t.Fatalf("recreate = %d", rec.Code)
	}
	if entries, err := os.ReadDir(filepath.Join(dir, "recreated_qr")); err != nil || len(entries) == 0 {
		t.Errorf("recreate wrote nothing into recreated_qr/: %v", err)
	}

	if rec := do(t, s, "POST", "/api/organize", allowedOrigin, s.token,
		body("")); rec.Code != http.StatusOK {
		t.Fatalf("organize = %d", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(dir, "with_qr", "code.png")); err != nil {
		t.Errorf("organize did not move the image into with_qr/: %v", err)
	}

	found := false
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "qr_scan_") && strings.HasSuffix(e.Name(), ".json") {
			found = true
		}
	}
	if !found {
		t.Error("export left no qr_scan_*.json in the scanned folder")
	}
}

func mustQRPNG(t *testing.T, content string) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "qr.png")
	if err := exporter.WriteQRCodeFormat(content, path, exporter.QRFormatPNG); err != nil {
		t.Fatalf("build a QR fixture: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
