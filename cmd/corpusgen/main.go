// Command corpusgen bootstraps a corpus.csv manifest from a directory of
// unlabelled images.
//
//	go run ./cmd/corpusgen /path/to/photos
//
// It runs the production scanner.ScanImage over every supported image under
// the directory, recursively, and writes a manifest in the format that
// scanner's TestCorpus reads. Images that decoded become live rows carrying
// the decoded payload as the expected value. Images that did not decode are
// written commented out and pre-labelled EXPECTED_FAIL, ready to be
// uncommented and corrected by hand.
//
// The result is a BOOTSTRAP, not ground truth: nothing here verifies that a
// decoded payload is what the QR actually says. See the header it writes.
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/thousandflowers/qr-multi-imgs/scanner"
)

// expectedFail is the manifest's label for an image that contains no QR.
const expectedFail = "EXPECTED_FAIL"

// supportedExtensions mirrors scanner.supportedExtensions, which is
// unexported. Filtering by extension matters for more than speed: ScanImage
// derives a companion mask path by swapping the extension for .npy, so handing
// it a .npy file would make it decode that file as its own mask and land a
// non-image in the manifest. Keep in sync with scanner/scanner.go.
var supportedExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true,
	".gif": true, ".bmp": true, ".webp": true,
}

func defaultJobs() int {
	// Mirrors scanner's own pool: decoding is CPU-bound plus an occasional
	// exec, so more workers than this stops helping.
	if n := runtime.NumCPU(); n < 6 {
		return n
	}
	return 6
}

type result struct {
	rel     string // slash-separated, relative to the corpus root
	payload string
	decoded bool
	err     error
}

func main() {
	out := flag.String("o", "", "output manifest path (default <dir>/corpus.csv)")
	force := flag.Bool("f", false, "overwrite the output file if it already exists")
	jobs := flag.Int("j", defaultJobs(), "images decoded concurrently")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: corpusgen [flags] <directory>\n\n")
		fmt.Fprintf(os.Stderr, "Bootstraps a corpus.csv from unlabelled images. Decoded payloads are\n")
		fmt.Fprintf(os.Stderr, "ASSUMED CORRECT and are not ground truth.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(flag.Arg(0), *out, *force, *jobs); err != nil {
		fmt.Fprintf(os.Stderr, "corpusgen: %v\n", err)
		os.Exit(1)
	}
}

func run(root, out string, force bool, jobs int) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", root)
	}
	if out == "" {
		out = filepath.Join(root, "corpus.csv")
	}
	// Never clobber a manifest that has already been labelled by hand.
	if _, err := os.Stat(out); err == nil && !force {
		return fmt.Errorf("%s already exists; pass -f to overwrite", out)
	}
	if jobs < 1 {
		jobs = 1
	}

	files, err := listImages(root)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no supported images under %s", root)
	}

	results := scanAll(root, files, jobs)

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	decoded, err := writeManifest(f, root, results)
	if err != nil {
		return err
	}

	total := len(results)
	var errored int
	for _, r := range results {
		if r.err != nil {
			errored++
		}
	}
	undecoded := total - decoded

	fmt.Printf("wrote %s\n", out)
	fmt.Printf("total:      %d\n", total)
	fmt.Printf("decoded:    %d\n", decoded)
	fmt.Printf("undecoded:  %d  (commented out, pre-labelled %s)\n", undecoded, expectedFail)
	if errored > 0 {
		fmt.Printf("read errors: %d  (counted as undecoded)\n", errored)
	}
	fmt.Printf("bootstrap rate: %.2f%%\n", 100*float64(decoded)/float64(total))
	fmt.Printf("\nThis is a BOOTSTRAP, not ground truth: every payload above is whatever\n")
	fmt.Printf("the decoder produced, assumed correct and verified against nothing. A\n")
	fmt.Printf("benchmark run against it measures self-consistency, not accuracy.\n")
	return nil
}

// listImages returns every supported image under root, as slash-separated
// paths relative to root, in lexical order. Unsupported files are skipped
// silently. Relative paths are what makes two files with the same base name in
// different directories distinct rows.
func listImages(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// An unreadable subdirectory should not abort the whole walk.
			fmt.Fprintf(os.Stderr, "corpusgen: skipping %s: %v\n", path, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !supportedExtensions[strings.ToLower(filepath.Ext(d.Name()))] {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return files, err
}

// scanAll decodes every file, preserving input order so the manifest is
// deterministic regardless of how the work interleaved.
func scanAll(root string, files []string, jobs int) []result {
	results := make([]result, len(files))
	var next, done atomic.Int64

	var wg sync.WaitGroup
	for range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1)) - 1
				if i >= len(files) {
					return
				}
				results[i] = scanOne(root, files[i])
				if n := done.Add(1); n%50 == 0 || int(n) == len(files) {
					fmt.Fprintf(os.Stderr, "\rscanned %d/%d", n, len(files))
				}
			}
		}()
	}
	wg.Wait()
	fmt.Fprintln(os.Stderr)

	for _, r := range results {
		if r.err != nil {
			fmt.Fprintf(os.Stderr, "corpusgen: %s: %v\n", r.rel, r.err)
		}
	}
	return results
}

func scanOne(root, rel string) result {
	contents, err := scanner.ScanImage(filepath.Join(root, filepath.FromSlash(rel)))
	switch {
	case err != nil:
		return result{rel: rel, err: err}
	case len(contents) > 0:
		return result{rel: rel, payload: contents[0], decoded: true}
	default:
		return result{rel: rel}
	}
}

const manifestHeader = `# BOOTSTRAP — NOT GROUND TRUTH.
#
# Generated by cmd/corpusgen from: %s
#
# Every payload below is whatever the decoder produced. Nothing verified it
# against the real QR, which is the whole premise of this file: a decoder bug
# reproduces itself here as a "correct" expectation. Benchmarking against this
# manifest unchanged measures self-consistency, not accuracy. Check the
# payloads by hand before treating any number from it as a success rate.
#
# Commented rows are images that did not decode, pre-labelled %s.
# Uncomment and correct each one: %s is right only if the image truly
# has no QR — otherwise replace it with the real payload.
`

// writeManifest writes the header and one row per result, returning how many
// rows were live (decoded). Undecoded rows are written commented out.
func writeManifest(w io.Writer, root string, results []result) (int, error) {
	if _, err := fmt.Fprintf(w, manifestHeader, root, expectedFail, expectedFail); err != nil {
		return 0, err
	}
	if _, err := io.WriteString(w, "path,expected\n"); err != nil {
		return 0, err
	}

	decoded := 0
	for _, r := range results {
		var line string
		var err error
		if r.decoded {
			decoded++
			line, err = encodeRow(r.rel, r.payload)
		} else {
			line, err = encodeRow(r.rel, expectedFail)
			line = commentOut(line)
		}
		if err != nil {
			return decoded, err
		}
		if _, err := io.WriteString(w, line); err != nil {
			return decoded, err
		}
	}
	return decoded, nil
}

// encodeRow renders one path,expected pair as a CSV line. encoding/csv handles
// commas, quotes and newlines in the payload; it does not handle a path
// beginning with '#', which it writes unquoted and which csv.Reader then
// silently discards as a comment, so that case is quoted here by hand.
func encodeRow(path, expected string) (string, error) {
	var b strings.Builder
	cw := csv.NewWriter(&b)
	if err := cw.Write([]string{path, expected}); err != nil {
		return "", err
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return "", err
	}
	line := b.String()
	if strings.HasPrefix(line, "#") {
		// The line starting with '#' proves csv wrote the path unquoted, which
		// in turn proves it holds no quote, comma or newline — so wrapping it
		// in quotes needs no further escaping.
		line = `"` + path + `"` + line[len(path):]
	}
	return line, nil
}

// commentOut prefixes every line of a record with '#'. A record spans more
// than one line only when a field contains a newline.
func commentOut(line string) string {
	trimmed := strings.TrimSuffix(line, "\n")
	parts := strings.Split(trimmed, "\n")
	for i, p := range parts {
		parts[i] = "#" + p
	}
	return strings.Join(parts, "\n") + "\n"
}
