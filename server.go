package main

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/thousandflowers/qr-multi-imgs/exporter"
	"github.com/thousandflowers/qr-multi-imgs/scanner"
)

// The API can delete files, so it is built on three assumptions and each one is
// enforced here rather than documented and hoped for:
//
//  1. It listens on the loopback only. See loopbackAddr.
//  2. Every call carries a random token in a *header*. A header forces a CORS
//     preflight, so a hostile page cannot fire a request and ignore the reply.
//  3. Destructive calls name a session, never a path. The worst a leaked token
//     can do is repeat an action on files you already chose to scan.
const (
	tokenHeader   = "X-QMI-Token"
	allowedOrigin = "https://thousandflowers.github.io"
	defaultPort   = "8787"
	maxSessions   = 8
)

//go:embed all:web
var webFS embed.FS

type apiServer struct {
	token string

	mu       sync.Mutex
	sessions map[string]*scanner.Summary
	order    []string
}

func newAPIServer() (*apiServer, error) {
	buf := make([]byte, 24) // 32 base64url characters
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}
	return &apiServer{
		token:    base64.RawURLEncoding.EncodeToString(buf),
		sessions: make(map[string]*scanner.Summary),
	}, nil
}

// putSession keeps the last maxSessions scans addressable and forgets the rest,
// so a long-lived server does not hold every scan of the day in memory.
func (s *apiServer) putSession(sum *scanner.Summary) string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		// Unreachable in practice; a predictable id is still gated by the token.
		buf = []byte(fmt.Sprintf("%d", len(s.sessions)))
	}
	id := base64.RawURLEncoding.EncodeToString(buf)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = sum
	s.order = append(s.order, id)
	for len(s.order) > maxSessions {
		delete(s.sessions, s.order[0])
		s.order = s.order[1:]
	}
	return id
}

func (s *apiServer) session(id string) (*scanner.Summary, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sum, ok := s.sessions[id]
	return sum, ok
}

// loopbackAddr normalises a --serve value and refuses anything that would put
// the delete endpoint on the network. "" and ":8787" and "8787" all mean the
// same thing; "0.0.0.0:8787" is an error, not a preference.
func loopbackAddr(in string) (string, error) {
	if in == "" {
		return "127.0.0.1:" + defaultPort, nil
	}
	if !strings.Contains(in, ":") {
		return "127.0.0.1:" + in, nil
	}
	host, port, err := net.SplitHostPort(in)
	if err != nil {
		return "", fmt.Errorf("bad address %q: %w", in, err)
	}
	if port == "" {
		port = defaultPort
	}
	switch host {
	case "", "localhost", "127.0.0.1", "::1":
		return "127.0.0.1:" + port, nil
	}
	return "", fmt.Errorf(
		"refusing to listen on %q: this API deletes files, so it binds 127.0.0.1 only", host)
}

func isAllowedOrigin(origin string) bool {
	if origin == allowedOrigin {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" {
		return false
	}
	// The UI the binary serves itself, on whatever port it was given.
	return u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost"
}

func (s *apiServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/scan", s.handleScan)
	mux.HandleFunc("/api/delete", s.handleDelete)
	mux.HandleFunc("/api/organize", s.handleOrganize)
	mux.HandleFunc("/api/recreate", s.handleRecreate)
	mux.HandleFunc("/api/export", s.handleExport)

	sub, err := fs.Sub(webFS, "web")
	if err == nil {
		// Compressed, and cacheable under a fingerprinted path. See webassets.go.
		mux.Handle("/", newWebAssets(sub))
	}
	return s.guard(mux)
}

// guard applies to /api/ only: the UI itself must load before it can present a
// token, and serving static files to the loopback gives nothing away.
func (s *apiServer) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			// frame-ancestors is ignored in a <meta>, so the page cannot set it
			// itself; served from here it stops the UI being framed.
			w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			next.ServeHTTP(w, r)
			return
		}

		// Origin is checked before the token: a token can leak, but a page
		// cannot lie about which site it is.
		origin := r.Header.Get("Origin")
		if origin != "" {
			if !isAllowedOrigin(origin) {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", tokenHeader+", Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
			// Chrome will not let a public page reach 127.0.0.1 without this.
			if r.Header.Get("Access-Control-Request-Private-Network") == "true" {
				w.Header().Set("Access-Control-Allow-Private-Network", "true")
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if subtle.ConstantTimeCompare([]byte(r.Header.Get(tokenHeader)), []byte(s.token)) != 1 {
			http.Error(w, "bad or missing token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *apiServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"ok": true, "version": versionString()})
}

// wireSummary is scanner.Summary with a duration a browser can read.
type wireSummary struct {
	Total     int `json:"total"`
	WithQR    int `json:"with_qr"`
	WithoutQR int `json:"without_qr"`
	// Detected is the part of WithoutQR holding a code that was located and
	// could not be read. It ships because a client that only sees without_qr
	// has to merge the two states back together to render a number, which is
	// the bug this whole change removes from the UI.
	Detected   int                  `json:"detected"`
	Errors     int                  `json:"errors"`
	TotalSize  int64                `json:"total_size"`
	DurationMs int64                `json:"duration_ms"`
	Results    []scanner.ScanResult `json:"results"`
}

func toWire(s *scanner.Summary) wireSummary {
	return wireSummary{
		Total: s.Total, WithQR: s.WithQR, WithoutQR: s.WithoutQR,
		Detected: s.Detected, Errors: s.Errors,
		TotalSize: s.TotalSize, DurationMs: s.Duration.Milliseconds(), Results: s.Results,
	}
}

type scanEvent struct {
	Done    int          `json:"done"`
	Total   int          `json:"total"`
	File    string       `json:"file,omitempty"`
	Session string       `json:"session,omitempty"`
	Summary *wireSummary `json:"summary,omitempty"`
}

// handleScan streams newline-delimited JSON rather than Server-Sent Events:
// EventSource cannot set a request header, and the token lives in one.
func (s *apiServer) handleScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request body", http.StatusBadRequest)
		return
	}
	if len(req.Paths) == 0 {
		http.Error(w, "no paths given", http.StatusBadRequest)
		return
	}

	// Validation is synchronous, so a bad path is a 400 and not a half-written
	// stream the page has to unpick.
	ch, err := scanner.ScanPathsStream(req.Paths)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)

	for p := range ch {
		ev := scanEvent{Done: p.Done, Total: p.Total, File: p.File}
		if p.Summary != nil {
			wire := toWire(p.Summary)
			ev.Summary = &wire
			ev.Session = s.putSession(p.Summary)
		}
		if err := enc.Encode(ev); err != nil {
			return // the page went away; the scan goroutine drains on its own
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
}

// sessionOf reads the session a mutating call names and resolves it. A caller
// never names a path, which is what keeps a leaked token from being a wipe.
func (s *apiServer) sessionOf(w http.ResponseWriter, r *http.Request) (*scanner.Summary, string, bool) {
	var req struct {
		Session string `json:"session"`
		Format  string `json:"format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request body", http.StatusBadRequest)
		return nil, "", false
	}
	sum, ok := s.session(req.Session)
	if !ok {
		http.Error(w, "unknown session: scan again", http.StatusNotFound)
		return nil, "", false
	}
	return sum, req.Format, true
}

func (s *apiServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	sum, _, ok := s.sessionOf(w, r)
	if !ok {
		return
	}
	writeJSON(w, deleteWithoutQRNow(sum))
}

func (s *apiServer) handleOrganize(w http.ResponseWriter, r *http.Request) {
	sum, _, ok := s.sessionOf(w, r)
	if !ok {
		return
	}
	writeJSON(w, organizeByQRNow(sum))
}

func (s *apiServer) handleRecreate(w http.ResponseWriter, r *http.Request) {
	sum, format, ok := s.sessionOf(w, r)
	if !ok {
		return
	}
	if format == "" {
		format = "png"
	}
	writeJSON(w, recreateQRsNow(sum, format))
}

func (s *apiServer) handleExport(w http.ResponseWriter, r *http.Request) {
	sum, format, ok := s.sessionOf(w, r)
	if !ok {
		return
	}
	if format == "" {
		format = "json"
	}
	path, err := exporter.Export(sum.Results, exporter.ExportFormat(format), exportDir(sum))
	if err != nil {
		writeJSON(w, actionOutcome{Message: "Export failed.", Errors: []string{err.Error()}})
		return
	}
	writeJSON(w, actionOutcome{Message: fmt.Sprintf("Exported to: %s", path)})
}

// serveAddr reads --serve / --serve=ADDR without disturbing parseArgs, which
// already ignores anything starting with a dash. Only the `=` form takes an
// address: `--serve ./photos` would otherwise silently treat the folder as a
// bind address.
func serveAddr(args []string) (string, bool) {
	for _, a := range args {
		if a == "--serve" {
			return "", true
		}
		if strings.HasPrefix(a, "--serve=") {
			return strings.TrimPrefix(a, "--serve="), true
		}
	}
	return "", false
}

func runServer(addrIn string) error {
	addr, err := loopbackAddr(addrIn)
	if err != nil {
		return err
	}
	s, err := newAPIServer()
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	origin := "http://" + ln.Addr().String()

	// The token rides in the URL fragment: browsers never put a fragment in a
	// request, a Referer header or a server log.
	fmt.Printf("qr-multi-imgs %s, local API on %s\n\n", versionString(), origin)
	fmt.Printf("  Local UI   %s/#t=%s\n", origin, s.token)
	fmt.Printf("  Hosted UI  %s/qr-multi-imgs/#t=%s&api=%s\n\n", allowedOrigin, s.token, origin)
	fmt.Println("Anyone holding that URL can scan, move and delete files on this machine.")
	fmt.Println("Ctrl-C to stop.")

	return http.Serve(ln, s.handler())
}
