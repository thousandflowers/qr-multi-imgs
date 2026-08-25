# Changelog

All notable changes to qr-multi-imgs are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed
- Clicking a statistic to filter threw the panel around. Three separate faults, all
  reproduced before being touched: the pressed state was derived from the filter
  *value*, so `with a code` and `codes` lit up together since they mean the same
  thing; clicking `codes` right after `with a code` compared values and so switched
  the filter off instead of keeping it; and a filter matching nothing raised the
  big empty state, which sits above the statistics and shoved the whole panel down.
  The active *tile* is remembered now rather than the value, and an empty filter says
  so from inside the list, where the missing rows would have been

### Changed
- **The decoder is compiled once and shared with every worker.** Each worker used to
  fetch `qr.wasm` for itself, so six workers meant six downloads of the same 4.6 MB
  file. Measured: one fetch instead of six, and a local cold start of 190 ms against
  322 ms. Over a network the difference is six round trips against one, which was most
  of what made the page feel slow to wake up. Frame timing during a scan was already
  clean and stayed so: 588 frames over eighteen 12-megapixel photos, worst 17 ms,
  none above 50
- **The progress bar names what it is working through**, not just a count: `rich 2 / 5`
  while a folder runs, `archivio 0 / 5` when a zip follows it. Each drop is its own
  batch, so a second folder does not silently join the first one's total
- Plainer copy, in all six languages, and no em dashes anywhere on the page. The
  sentences were built the same way every time: a clause bolted on after a dash

### Added
- **Six languages** - English, Italian, Spanish, French, German, Portuguese -
  chosen from the browser's own order of preference, with a menu in the header.
  The words live in `web/strings.js` as data, so adding a language is one block and
  nothing else; the self-test fails if any language is short a key or drops a `{n}`
  placeholder, which is how a sentence ends up with a hole in it
- **A dropped zip is unpacked and read.** No library: `DecompressionStream` handles
  deflate and the rest is three fixed-layout records. The central directory is the
  authority rather than the local headers, which a streaming writer may leave with
  zero sizes. Inner paths are kept, the `./` a `zip -r` leaves is stripped, and
  macOS's `__MACOSX` resource-fork tree is skipped
- **Every file gets a row the moment it is queued**, with a spinner and a shimmering
  placeholder, and gains a green tick where it already sits when its answer arrives.
  The list is now the list of what was loaded, not only of what has finished, and the
  drop zone acknowledges the drop
- **The statistics are the filters.** Clicking `with a code`, `without`, `unreadable`
  or `unique` filters to it and clicking again returns to everything. `unique` keeps
  the first image that found each payload, so the view does not shuffle as later
  copies arrive

### Changed
- **Three times faster on photos.** Measured on a 3024x4032 photo: a 3000 px cap took
  3826 ms, a 1600 px cap took 1287 ms and found the same code. The first pass is now
  1600 px, with one retry at full size only when nothing was found *and* the image was
  actually shrunk - failure is the expensive case, since with no candidate count the
  decoder runs every strategy. The worker pool went from 3 to 6. A five-image folder
  went 4.5 s to 1.3 s
- Clear moved into the progress bar, in red, and asks first through a small popover
  beside it rather than a system dialog that stops the page
- "Advanced" is gone. That section is not an advanced feature, it is a different way
  in for people who prefer the terminal, and it now says exactly that

### Added
- **What a code means, not the string it contains.** Every payload is run past a
  table of recognisers and comes back as a kind with named fields: a link shows its
  site and the reference inside it, a Wi-Fi code shows network and security, a SEPA
  payment shows payee, IBAN and amount, a contact shows a name a person would say
  out loud rather than `Rossi;Mario`. Adding a format means adding a row, not a
  branch
- **Secrets stay covered.** A two-factor setup code (`otpauth://`) carries the seed
  that generates every future code for that account, and a Wi-Fi payload carries the
  password. Both are marked secret, rendered as dots until deliberately revealed, and
  a seed can never end up in a file name - a scan is often done with other people
  looking at the screen
- **Rename and download as one archive.** New names are built from the meaningful
  part of each code - for a link that is the reference, not the campaign path - with
  collisions numbered rather than dropped. The ZIP is written by hand, store-only,
  because the images are already compressed and the page's CSP forbids fetching a
  library. The image-to-code manifest travels *inside* the archive, since that
  mapping is the valuable part of a batch scan
- **Search inside the decoded content**, across payloads and file names together,
  alongside the with/without/unreadable filters and a new "only duplicates" one
- **Duplicates are named.** A payload found in several images says so on every one of
  them, and the statistics carry a "unique" figure beside the total
- **Italian**, picked from the browser's own preference with a switch in the header.
  Both languages are complete and a self-test fails if a key is missing from either
- **Camera and paste** as first-class inputs, plus a "take a photo" button that opens
  the camera directly on a phone
- Unreadable and code-less images now carry advice a non-technical person can act on
  rather than only a count

### Changed
- Developer-facing machinery - the command line, the local API, acting on files on
  disk - moved behind an "Advanced" disclosure. It is the right tool for scripted and
  batch use and it is documented as such, but it is noise for someone who came to
  read one receipt
- CSV export carries the full path, the payload type, how many images share the code,
  and a first-seen timestamp; JSON export is structured per code rather than per image

- The web app is laid out like the utility sites people already know: a fixed
  header, a headline that says what it does, and a two-column tool with the input
  on the left and a results panel that is present from the start rather than
  appearing after the first scan. Statistics read as a strip of figures, each code
  sits in its own field with a copy button beside it
- **Paste**: press paste anywhere on the page, or use the button. This is what
  people reach for after taking a screenshot of a code, and every scanner worth
  using supports it
- **Camera**: read codes live through the webcam, as a tab beside the file input.
  Frames go through the same engine, one in flight at a time so a backlog of
  moments the user has moved past cannot build up, and repeated payloads are
  reported once rather than every frame
- A web app that needs nothing installed:
  <https://thousandflowers.github.io/qr-multi-imgs/>. Drop a single photo, a
  folder, or several folders; every code comes back with a thumbnail, a copy
  button and a link where the payload is a URL, filterable and downloadable as
  JSON/CSV/TXT. Folder drops are walked recursively, and `readEntries` is looped
  to exhaustion rather than called once, which would have silently truncated any
  folder past about a hundred files
- The decoder is a WebAssembly build of the same Go code (`wasm/`), 4.6 MB and
  about 1.4 MB over the wire, running in a pool of Web Workers - one per core up
  to three - so a folder of photos does not freeze the page. It is built by the
  Pages workflow rather than committed, and CI compiles the `js/wasm` target on
  every run so it cannot rot unnoticed
- The split of labour is deliberate: the browser turns files into pixels, since
  it already ships native decoders for everything it can display (JPEG through
  AVIF, and HEIC on Safari), and Go does the part a browser has no answer for.
  That is what `scanner.ScanDecodedImage` was added for, and it means no image
  decoders are duplicated into the wasm binary
- The page's `connect-src` allows only its own origin and the loopback, so it
  cannot send a filename or a payload to any third party even if tampered with

### Changed
- Releasing no longer depends on a secret that does not exist. goreleaser's
  `brews:` block needed a long-lived token with push rights on the tap repo, stored
  as `GH_PAT` - and that secret has never been set here, so the block had never once
  run. Worse, the formula push happens at the *end* of goreleaser, so a release would
  publish and only then fail the bump, leaving `brew` on the old version behind a
  green-looking release. The block is gone; `scripts/bump_tap.py` does the same job
  with whatever auth `gh` already has, and the release job prints the exact command
  in its summary
- `scripts/bump_tap.py` reads the formula rather than knowing about any project: the
  source repo comes from `homepage`, the current version from `version` or from the
  tag inside a url, and each sha256 is recomputed from the file the url above it
  actually points at - so a per-architecture formula cannot end up carrying one
  arch's checksum on another's download, which is exactly the bug that shipped in
  the skillreaper formula once. It refuses rather than guesses when the number of
  sha256 lines and urls disagree. `--self-test` covers both, `--dry-run` prints the
  formula it would write

## [v1.6.0] - 2026-08-22

### Added
- `qr-multi-imgs --serve` runs a local HTTP API and serves a web UI with the same
  actions as the TUI: scan several paths, export, recreate, organize, delete. Progress
  streams as newline-delimited JSON rather than Server-Sent Events, because
  `EventSource` cannot set a request header and the token lives in one
- The same `web/` directory is embedded in the binary and published to GitHub Pages,
  so the hosted page and the one `--serve` hands you are the same file. The hosted
  page drives the local process through `#api=`; nothing is uploaded either way
- The page ships a `Content-Security-Policy` whose `connect-src` allows only its own
  origin and the loopback, so it cannot send a path, filename or payload to any third
  party even if tampered with, and it loads no external font, script or image

### Security
- The API listens on 127.0.0.1 and refuses any other bind address rather than
  honouring it. Every call carries a random 32-character token in a header, which
  forces a CORS preflight; the token rides in the URL fragment, which browsers never
  place in a request, a `Referer` or a log. Origins outside the allowlist are rejected
  before the token is read
- Destructive endpoints name a session, never a path: `organize`, `delete` and
  `recreate` act on the results of a scan this server performed, so a leaked token
  cannot be pointed at an arbitrary directory
- Measured, not assumed: Chrome 151 gates a public page's access to the loopback
  behind a Local Network Access permission that reports `prompt`. The page waits for a
  click before connecting, since a permission prompt only appears reliably when a
  gesture asked for it

### Changed
- The TUI's delete, organize and recreate actions were `tea.Cmd` closures wrapping
  logic nothing else could call. The logic now lives in plain functions returning an
  `actionOutcome`, and the TUI commands are one-line wrappers over them, so the HTTP
  API runs the same code rather than a second copy of it

## [v1.5.0] - never released

Tagged nowhere: the release workflow at the commit this section describes still
required a tap token that does not exist, and GitHub Actions runs the workflow
file from the tagged commit rather than from the default branch. Everything
below shipped in v1.6.0 instead.


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
  NOTICE, so no released artifact contains the codec. Verified - a goreleaser snapshot
  holds no occurrence of wazero, gen2brain, libheif or de265. Tagged binary costs
  11.56 MB against 7.21 MB. Tracking gen2brain/heic#17
- `cmd/corpusgen` bootstraps a `corpus.csv` manifest from unlabelled images
- Homebrew formula is published to `thousandflowers/homebrew-tap` automatically on
  every release (goreleaser `brews:`), so `brew` users stop drifting behind
- `scanner.ScanDecodedImage(image.Image) ([]string, error)` - decodes from pixels with
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
  `CGImageSourceCopyTypeIdentifiers` supplies it - 70 formats on a current release,
  including every camera raw the OS knows (cr2, cr3, arw, dng, raf, orf, rw2, nef and
  the rest) plus psd, jxl, avif, exr, hdr, dds, ico, icns, pict, jp2. The old list was
  already wrong for anyone shooting Canon, Sony or Fuji and went stale with each new
  camera. Elsewhere the built-in set applies
- TIFF everywhere, via a package already in `go.mod`
- PDFs are read, every page. Rendered through Core Graphics onto white - a QR is black
  vector art over nothing, so an uninitialised bitmap gives black on black - honouring
  `/Rotate` and the crop box via `CGPDFPageGetDrawingTransform`
- Ground-truth decode benchmark behind the `corpus` build tag:
  `go test -tags corpus ./scanner -run TestCorpus -v`. Reads a `corpus.csv` manifest,
  reports total/correct/wrong/missed/errors and a success rate. `QR_CORPUS_DIR` points
  it at an uncommitted private photo set

### Changed
- The raster strategy loop stops on finder-pattern evidence. Decoding every code meant
  unioning all ten strategies, which cost 31.8x on real photos holding a code (40 of
  them: 3.9s → 122.6s, 10.4 img/s → 0.3). A pre-pass counts the QR candidates visible
  across every colour projection and the loop stops once it has decoded that many -
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
  matters on a real camera roll - a sample library here holds 727 HEIC against 42 jpeg
  and 14 png
- Build failure on macOS with `CGO_ENABLED=0`: both Apple Vision files were excluded by
  their build tags, leaving `decodeWithVision` undefined. The stub is now selected by
  `!darwin || !cgo` and the cgo implementation by `darwin && cgo`

## [v1.4.1] - 2026-06-26

### Fixed
- Release build: macOS artifacts are built on a macOS runner with `CGO_ENABLED=1` so the Vision backend links; Linux/Windows stay pure Go and cross-compile

## [v1.4.0] - 2026-06-26

### Added
- macOS Vision QR backend - decodes real-world photos (perspective-warped, low-contrast, small-module receipt QRs) that the pure-Go path misses. Uses the framework shipped with macOS, so it adds no install step

## [v1.3.0] - 2026-06-23

### Added
- Live progress bar pinned at the top during folder scans, with the file being scanned and a done/total counter
- Color-channel decode strategies (R/G/B/min/max projections) that recover hard, low-contrast and colored QR codes
- Homebrew formula (`brew install thousandflowers/tap/qr-multi-imgs`)
- Prebuilt binaries for macOS, Linux, and Windows via GitHub Releases
- `codecov.yml` - Codecov coverage tracking
- Tests for pure functions in `main.go` (resolvePath, validateScanPath, etc.)
- `CONTRIBUTING.md` - contributor guide
- `CHANGELOG.md` - this file

### Changed
- Decoder returns the exact QR payload - no longer trims whitespace (fixes whitespace-only payloads)
- Faster scanning: dropped the ineffective 3x/4x upscale strategies
- CI now builds and tests on Windows too
- README: Codecov badge, `go install` instructions, explicit Windows support

### Fixed
- Crash (index out of range) when scanning a folder containing no supported images

## [v1.2.0] - 2026-06-19

### Added
- 50+ new tests across all modules - coverage from 55% → 90.5%
- Exporter test suite reaches 100% coverage
- CI workflow for automatic cross-platform releases via GoReleaser
- GoReleaser config for prebuilt binaries (macOS, Linux, Windows)

### Changed
- `npy_test.go` - full test coverage for numpy mask parser
- `main_test.go` - View, Update, action tests for all TUI pages
- `scanner_test.go` - ScanImage, ScanFolder edge cases (unreadable dir, corrupt files)
- README: massive simplification, single benchmark, no stale references

## [v1.1.0] - 2026-06-19

### Added
- `scanner/npy.go` - minimal `.npy` v1.0 parser for boolean arrays
- Mask-based QR detection fallback using `zbarimg` via stdin pipe
- Parallel worker pool (6 workers, ~20× speedup)
- JPG/SVG output for QR regeneration
- Format picker page (PNG/JPG/SVG) before recreating QR codes

### Performance
- 3332/3332 detection at ~142 img/s (M2 Pro)

## [v1.0.0] - 2026-06-09

### Added
- Go rewrite - pure Go QR scanner, no CGo, no Python
- Bubbletea TUI with folder input, drag & drop, clipboard
- Exporter test suite: JSON/CSV/TXT round trips, QR round trip through scanner
- GitHub Actions CI (build + vet + test on ubuntu & macOS)

### Fixed
- Unknown export format now returns an explicit error
- CSV `Flush()` before error check (buffered write failures were silently dropped)
- Use `filepath.Join` instead of `Sprintf` for output paths (Windows compat)

### Changed
- Versioning reset from Python 0.x to Go 1.0.0

## [v0.5.0] - 2026-04-16

### Changed
- Refactored TUI screens into `tui_screens.py` (main file reduced from 1669 to 1345 lines)

## [v0.4.2] - 2026-04-16

### Changed
- Replaced Textual TUI with simple text-based interactive menu for terminal compatibility

## [v0.4.1] - 2026-04-16

### Fixed
- TUI not launching, added fallback menu

## [v0.4.0] - 2026-04-16

### Added
- New wizard TUI with step-by-step flow

## [v0.3.4] - 2026-04-16

### Changed
- Simplified TUI: enter folder first, then select action

## [v0.3.3] - 2026-04-16

### Fixed
- TUI now launches immediately without requiring Enter key

## [v0.3.2] - 2026-04-16

### Fixed
- Homebrew formula `chmod` syntax

## [v0.3.1] - 2026-04-16

### Changed
- Updated LICENSE with new project name

## [v0.1.0] - 2026-06-09

### Added
- Initial Go release
- Cross-platform disk/clipboard support
- `--help` flag and README polish

> Earlier Python versions (v0.1.0–v0.5.0, v0.1.0–v0.3.4) are not listed in detail here.
> See the Git history for the full record.
