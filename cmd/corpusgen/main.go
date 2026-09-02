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

func defaultJobs() int {
	// Mirrors scanner's own pool: decoding is CPU-bound plus an occasional
	// exec, so more workers than this stops helping.
	if n := runtime.NumCPU(); n < 6 {
		return n
	}
	return 6
}

type result struct {
	rel      string // slash-separated, relative to the corpus root
	payloads []string
	err      error
}

func (r result) decoded() bool { return len(r.payloads) > 0 }

func main() {
	out := flag.String("o", "", "output manifest path (default <dir>/corpus.csv)")
	force := flag.Bool("f", false, "overwrite the output file if it already exists")
	jobs := flag.Int("j", defaultJobs(), "images decoded concurrently")
	truth := flag.String("truth", "", "read each expected payload from a sidecar file with this extension (e.g. .txt) instead of decoding")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: corpusgen [flags] <directory>\n\n")
		fmt.Fprintf(os.Stderr, "Bootstraps a corpus.csv from unlabelled images. Decoded payloads are\n")
		fmt.Fprintf(os.Stderr, "ASSUMED CORRECT and are not ground truth.\n\n")
		fmt.Fprintf(os.Stderr, "With -truth .ext the payloads are read from sidecar files instead, which\n")
		fmt.Fprintf(os.Stderr, "is real ground truth and decodes nothing. This is how a generated\n")
		fmt.Fprintf(os.Stderr, "dataset that ships its own answers (lovasoa/qrcode-dataset) is labelled.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(flag.Arg(0), *out, *force, *jobs, *truth); err != nil {
		fmt.Fprintf(os.Stderr, "corpusgen: %v\n", err)
		os.Exit(1)
	}
}

func run(root, out string, force bool, jobs int, truthExt string) error {
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

	var results []result
	var skipped int
	if truthExt != "" {
		results, skipped = truthAll(root, files, truthExt)
		if len(results) == 0 {
			return fmt.Errorf("no image under %s has a %s sidecar", root, truthExt)
		}
	} else {
		results = scanAll(root, files, jobs)
	}

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	decoded, err := writeManifest(f, root, results, truthExt)
	if err != nil {
		return err
	}

	total := len(results)
	var errored, codes int
	for _, r := range results {
		if r.err != nil {
			errored++
		}
		codes += len(r.payloads)
	}
	undecoded := total - decoded

	fmt.Printf("wrote %s\n", out)
	if truthExt != "" {
		fmt.Printf("labelled:   %d images from %s sidecars\n", total, truthExt)
		fmt.Printf("codes:      %d across %d images (%.2f per image)\n", codes, total, perImage(codes, total))
		if skipped > 0 {
			fmt.Printf("no sidecar: %d  (left out entirely — an unknown payload is not %s)\n", skipped, expectedFail)
		}
		fmt.Printf("\nPayloads came from %s sidecars, not from this decoder, so this manifest\n", truthExt)
		fmt.Printf("is ground truth and a benchmark against it measures accuracy.\n")
		return nil
	}

	fmt.Printf("total:      %d\n", total)
	fmt.Printf("decoded:    %d\n", decoded)
	fmt.Printf("undecoded:  %d  (commented out, pre-labelled %s)\n", undecoded, expectedFail)
	fmt.Printf("codes:      %d across %d images (%.2f per decoded image)\n", codes, decoded, perImage(codes, decoded))
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
func perImage(codes, images int) float64 {
	if images == 0 {
		return 0
	}
	return float64(codes) / float64(images)
}

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
		// Filtering by extension matters for more than speed: ScanImage derives
		// a companion mask path by swapping the extension for .npy, so handing
		// it a .npy file would make it decode that file as its own mask and
		// land a non-image in the manifest.
		if !scanner.SupportsExtension(d.Name()) {
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

// truthAll reads the expected payload for each image from a sidecar file whose
// name is the image's with its extension replaced by ext.
//
// Nothing is decoded here, which is the point: a generated dataset that ships
// the string it encoded knows the answer, and asking this decoder for it
// instead would make the benchmark measure the decoder against itself. That is
// the failure the BOOTSTRAP header exists to warn about, and this path avoids
// it rather than warning about it.
//
// The sidecar's bytes ARE the payload, verbatim — no trimming. A trailing
// newline in the file is a trailing newline in the expected string, because
// the manifest's own rule is that payloads are exact and trailing whitespace
// is preserved. A generator that terminates its answers with a newline has to
// be handled by the person writing the -truth flag, not guessed at here.
//
// An image with no sidecar is left out of the manifest entirely and counted.
// Writing it as EXPECTED_FAIL would assert the image holds no QR, which is a
// claim nothing here supports: a missing answer is an unknown, not a negative.
func truthAll(root string, files []string, ext string) (results []result, skipped int) {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	for _, rel := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		sidecar := strings.TrimSuffix(abs, filepath.Ext(abs)) + ext
		payload, err := os.ReadFile(sidecar)
		if err != nil {
			skipped++
			continue
		}
		results = append(results, result{rel: rel, payloads: []string{string(payload)}})
	}
	return results, skipped
}

// scanOne decodes one image in exhaustive mode: every raster strategy, then
// Apple Vision and zbarimg, all unconditionally, unioned. That is far slower
// than the CLI path — Vision alone costs about a second an image — and it is
// the right trade here. This builds the ground truth everything else is
// measured against, it runs once, and it can run overnight. Ground truth
// costing more than the thing it measures is normal.
func scanOne(root, rel string) result {
	contents, err := scanner.ScanImageMode(filepath.Join(root, filepath.FromSlash(rel)), scanner.ScanExhaustive)
	if err != nil {
		return result{rel: rel, err: err}
	}
	return result{rel: rel, payloads: contents}
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
# An image holding several codes appears as several rows sharing its path,
# one row per payload, in reading order. Two rows with the same payload mean
# two physically distinct codes that happen to carry the same content.
#
# Commented rows are images that did not decode, pre-labelled %s.
# Uncomment and correct each one: %s is right only if the image truly
# has no QR — otherwise replace it with the real payload.
`

const truthManifestHeader = `# GROUND TRUTH, from %s sidecar files.
#
# Generated by cmd/corpusgen -truth %s from: %s
#
# No payload here came from this decoder. Each one was read from the sidecar
# file the dataset ships beside its image, so a benchmark against this manifest
# measures accuracy rather than self-consistency.
#
# Images with no sidecar were left out rather than labelled %s: a missing
# answer is an unknown, not a claim that the image holds no QR.
`

// writeManifest writes the header and one row per result, returning how many
// rows were live (decoded). Undecoded rows are written commented out.
//
// truthExt selects the header only. A sidecar-labelled manifest carries no
// commented rows, because a row exists exactly when an answer was found.
func writeManifest(w io.Writer, root string, results []result, truthExt string) (int, error) {
	header := fmt.Sprintf(manifestHeader, root, expectedFail, expectedFail)
	if truthExt != "" {
		header = fmt.Sprintf(truthManifestHeader, truthExt, truthExt, root, expectedFail)
	}
	if _, err := io.WriteString(w, header); err != nil {
		return 0, err
	}
	if _, err := io.WriteString(w, "path,expected\n"); err != nil {
		return 0, err
	}

	decoded := 0
	for _, r := range results {
		// An image holding several codes becomes several rows sharing a path,
		// one per payload, in the reading order the decoder returned them.
		rows := r.payloads
		commented := false
		if !r.decoded() {
			rows = []string{expectedFail}
			commented = true
		} else {
			decoded++
		}
		for _, payload := range rows {
			line, err := encodeRow(r.rel, payload)
			if err != nil {
				return decoded, err
			}
			if commented {
				line = commentOut(line)
			}
			if _, err := io.WriteString(w, line); err != nil {
				return decoded, err
			}
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
