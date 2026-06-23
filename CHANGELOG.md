# Changelog

All notable changes to qr-multi-imgs are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
