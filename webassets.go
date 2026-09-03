package main

// Serving the page fast enough that nobody thinks about it.
//
// Two things were wrong and both were measured against the local server rather
// than guessed at: nothing was compressed, and nothing could be cached. The
// page and its scripts are 5.4 MB of text and WebAssembly, which gzip takes to
// 1.7 MB; and every reload fetched all of it again, because the responses said
// nothing about how long they stay good.
//
// Compression is the easy half. Caching is the half that can go wrong, because
// a file cached for a year is a file that cannot be corrected for a year. So
// nothing is cached under its own name: the same files are also served under
// /a/<fingerprint>/, the fingerprint is the hash of every embedded web file,
// and only that path is marked immutable. Upgrade the binary and the
// fingerprint changes, which changes the URLs, which means the old ones are
// simply never asked for again. The page itself is always revalidated, and it
// is what carries the new fingerprint.
//
// A worker resolves importScripts against its own URL, so loading the worker
// from the fingerprinted path takes wasm_exec.js, result.js and native.js with
// it without a single rewritten string inside it.
//
// On GitHub Pages none of this applies: there is no process to rewrite
// anything, the plain paths are served, and Pages does its own compression.

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// assetPrefix is where the fingerprinted copies live.
const assetPrefix = "/a/"

// gzipMin is the size below which compressing costs more than it saves. A few
// hundred bytes of headers and a deflate stream is not a saving on a 200-byte
// file.
const gzipMin = 1024

type webAssets struct {
	fsys        fs.FS
	fingerprint string
	index       []byte // index.html with its references pointed at the fingerprint
	inner       http.Handler
}

// newWebAssets fingerprints the embedded site and rewrites the page's own
// references to point at the immutable copies.
func newWebAssets(fsys fs.FS) *webAssets {
	w := &webAssets{fsys: fsys, fingerprint: fingerprintFS(fsys)}
	w.inner = http.FileServer(http.FS(fsys))
	if raw, err := fs.ReadFile(fsys, "index.html"); err == nil {
		w.index = rewriteRefs(raw, assetPrefix+w.fingerprint+"/")
	}
	return w
}

// fingerprintFS hashes every file in the site, names included, so that adding
// or removing one changes the answer as surely as editing one does.
func fingerprintFS(fsys fs.FS) string {
	var names []string
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, p)
		}
		return nil
	})
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		h.Write([]byte(n))
		if b, err := fs.ReadFile(fsys, n); err == nil {
			h.Write(b)
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// The three shapes in which index.html names a file of its own: a script tag,
// the worker it starts, and the two fetches of the wasm module.
var refPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(<script src=")([\w.-]+\.js)(")`),
	regexp.MustCompile(`(new Worker\(")([\w.-]+\.js)(")`),
	regexp.MustCompile(`(fetch\(")([\w.-]+\.wasm)(")`),
}

func rewriteRefs(html []byte, prefix string) []byte {
	for _, re := range refPatterns {
		html = re.ReplaceAll(html, []byte("${1}"+prefix+"${2}${3}"))
	}
	return html
}

func (w *webAssets) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	immutable := false
	if rest, ok := strings.CutPrefix(upath, assetPrefix+w.fingerprint+"/"); ok {
		// A hit on the current fingerprint: this content can never change
		// under this URL, which is what lets it be kept for a year.
		immutable = true
		r = r.Clone(r.Context())
		r.URL.Path = "/" + rest
		upath = r.URL.Path
	} else if strings.HasPrefix(upath, assetPrefix) {
		// An older fingerprint, from a page cached before an upgrade. Serve the
		// file it is asking for rather than a 404, but do not let it be kept.
		r = r.Clone(r.Context())
		r.URL.Path = "/" + path.Base(upath)
		upath = r.URL.Path
	}

	if immutable {
		rw.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		// Always ask. With an ETag the answer is usually 304 and costs nothing.
		rw.Header().Set("Cache-Control", "no-cache")
	}

	// The page is the one file that is rewritten, so it is served from memory
	// rather than from the embedded copy.
	if w.index != nil && (upath == "/" || upath == "/index.html") {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		rw.Header().Set("ETag", `"`+w.fingerprint+`"`)
		if match := r.Header.Get("If-None-Match"); match == `"`+w.fingerprint+`"` {
			rw.WriteHeader(http.StatusNotModified)
			return
		}
		writeMaybeGzip(rw, r, w.index)
		return
	}

	if !acceptsGzip(r) {
		w.inner.ServeHTTP(rw, r)
		return
	}
	body, err := fs.ReadFile(w.fsys, strings.TrimPrefix(upath, "/"))
	if err != nil || len(body) < gzipMin {
		w.inner.ServeHTTP(rw, r)
		return
	}
	if ct := typeByPath(upath); ct != "" {
		rw.Header().Set("Content-Type", ct)
	}
	writeMaybeGzip(rw, r, body)
}

// typeByPath covers the extensions this site actually ships. Go's own table
// has no answer for .wasm on every platform, and a wrong or missing type is
// what makes WebAssembly.compileStreaming refuse the stream.
func typeByPath(p string) string {
	switch {
	case strings.HasSuffix(p, ".js"):
		return "text/javascript; charset=utf-8"
	case strings.HasSuffix(p, ".wasm"):
		return "application/wasm"
	case strings.HasSuffix(p, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(p, ".png"):
		return "image/png"
	case strings.HasSuffix(p, ".xml"):
		return "application/xml"
	case strings.HasSuffix(p, ".txt"):
		return "text/plain; charset=utf-8"
	}
	return ""
}

func acceptsGzip(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
}

// gzipPool keeps the writers rather than allocating one per response: the
// wasm module alone is 5 MB through the same compressor on every cold load.
var gzipPool = sync.Pool{New: func() any {
	zw, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
	return zw
}}

func writeMaybeGzip(rw http.ResponseWriter, r *http.Request, body []byte) {
	if !acceptsGzip(r) || len(body) < gzipMin {
		rw.Header().Set("Content-Length", itoaLen(len(body)))
		_, _ = rw.Write(body)
		return
	}
	var buf bytes.Buffer
	zw := gzipPool.Get().(*gzip.Writer)
	zw.Reset(&buf)
	if _, err := zw.Write(body); err != nil || zw.Close() != nil {
		gzipPool.Put(zw)
		_, _ = rw.Write(body)
		return
	}
	gzipPool.Put(zw)
	rw.Header().Set("Content-Encoding", "gzip")
	rw.Header().Set("Vary", "Accept-Encoding")
	rw.Header().Set("Content-Length", itoaLen(buf.Len()))
	_, _ = rw.Write(buf.Bytes())
}

func itoaLen(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
