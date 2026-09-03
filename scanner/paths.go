package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// scanJob is one file queued for scanning, with its stat already taken.
type scanJob struct {
	path string
	fi   os.FileInfo
}

// collectJobs turns a mixed list of files and directories into a work list.
//
// A directory contributes its supported image files, non-recursively, the rule
// a folder scan has always used. A file contributes itself whatever its
// extension: naming a file is an explicit request, and refusing it because the
// extension is unfamiliar would be second-guessing the caller. If it turns out
// to be undecodable its row carries the error, which is the honest answer and
// better than pretending the file was not named.
//
// Paths are deduplicated by cleaned path, first occurrence winning, so passing
// a folder and a file inside it scans that file once rather than twice.
//
// A path that cannot be stat'd is an error rather than a skip. Every path here
// was named by the caller, so a typo should say so instead of silently scanning
// the rest.
func collectJobs(paths []string) ([]scanJob, error) {
	var jobs []scanJob
	seen := make(map[string]bool)

	add := func(path string, fi os.FileInfo) {
		path = filepath.Clean(path)
		if seen[path] {
			return
		}
		seen[path] = true
		jobs = append(jobs, scanJob{path: path, fi: fi})
	}

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			add(p, info)
			continue
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !supportedExtensions[strings.ToLower(filepath.Ext(entry.Name()))] {
				continue
			}
			fi, err := entry.Info()
			if err != nil {
				continue
			}
			add(filepath.Join(p, entry.Name()), fi)
		}
	}
	return jobs, nil
}

// runJobs scans every job on the worker pool and returns the results. onResult,
// when non-nil, is called once per finished file from the single draining
// goroutine, so it needs no synchronisation of its own.
func runJobs(jobs []scanJob, onResult func(ScanResult)) []ScanResult {
	in := make(chan scanJob, len(jobs))
	results := make(chan ScanResult, len(jobs))

	var wg sync.WaitGroup
	for range scanWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range in {
				r := ScanResult{FilePath: j.path, FileSize: j.fi.Size()}
				d, err := ScanImageDetail(j.path, ScanFast)
				if err != nil {
					// An unreadable file gets no classification: its pixels were
					// never seen, so there is nothing to say about them.
					r.Error = err.Error()
				} else {
					r.Classification = d.Classification
					r.Detections = d.Detections
					r.Metadata = d.Metadata
					if len(d.Codes) > 0 {
						r.HasQR = true
						r.Contents = d.Codes
					}
				}
				results <- r
			}
		}()
	}
	for _, j := range jobs {
		in <- j
	}
	close(in)
	go func() {
		wg.Wait()
		close(results)
	}()

	// ponytail: stays nil when nothing scanned, so an empty scan still exports
	// null rather than [] and the JSON shape does not change under this refactor.
	var list []ScanResult
	for r := range results {
		list = append(list, r)
		if onResult != nil {
			onResult(r)
		}
	}
	return list
}

// ScanPaths scans every file the given paths name, directories contribute
// their supported images, files contribute themselves, and returns one Summary
// covering all of them together. Formats may be mixed freely.
func ScanPaths(paths []string) (*Summary, error) {
	start := time.Now()
	jobs, err := collectJobs(paths)
	if err != nil {
		return nil, err
	}
	s := tallySummary(runJobs(jobs, nil), start)
	sortResults(s)
	return s, nil
}

// ScanPathsStream scans like ScanPaths but emits a Progress per finished file,
// ending with one event carrying the Summary. Path validation is synchronous so
// errors surface before scanning starts.
func ScanPathsStream(paths []string) (<-chan Progress, error) {
	start := time.Now()
	jobs, err := collectJobs(paths)
	if err != nil {
		return nil, err
	}
	total := len(jobs)

	out := make(chan Progress, total+1)
	go func() {
		defer close(out)
		done := 0
		list := runJobs(jobs, func(r ScanResult) {
			done++
			out <- Progress{Done: done, Total: total, File: r.FilePath}
		})
		s := tallySummary(list, start)
		sortResults(s)
		out <- Progress{Done: total, Total: total, Summary: s}
	}()
	return out, nil
}

// mustBeDir keeps the folder entry points rejecting a file path, which they
// have always done and which ScanPaths deliberately does not.
func mustBeDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", dir)
	}
	return nil
}
