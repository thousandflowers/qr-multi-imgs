//go:build corpus

package scanner

// Ground-truth decode benchmark, tagged `corpus` so it never runs in normal CI.
//
//	go test -tags corpus ./scanner -run TestCorpus -v
//
// Point it at a private photo set that is never committed (add -timeout 0 for
// large sets; the harness is sequential and Vision is not fast):
//
//	QR_CORPUS_DIR=~/Pictures/qr-set go test -tags corpus ./scanner -run TestCorpus -v -timeout 0 -count=1
//
// -count=1 matters on reruns: go test caches by source+env, not by corpus
// content, so editing images or the manifest otherwise replays a stale result.
//
// The directory must contain corpus.csv. See testdata/README.md.

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// corpusMasks re-enables the companion .npy short circuit, which the harness
// ignores by default.
//
//	go test -tags corpus ./scanner -run TestCorpus -corpus.masks
//
// Masks are off by default because a mask is the generator's own answer key
// lying beside the image: reading it measures a path on which no decoding
// happens. The dataset this project benchmarks against ships one beside all
// 3332 images, so with masks on the headline number is the speed of parsing
// answers. The flag is kept because that path is real code that ships and a
// regression in it should be catchable - it is simply not the decoder's score.
var corpusMasks = flag.Bool("corpus.masks", false,
	"benchmark with the companion .npy masks enabled (default: ignore them and force every image through the raster loop)")

// corpusDump writes one row per image to a CSV, for characterising failures
// rather than counting them.
//
//	go test -tags corpus ./scanner -run TestCorpus -corpus.dump=/tmp/rows.csv
//
// The summary the harness logs says how many images missed. It cannot say what
// the misses have in common, and "what do they have in common" is the question
// that decides which retry strategies are worth writing: an image whose finder
// patterns were located and whose payload would not come out is a binarization
// and contrast problem, and an image where nothing was detected at all is a
// detection problem. Those want different code.
var corpusDump = flag.String("corpus.dump", "",
	"write a per-image CSV of outcome, classification and metadata to this path")

// expectedFail marks a manifest row whose image contains no QR at all, so the
// correct outcome is "decoded nothing".
const expectedFail = "EXPECTED_FAIL"

const (
	defaultCorpusDir = "testdata/corpus"
	manifestName     = "corpus.csv"
)

// corpusEntry is one image and every payload it is expected to yield, in
// reading order. An empty expected list means the image holds no QR at all.
type corpusEntry struct {
	path     string // absolute
	name     string // as written in the manifest, for reporting
	expected []string
}

// readManifest parses dir/corpus.csv into entries. Rows starting with # are
// comments; an optional "path,expected" header row is skipped.
//
// An image with several codes is written as several rows sharing a path, one
// per payload, in reading order. Keeping two fields per row rather than
// widening to ragged columns preserves FieldsPerRecord as a typo guard: in a
// hand-edited manifest an unescaped comma would otherwise turn one payload
// into two silently.
func readManifest(dir string) ([]corpusEntry, error) {
	f, err := os.Open(filepath.Join(dir, manifestName))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comment = '#'
	r.FieldsPerRecord = 2
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	var entries []corpusEntry
	index := make(map[string]int)
	for _, row := range rows {
		name, expected := row[0], row[1]
		if name == "path" && expected == "expected" {
			continue
		}
		i, seen := index[name]
		if !seen {
			index[name] = len(entries)
			entries = append(entries, corpusEntry{
				path: filepath.Join(dir, filepath.FromSlash(name)),
				name: name,
			})
			i = len(entries) - 1
		}
		if expected == expectedFail {
			if len(entries[i].expected) > 0 {
				return nil, fmt.Errorf("%s: %s mixed with real payloads", name, expectedFail)
			}
			continue
		}
		if seen && len(entries[i].expected) == 0 {
			return nil, fmt.Errorf("%s: real payload mixed with %s", name, expectedFail)
		}
		entries[i].expected = append(entries[i].expected, expected)
	}
	return entries, nil
}

// score compares what an image decoded against what it was expected to yield.
// matched counts payloads found that were expected, spurious counts payloads
// found that were not. Both are multiset counts, so two codes carrying the
// same payload need both to be found to score two.
func score(expected, decoded []string) (matched, spurious int) {
	remaining := make(map[string]int, len(expected))
	for _, e := range expected {
		remaining[e]++
	}
	for _, d := range decoded {
		if remaining[d] > 0 {
			remaining[d]--
			matched++
		} else {
			spurious++
		}
	}
	return matched, spurious
}

func TestCorpus(t *testing.T) {
	dir := os.Getenv("QR_CORPUS_DIR")
	if dir == "" {
		dir = defaultCorpusDir
	}
	runCorpus(t, dir)
}

// runCorpus scores one corpus directory. It is separate from TestCorpus so a
// second corpus - a HEIC photo set, see corpus_heic_test.go - is scored by the
// same code rather than by a copy of it that can drift.
func runCorpus(t *testing.T, dir string) {
	t.Helper()

	// The mask short circuit is process-wide state. The harness is sequential
	// and this is the only writer, but restoring it keeps a second test in the
	// same binary honest.
	prevMasks := npyMaskEnabled
	npyMaskEnabled = *corpusMasks
	defer func() { npyMaskEnabled = prevMasks }()

	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("resolve corpus dir %q: %v", dir, err)
	}

	entries, err := readManifest(abs)
	if err != nil {
		t.Fatalf("read manifest in %s: %v (set QR_CORPUS_DIR to a directory containing %s)", abs, err, manifestName)
	}
	if len(entries) == 0 {
		t.Fatalf("manifest %s lists no images", filepath.Join(abs, manifestName))
	}
	requireReadableFormats(t, entries)

	// Per-image outcomes.
	var exact, partial, missed, falsePositive, correctNegative, errored int
	// Per-code outcomes, which is where recall actually lives: an image with
	// three codes where two decode is neither a pass nor a plain failure.
	var codesExpected, codesMatched, codesSpurious int
	var failures []string

	// The harness measures the CLI path — ScanImage, i.e. ScanFast — on
	// purpose. Ground truth is built with ScanExhaustive by corpusgen, so the
	// gap between the two numbers is exactly the set of codes the fast path
	// misses and Vision recovers, and what a browser build can never reach.
	// It cannot pass an expected count: ScanMode has no field for one.
	//
	// ponytail: sequential. Thousands of photos will take minutes — parallelise
	// with the scanWorkers pool from scanner.go if that becomes the bottleneck.
	// Which stage read each image, counted across the corpus. This is the
	// baseline a retry ladder is measured against: a rung that never appears
	// here as the only stage that found anything has bought nothing.
	wins := map[string]int{}
	ran := map[string]int{}
	dump := newDumpWriter(t, *corpusDump)
	defer dump.close(t)
	start := time.Now()

	for _, e := range entries {
		detail, derr := ScanImageDetail(e.path, ScanFast)
		decoded := detail.Codes
		codesExpected += len(e.expected)
		dump.row(t, e, detail, derr)
		if m := detail.Metadata; m != nil {
			for _, a := range m.Strategies {
				ran[a.Name]++
				if a.Found > 0 {
					wins[a.Name]++
				}
			}
		}

		if derr != nil {
			errored++
			failures = append(failures, fmt.Sprintf("ERROR    %s: %v", e.name, derr))
			continue
		}

		matched, spurious := score(e.expected, decoded)
		codesMatched += matched
		codesSpurious += spurious

		switch {
		case len(e.expected) == 0 && len(decoded) == 0:
			correctNegative++
		case len(e.expected) == 0:
			falsePositive++
			failures = append(failures, fmt.Sprintf("FALSEPOS %s: expected no QR, decoded %q", e.name, decoded))
		case spurious > 0:
			falsePositive++
			failures = append(failures, fmt.Sprintf("FALSEPOS %s: %d/%d expected codes, plus %d not in the manifest: decoded %q",
				e.name, matched, len(e.expected), spurious, decoded))
		case matched == len(e.expected):
			exact++
		case matched == 0:
			missed++
			failures = append(failures, fmt.Sprintf("MISSED   %s: 0/%d codes, expected %q", e.name, len(e.expected), e.expected))
		default:
			partial++
			failures = append(failures, fmt.Sprintf("PARTIAL  %s: %d/%d codes, missing %q",
				e.name, matched, len(e.expected), missingFrom(e.expected, decoded)))
		}
	}

	sort.Strings(failures)
	for _, line := range failures {
		t.Log(line)
	}

	elapsed := time.Since(start)

	images := len(entries)
	t.Logf("corpus: %s", abs)
	if *corpusMasks {
		t.Logf("masks:  USED - a .npy beside an image is decoded instead of its pixels,")
		t.Logf("        so these numbers are not a measure of image decoding")
	} else {
		t.Logf("masks:  ignored - every image forced through the raster loop")
	}
	t.Logf("images:  %d total", images)
	t.Logf("  exact:            %d", exact)
	t.Logf("  partial:          %d", partial)
	t.Logf("  missed:           %d", missed)
	t.Logf("  false positive:   %d", falsePositive)
	t.Logf("  correct negative: %d", correctNegative)
	if errored > 0 {
		t.Logf("  errors:           %d", errored)
	}
	t.Logf("codes:   %d expected, %d decoded, %d spurious", codesExpected, codesMatched, codesSpurious)
	t.Logf("per-image exact rate: %.2f%%", pct(exact+correctNegative, images))
	t.Logf("per-code recall:      %.2f%%", pct(codesMatched, codesExpected))
	t.Logf("wall clock: %s total, %s per image, %.1f img/s",
		elapsed.Round(time.Millisecond),
		(elapsed / time.Duration(images)).Round(time.Microsecond),
		float64(images)/elapsed.Seconds())

	// Stages in the order they run, so the table reads as the cascade does.
	t.Logf("stages (ran -> found something):")
	for _, name := range stageOrder(ran) {
		t.Logf("  %-16s %6d ran  %6d found  %6.2f%%", name, ran[name], wins[name], pct(wins[name], ran[name]))
	}
}

// stageOrder lists the raster strategies in cascade order first, then any
// other stage that ran, so the log matches the order work actually happened in
// rather than map iteration order.
func stageOrder(ran map[string]int) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range strategies {
		if n := s.name(); ran[n] > 0 {
			out = append(out, n)
			seen[n] = true
		}
	}
	var rest []string
	for n := range ran {
		if !seen[n] {
			rest = append(rest, n)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(n) / float64(total)
}

// missingFrom returns the expected payloads that were not decoded, as a
// multiset difference.
func missingFrom(expected, decoded []string) []string {
	have := make(map[string]int, len(decoded))
	for _, d := range decoded {
		have[d]++
	}
	var out []string
	for _, e := range expected {
		if have[e] > 0 {
			have[e]--
			continue
		}
		out = append(out, e)
	}
	return out
}

// requireReadableFormats stops a run that this build physically cannot read
// before it produces a number.
//
// QR_CORPUS_DIR takes any directory with a manifest, and a manifest may name
// HEIC photos - that is the second corpus this harness is for, a set of real
// iPhone pictures rather than generated PNGs. Nothing in the loader or the
// scoring needs to change for it: paths are opened by name and ScanImageDetail
// does not filter by extension.
//
// What does change is whether the binary can open them at all. HEIC needs
// either -tags heic (a pure-Go decoder, off by default for the licensing
// reason in heic_enabled.go) or macOS with cgo, where Apple Vision reads them
// from the path. A build with neither decodes nothing and would report 0%
// recall - a number that describes the build and reads like a verdict on the
// decoder. Skipping says which it is.
func requireReadableFormats(t *testing.T, entries []corpusEntry) {
	t.Helper()
	if heicRaster || visionAvailable {
		return
	}
	var heic int
	for _, e := range entries {
		switch strings.ToLower(filepath.Ext(e.path)) {
		case ".heic", ".heif":
			heic++
		}
	}
	if heic == 0 {
		return
	}
	t.Skipf("%d of %d images are HEIC and this build cannot read them: "+
		"rebuild with -tags heic, or run on macOS with cgo where Apple Vision reads them. "+
		"Scoring them now would report 0%% recall for the decoder when the cause is the build.",
		heic, len(entries))
}

// dumpWriter writes the per-image characterisation CSV. A nil path makes every
// method a no-op, so the scoring loop does not branch.
type dumpWriter struct {
	f *os.File
	w *csv.Writer
}

func newDumpWriter(t *testing.T, path string) *dumpWriter {
	t.Helper()
	if path == "" {
		return &dumpWriter{}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create dump %s: %v", path, err)
	}
	w := csv.NewWriter(f)
	if err := w.Write([]string{
		"name", "outcome", "classification",
		"expected_codes", "decoded_codes", "matched", "spurious",
		"width", "height", "laplacian_variance", "edge_density",
		"true_version", "true_modules", "payload_len",
		"detections", "est_version", "est_module_size",
		"stages",
	}); err != nil {
		t.Fatalf("write dump header: %v", err)
	}
	return &dumpWriter{f: f, w: w}
}

func (d *dumpWriter) row(t *testing.T, e corpusEntry, detail Detail, derr error) {
	t.Helper()
	if d.w == nil {
		return
	}

	outcome := "missed"
	matched, spurious := 0, 0
	switch {
	case derr != nil:
		outcome = "error"
	default:
		matched, spurious = score(e.expected, detail.Codes)
		switch {
		case len(e.expected) == 0 && len(detail.Codes) == 0:
			outcome = "correct_negative"
		case len(e.expected) == 0 || spurious > 0:
			outcome = "false_positive"
		case matched == len(e.expected):
			outcome = "exact"
		case matched > 0:
			outcome = "partial"
		}
	}

	// Ground truth from the dataset's own bit-matrix, not from any decode: a
	// version-N QR is 17+4N modules across, and the companion .npy is written
	// with the quiet zone stripped. Absent for a corpus that ships no masks,
	// which is every corpus of real photographs.
	trueVersion, trueModules := 0, 0
	if mask, err := readBoolNPY(strings.TrimSuffix(e.path, filepath.Ext(e.path)) + ".npy"); err == nil && len(mask) > 0 {
		trueModules = len(mask)
		if v := (trueModules - 17) / 4; v >= 1 && v <= 40 && 17+4*v == trueModules {
			trueVersion = v
		}
	}

	payloadLen := 0
	for _, p := range e.expected {
		payloadLen += len(p)
	}

	var m Metadata
	if detail.Metadata != nil {
		m = *detail.Metadata
	}
	estVersion, estModule := 0, 0.0
	if len(detail.Detections) > 0 {
		estVersion = detail.Detections[0].Version
		estModule = detail.Detections[0].ModuleSize
	}

	stages := make([]string, 0, len(m.Strategies))
	for _, a := range m.Strategies {
		stages = append(stages, fmt.Sprintf("%s:%d", a.Name, a.Found))
	}

	if err := d.w.Write([]string{
		e.name, outcome, string(detail.Classification),
		strconv.Itoa(len(e.expected)), strconv.Itoa(len(detail.Codes)), strconv.Itoa(matched), strconv.Itoa(spurious),
		strconv.Itoa(m.Width), strconv.Itoa(m.Height),
		strconv.FormatFloat(m.LaplacianVariance, 'f', 4, 64),
		strconv.FormatFloat(m.EdgeDensity, 'f', 6, 64),
		strconv.Itoa(trueVersion), strconv.Itoa(trueModules), strconv.Itoa(payloadLen),
		strconv.Itoa(len(detail.Detections)), strconv.Itoa(estVersion),
		strconv.FormatFloat(estModule, 'f', 3, 64),
		strings.Join(stages, ";"),
	}); err != nil {
		t.Fatalf("write dump row: %v", err)
	}
}

func (d *dumpWriter) close(t *testing.T) {
	t.Helper()
	if d.w == nil {
		return
	}
	d.w.Flush()
	if err := d.w.Error(); err != nil {
		t.Errorf("flush dump: %v", err)
	}
	if err := d.f.Close(); err != nil {
		t.Errorf("close dump: %v", err)
	}
}
