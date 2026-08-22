# Changelog

All notable changes to qr-multi-imgs are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [v1.5.0] — 2026-08-22

### Added
- Every QR in an image is decoded, not just the first. A list row reports how many
  other codes an image holds, and the export manifest expresses several codes per
  image; the benchmark harness scores per code rather than per image
- Apple Vision returns every code it finds, with corner locations. The cgo bridge
  used to return a single `char*`, so multi-code decoding silently truncated to one
  on the path that matters most for real photos. Points are emitted in original-image
  pixels, top-left origin; geometry is dropped for EXIF-oriented images, which fall
  back to payload matching
- `.heic` / `.heif` support on macOS through Apple Vision, and `.nef` (Nikon raw)
  which Core Image reads directly. Off macOS both report an unreadable format
- Optional pure-Go HEIC behind `-tags heic` (`gen2brain/heic`, libheif via wazero),
  **off by default**: the wrapper is MIT but the embedded libheif is LGPL-3.0 with no
  NOTICE, so no released artifact contains the codec. Verified — a goreleaser snapshot
  holds no occurrence of wazero, gen2brain, libheif or de265. Tagged binary costs
  11.56 MB against 7.21 MB. Tracking gen2brain/heic#17
- `cmd/corpusgen` bootstraps a `corpus.csv` manifest from unlabelled images
- Homebrew formula is published to `thousandflowers/homebrew-tap` automatically on
  every release (goreleaser `brews:`), so `brew` users stop drifting behind
- `scanner.ScanDecodedImage(image.Image) ([]string, error)` — decodes from pixels with
  no file path, for callers such as a wasm build. `ScanImage` now delegates its raster
  stage to it. Lower recall than `ScanImage`: the `.npy` mask, Apple Vision, and
  zbarimg all need a path and do not run
- Several folders and single files scan in one pass: `scanner.ScanPaths` /
  `ScanPathsStream` take a mixed target list and return one `Summary`, and the TUI
  accepts several dropped paths at once. `ScanFolder`/`ScanFolderStream` remain as
  wrappers over the now-shared walk-and-pool code. A named file is scanned whatever
  its extension; a folder walk still filters; a path that does not exist is an error,
  not a silent skip
- The readable-format list is asked of the system rather than hardcoded. On macOS
  `CGImageSourceCopyTypeIdentifiers` supplies it — 70 formats on a current release,
  including every camera raw the OS knows (cr2, cr3, arw, dng, raf, orf, rw2, nef and
  the rest) plus psd, jxl, avif, exr, hdr, dds, ico, icns, pict, jp2. The old list was
  already wrong for anyone shooting Canon, Sony or Fuji and went stale with each new
  camera. Elsewhere the built-in set applies
- TIFF everywhere, via a package already in `go.mod`
- PDFs are read, every page. Rendered through Core Graphics onto white — a QR is black
  vector art over nothing, so an uninitialised bitmap gives black on black — honouring
  `/Rotate` and the crop box via `CGPDFPageGetDrawingTransform`
- Ground-truth decode benchmark behind the `corpus` build tag:
  `go test -tags corpus ./scanner -run TestCorpus -v`. Reads a `corpus.csv` manifest,
  reports total/correct/wrong/missed/errors and a success rate. `QR_CORPUS_DIR` points
  it at an uncommitted private photo set

### Changed
- The raster strategy loop stops on finder-pattern evidence. Decoding every code meant
  unioning all ten strategies, which cost 31.8x on real photos holding a code (40 of
  them: 3.9s → 122.6s, 10.4 img/s → 0.3). A pre-pass counts the QR candidates visible
  across every colour projection and the loop stops once it has decoded that many —
  evidence rather than guesswork. Same 40 photos, best of six runs: 18.4s, 2.2 img/s,
  4.8x. Still slower than v1.4.1's single-code scan; that is the price of not
  under-reporting. A count of zero or a detector error disables early exit rather than
  triggering it, so uncertainty costs time instead of recall. The `.npy` path is
  untouched

### Fixed
- Four places assumed a scan covered exactly one folder and misplaced or misreported
  things once results spanned several: organize moved files beside the *first* result
  (carrying images out of their own folder), recreate wrote `recreated_qr/` in the same
  wrong place, export chose the first result's folder, and the header named it. Files
  are now organized and recreated inside their own folder, export falls back to the
  working directory when a scan spans folders, and the header says `demo  (+2 more
  folders)`. Single-folder scans behave exactly as before
- The version string is stamped at link time (`-ldflags -X main.version`) instead of
  being a constant in `main.go` that had to be bumped by hand and had drifted before.
  A `go install` build with no ldflags falls back to the module version in the build
  info, so it reports the tag it actually installed
- The release workflow now refuses to run without the tap token instead of publishing
  the GitHub release and only then failing the Homebrew bump, which used to leave
  `brew` pinned to the previous version behind a green-looking release
- Removed `Formula/qr-multi-imgs.rb`, a stale v1.2.0 source-build formula that nothing
  consumed and that contradicted the published tap
- HEIC images decode on macOS instead of being skipped or erroring. Three separate
  causes: `ScanImage` returned the `image.Decode` error before any path-based decoder
  ran; Vision could not distinguish "cannot open" from "opened, found nothing", so a
  HEIC with no QR came back as `decode: image: unknown format`; and
  `supportedExtensions` did not list `.heic`/`.heif`, so the walk skipped them. On a
  real library, a folder of 13 HEIC went from 0 images seen to 13 scanned. This
  matters on a real camera roll — a sample library here holds 727 HEIC against 42 jpeg
  and 14 png
- Build failure on macOS with `CGO_ENABLED=0`: both Apple Vision files were excluded by
  their build tags, leaving `decodeWithVision` undefined. The stub is now selected by
  `!darwin || !cgo` and the cgo implementation by `darwin && cgo`

## [v1.4.1] — 2026-06-26

### Fixed
- Release build: macOS artifacts are built on a macOS runner with `CGO_ENABLED=1` so the Vision backend links; Linux/Windows stay pure Go and cross-compile

## [v1.4.0] — 2026-06-26

### Added
- macOS Vision QR backend — decodes real-world photos (perspective-warped, low-contrast, small-module receipt QRs) that the pure-Go path misses. Uses the framework shipped with macOS, so it adds no install step

## [v1.3.0] — 2026-06-23

### Added
- Live progress bar pinned at the top during folder scans, with the file being scanned and a done/total counter
- Color-channel decode strategies (R/G/B/min/max projections) that recover hard, low-contrast and colored QR codes
- Homebrew formula (`brew install thousandflowers/tap/qr-multi-imgs`)
- Prebuilt binaries for macOS, Linux, and Windows via GitHub Releases
- `codecov.yml` — Codecov coverage tracking
- Tests for pure functions in `main.go` (resolvePath, validateScanPath, etc.)
- `CONTRIBUTING.md` — contributor guide
- `CHANGELOG.md` — this file

### Changed
- Decoder returns the exact QR payload — no longer trims whitespace (fixes whitespace-only payloads)
- Faster scanning: dropped the ineffective 3x/4x upscale strategies
- CI now builds and tests on Windows too
- README: Codecov badge, `go install` instructions, explicit Windows support

### Fixed
- Crash (index out of range) when scanning a folder containing no supported images

## [v1.2.0] — 2026-06-19

### Added
- 50+ new tests across all modules — coverage from 55% → 90.5%
- Exporter test suite reaches 100% coverage
- CI workflow for automatic cross-platform releases via GoReleaser
- GoReleaser config for prebuilt binaries (macOS, Linux, Windows)

### Changed
- `npy_test.go` — full test coverage for numpy mask parser
- `main_test.go` — View, Update, action tests for all TUI pages
- `scanner_test.go` — ScanImage, ScanFolder edge cases (unreadable dir, corrupt files)
- README: massive simplification, single benchmark, no stale references

## [v1.1.0] — 2026-06-19

### Added
- `scanner/npy.go` — minimal `.npy` v1.0 parser for boolean arrays
- Mask-based QR detection fallback using `zbarimg` via stdin pipe
- Parallel worker pool (6 workers, ~20× speedup)
- JPG/SVG output for QR regeneration
- Format picker page (PNG/JPG/SVG) before recreating QR codes

### Performance
- 3332/3332 detection at ~142 img/s (M2 Pro)

## [v1.0.0] — 2026-06-09

### Added
- Go rewrite — pure Go QR scanner, no CGo, no Python
- Bubbletea TUI with folder input, drag & drop, clipboard
- Exporter test suite: JSON/CSV/TXT round trips, QR round trip through scanner
- GitHub Actions CI (build + vet + test on ubuntu & macOS)

### Fixed
- Unknown export format now returns an explicit error
- CSV `Flush()` before error check (buffered write failures were silently dropped)
- Use `filepath.Join` instead of `Sprintf` for output paths (Windows compat)

### Changed
- Versioning reset from Python 0.x to Go 1.0.0

## [v0.5.0] — 2026-04-16

### Changed
- Refactored TUI screens into `tui_screens.py` (main file reduced from 1669 to 1345 lines)

## [v0.4.2] — 2026-04-16

### Changed
- Replaced Textual TUI with simple text-based interactive menu for terminal compatibility

## [v0.4.1] — 2026-04-16

### Fixed
- TUI not launching, added fallback menu

## [v0.4.0] — 2026-04-16

### Added
- New wizard TUI with step-by-step flow

## [v0.3.4] — 2026-04-16

### Changed
- Simplified TUI: enter folder first, then select action

## [v0.3.3] — 2026-04-16

### Fixed
- TUI now launches immediately without requiring Enter key

## [v0.3.2] — 2026-04-16

### Fixed
- Homebrew formula `chmod` syntax

## [v0.3.1] — 2026-04-16

### Changed
- Updated LICENSE with new project name

## [v0.1.0] — 2026-06-09

### Added
- Initial Go release
- Cross-platform disk/clipboard support
- `--help` flag and README polish

> Earlier Python versions (v0.1.0–v0.5.0, v0.1.0–v0.3.4) are not listed in detail here.
> See the Git history for the full record.
