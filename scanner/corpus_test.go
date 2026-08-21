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

type corpusEntry struct {
	path     string // absolute
	name     string // as written in the manifest, for reporting
	expected string
}

// readManifest parses dir/corpus.csv into entries. Rows starting with # are
// comments; an optional "path,expected" header row is skipped.
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
	for _, row := range rows {
		if row[0] == "path" && row[1] == "expected" {
			continue
		}
		entries = append(entries, corpusEntry{
			path:     filepath.Join(dir, filepath.FromSlash(row[0])),
			name:     row[0],
			expected: row[1],
		})
	}
	return entries, nil
}

// decodeOne runs the production decode path and flattens it to a single
// payload: "" means nothing decoded.
func decodeOne(path string) (string, error) {
	contents, err := ScanImage(path)
	if err != nil {
		return "", err
	}
	if len(contents) == 0 {
		return "", nil
	}
	return contents[0], nil
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

	var correct, wrong, missed, errored int
	// failures collects one line per non-correct row, for eyeballing afterwards.
	var failures []string

	// ponytail: sequential. Thousands of photos will take minutes — parallelise
	// with the scanWorkers pool from scanner.go if that becomes the bottleneck.
	for _, e := range entries {
		got, derr := decodeOne(e.path)
		switch {
		case derr != nil:
			errored++
			failures = append(failures, fmt.Sprintf("ERROR   %s: %v", e.name, derr))
		case e.expected == expectedFail:
			if got == "" {
				correct++
			} else {
				wrong++
				failures = append(failures, fmt.Sprintf("FALSEPOS %s: expected no QR, decoded %q", e.name, got))
			}
		case got == e.expected:
			correct++
		case got == "":
			missed++
			failures = append(failures, fmt.Sprintf("MISSED  %s: expected %q", e.name, e.expected))
		default:
			wrong++
			failures = append(failures, fmt.Sprintf("WRONG   %s: expected %q, decoded %q", e.name, e.expected, got))
		}
	}

	total := len(entries)
	rate := 100 * float64(correct) / float64(total)

	sort.Strings(failures)
	for _, line := range failures {
		t.Log(line)
	}

	t.Logf("corpus:  %s", abs)
	t.Logf("total:   %d", total)
	t.Logf("correct: %d", correct)
	t.Logf("wrong:   %d", wrong)
	t.Logf("missed:  %d", missed)
	t.Logf("errors:  %d", errored)
	t.Logf("success rate: %.2f%%", rate)
}
