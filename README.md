# qr-multi-imgs

**Scan a folder → decode QR → organize, export, recreate - from a clean TUI.**

**Or read your codes with nothing installed: [open the web app](https://thousandflowers.github.io/qr-multi-imgs/)**
and drop a photo or a folder on it. The decoder is a WebAssembly build of this
same Go code running inside your tab - your images are never uploaded.

![demo](demo.gif)

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Go Report](https://goreportcard.com/badge/github.com/thousandflowers/qr-multi-imgs)](https://goreportcard.com/report/github.com/thousandflowers/qr-multi-imgs)
[![CI](https://github.com/thousandflowers/qr-multi-imgs/actions/workflows/ci.yml/badge.svg)](https://github.com/thousandflowers/qr-multi-imgs/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/thousandflowers/qr-multi-imgs/graph/badge.svg)](https://codecov.io/gh/thousandflowers/qr-multi-imgs)
![Zero Deps](https://img.shields.io/badge/system%20deps-zero-brightgreen)
![Speed](https://img.shields.io/badge/3332%20QR%20in%20~7s-M2%20Pro-orange)
![Windows](https://img.shields.io/badge/Windows-supported-0078D4?logo=windows)

```bash
# One command — go, or curl, or brew:
go install github.com/thousandflowers/qr-multi-imgs@latest
qr-multi-imgs ./folder-of-qr-images
```

---

## Why

My father needed to catalog hundreds of photos of orders - each one a QR code on a receipt. No CLI could take a whole folder, keep the results, and then do something with them.

qr-multi-imgs is that missing step: point it at a folder, and the decoded codes come out the other side as files you can act on - sorted, exported, or regenerated.

### Input - 5 ways

CLI argument · **drag & drop a folder onto the terminal** · type a path · paste (`Cmd+V`) · clipboard (`Ctrl+D`)

---

## Comparison

Plenty of CLI QR scanners exist. None ship a TUI built for batch work.

| | qr-multi-imgs | [qr-scanner-cli](https://github.com/victorperin/qr-scanner-cli) | [qrtool](https://github.com/sorairolake/qrtool) |
| --- | --- | --- | --- |
| Interactive TUI | ✅ | ❌ | ❌ |
| Organize files (with/without QR) | ✅ | ❌ | ❌ |
| Delete images without QR | ✅ | ❌ | ❌ |
| Recreate QR (PNG/JPG/SVG) | ✅ | ❌ | bitmap only |
| Export (JSON/CSV/TXT) | ✅ | ❌ | ✅ |
| Drag & drop + clipboard | ✅ | ❌ | ❌ |
| Photographed QRs (Apple Vision, macOS) | ✅ | ❌ | ❌ |
| Nothing to install (no runtime deps) | ✅ | ❌ (Node) | ✅ |

---

## Benchmark

Dataset: [lovasoa/qrcode-dataset](https://github.com/lovasoa/qrcode-dataset) - **3332 damaged & distorted QR images** built to stress-test scanners. Each sample ships three files: the distorted `NNN.png`, the decoded `NNN.txt`, and `NNN.npy` - the generator's **source bit-matrix**.

**The headline is the image-only number**, because it is the only one that
describes decoding:

| | |
|---|---|
| **Recall, image pixels only** | **58.4%** (1947 / 3332) |
| **Time** | 97 s |
| **Throughput** | 34 img/s |

Pure Go, no Apple Vision, no `zbarimg`, companion masks ignored. Reproduce with
`go test -tags corpus ./scanner -run TestCorpus` after building the manifest
with `corpusgen -truth .txt`; the harness ignores the masks by default and says
so in its output.

The other number this dataset can produce is **3332/3332 (100%) at ~475 img/s**,
and it is not a decoding result. It is the speed of reading the answer key the
dataset ships beside every image: when a `.npy` sits next to the file,
qr-multi-imgs decodes that bit-matrix first, in pure Go. That is exactly what
you want on this dataset and it never happens on your own photos, so it is not
the headline. Run it with `-corpus.masks` when you want to check that path has
not regressed.

**On pixels alone the dataset splits hard:**

| subset | what it holds | image-only |
|---|---|---|
| `with_qr` | clean, stylized codes | ~100% |
| `without_qr` | colored, dense, low-contrast adversarial codes | ~25.7% |

The ~70% of `without_qr` left unread is not low-hanging fruit. The classical
ceiling, measured across the union of every colour channel, binarizer and scale
we could throw at it, is ~28% with gozxing and ~30.3% with `zbarimg`. Passing it
needs an ML detector (WeChat/CNN), which means an OpenCV C dependency - the
thing this tool exists to avoid.

**Why the dense codes fail - measured, not assumed.** Not because the modules
are sub-pixel: a clean 153x153-module QR rendered at 256 px (1.67 px per module,
well under two) decodes fine straight from the pixels. What destroys it is the
dataset's *distortion*; density only decides how little of it is enough. Under a
box blur, 61 modules survives radius 2 and fails at 3, 97 and 125 fail at 2, and
153 fails already at 1. In every one of those failures the `.npy` still decodes.

**Known limits.** `zbarimg`, the last-resort fallback, **segfaults** on some
real photographs - exit 139 on a 3024x4032 iPhone JPEG here, with no output.
The scan survives it: the exit status is treated as "found nothing" and the
other decoders still run. It does mean `zbarimg` cannot be relied on as an
independent second opinion on exactly the files where a second opinion would be
worth most.

**Since v1.5.0 a scan is slower on purpose.** Earlier versions stopped at the
first code in an image; v1.5.0 returns every code in it. On 40 real photos that
moved 3.9s to 18.4s. The unbounded strategy union would have cost 122.6s - the
finder-pattern pre-pass is what keeps it at 18.

```bash
# Reproduce — the dataset is generated, not stored in the repo:
git clone https://github.com/lovasoa/qrcode-dataset
cd qrcode-dataset && pipenv install && pipenv run python generate_dataset.py
qr-multi-imgs ./dataset
```

Layout decides which number you measure: keep `png`, `npy` and `txt` co-located
and the companion path resolves (100%); separate the images into subfolders and
it does not, which measures the image-only path instead.

Measured on an Apple M2 Pro (10 cores, 16 GB).

---

## TUI actions

| Key | Action |
| --- | --- |
| `l` | List results |
| `e` | Export (JSON/CSV/TXT) |
| `d` | Delete images without a QR code |
| `o` | Organize into `with_qr/` and `without_qr/` |
| `r` | Recreate QR codes from decoded content (choose PNG/JPG/SVG) |
| `q` / `^C` | Quit |

---

## Web app - nothing to install

**<https://thousandflowers.github.io/qr-multi-imgs/>**

Drop a single photo, a folder, or several folders - most scanners take one
picture at a time, which is the point of this one. You can also paste a
screenshot, or read codes live through the camera.

Every QR code in every image comes back with a thumbnail, a copy button, and a
link when the payload is one. Filter to the images that had a code, or the ones
that did not, and download the lot as JSON, CSV or plain text.

**Nothing is uploaded, and that is enforced rather than promised.** The page
carries a `Content-Security-Policy` whose `connect-src` allows only its own
origin and the loopback, so it *cannot* send a filename or a payload anywhere
else even if the page were tampered with. It loads no font, script or image from
any third party. Your files never leave the tab.

How the work is split matters. The browser turns each file into pixels - it
already ships native decoders for everything it can display, JPEG through AVIF
and HEIC on Safari - and a WebAssembly build of the same Go decoder does the
part a browser has no answer for: finding and reading the codes. That is why the
engine is 4.6 MB (1.4 MB over the wire) instead of carrying image decoders it
would only duplicate. Decoding runs in a pool of Web Workers, one per core up to
three, so a folder of photos does not freeze the page.

What the browser cannot do is touch your disk. It reads the files you hand it
and nothing else - no moving, no deleting. For that, run the program.

## Driving the local program from a browser

```bash
qr-multi-imgs --serve
```

It prints two links. Both open the same page - `web/` is embedded in the binary
and published to Pages, so there is one file, not two - but with the program
running the page gains what a browser is not allowed to do on its own: organize
into `with_qr/` and `without_qr/`, recreate the codes as images, delete the ones
without a code, all where the files actually live.

| | | needs |
|---|---|---|
| **Local UI** | `http://127.0.0.1:8787/#t=...` | nothing; works in every browser |
| **Hosted UI** | `thousandflowers.github.io/qr-multi-imgs/#t=...&api=...` | one permission, below |

A page served over `https://` reaching `http://127.0.0.1` clears the
mixed-content rules - the loopback counts as a trustworthy origin - but Chrome
additionally gates it behind a **Local Network Access permission**. Measured on
Chrome 151: `navigator.permissions.query({name: "local-network-access"})` reports
`prompt`, so the browser asks and the choice is yours. Refuse it, or use a
browser that declines outright, and the Local UI link still works with no
permission at all.

### Why the local API cannot be turned against you

It deletes files, so it rests on three properties, each enforced in code:

1. **Loopback only.** `--serve=0.0.0.0:8787` is refused, not honoured.
2. **A random token in a request header.** A header forces a CORS preflight, so
   a hostile page cannot fire a request and ignore the answer. The token travels
   in the URL *fragment*, which browsers never put in a request, a `Referer` or
   a server log. Origins outside the allowlist are rejected before the token is
   even read.
3. **Destructive calls name a session, never a path.** `organize`, `delete` and
   `recreate` take the id of a scan *this* server performed. The worst a stolen
   token can do is repeat an action on files you already chose to scan - there
   is no request shape that says "delete this arbitrary path".

---

## Install

### macOS - Homebrew

```bash
brew install thousandflowers/tap/qr-multi-imgs
```

The tap formula is bumped after a release with `scripts/bump_tap.py qr-multi-imgs`,
which uses whatever auth `gh` already has. It deliberately does not run inside CI:
publishing the formula from there needs a long-lived cross-repo token, and that
token fails at the *end* of a release - after the GitHub release exists - which
leaves `brew` a version behind while the release looks fine.

### Any platform - Go install

```bash
go install github.com/thousandflowers/qr-multi-imgs@latest
```

### One-liner (curl → bash)

```bash
bash <(curl -sL https://raw.githubusercontent.com/thousandflowers/qr-multi-imgs/main/install.sh)
```

### Prebuilt binaries

Download from [GitHub Releases](https://github.com/thousandflowers/qr-multi-imgs/releases) - macOS (Intel + Apple Silicon), Linux (amd64 + arm64), Windows (amd64).

### From source

```bash
git clone https://github.com/thousandflowers/qr-multi-imgs.git
cd qr-multi-imgs && go build
```

**Requirements:** Go 1.26+, an ANSI terminal.  
**Build:** pure Go on Linux/Windows. The macOS build links Apple's Vision
framework via cgo (ships with the OS - nothing to install) for photographed,
perspective-warped QR codes.  
**Supported formats:** PNG / JPEG / GIF / BMP / WebP / TIFF everywhere. On macOS the list is whatever the system itself can decode - asked at startup via ImageIO rather than hardcoded - which adds HEIC / HEIF, PSD, JXL, AVIF and camera raw from any camera the OS supports (CR2, CR3, ARW, DNG, NEF, RAF, ORF, RW2 and the rest); 70 formats on a current macOS. PDFs are read too, by rendering each page - a multi-page document reports the codes from every page. Elsewhere HEIC needs a `-tags heic` build and raw is unavailable. A folder holding any mix of these scans in a single pass, and several folders or single files can be scanned together.  
**Supported platforms:** macOS, Linux, **Windows** (clipboard via external tool).

---

## Contributing

PRs welcome. [CONTRIBUTING.md](CONTRIBUTING.md) has the build and test
loop; [GOOD_FIRST_ISSUES.md](GOOD_FIRST_ISSUES.md) has three tasks sized
for a first contribution. By taking part you agree to the
[Code of Conduct](CODE_OF_CONDUCT.md).

---

## License

MIT - see [LICENSE](LICENSE).
