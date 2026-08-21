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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

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
	for _, e := range entries {
		decoded, derr := ScanImage(e.path)
		codesExpected += len(e.expected)

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

	images := len(entries)
	t.Logf("corpus: %s", abs)
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
