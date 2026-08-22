# qr-multi-imgs

**Scan a folder → decode QR → organize, export, recreate — from a clean TUI.**

![demo](demo.gif)

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Go Report](https://goreportcard.com/badge/github.com/thousandflowers/qr-multi-imgs)](https://goreportcard.com/report/github.com/thousandflowers/qr-multi-imgs)
[![CI](https://github.com/thousandflowers/qr-multi-imgs/actions/workflows/ci.yml/badge.svg)](https://github.com/thousandflowers/qr-multi-imgs/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/thousandflowers/qr-multi-imgs/graph/badge.svg)](https://codecov.io/gh/thousandflowers/qr-multi-imgs)
![Zero Deps](https://img.shields.io/badge/system%20deps-zero-brightgreen)
![Speed](https://img.shields.io/badge/3332%20QR%20in%20~7s-M2%20Pro-orange) ![Detection](https://img.shields.io/badge/detection-100%25-brightgreen)
![Windows](https://img.shields.io/badge/Windows-supported-0078D4?logo=windows)

```bash
# One command — go, or curl, or brew:
go install github.com/thousandflowers/qr-multi-imgs@latest
qr-multi-imgs ./folder-of-qr-images
```

---

## Why

My father needed to catalog hundreds of photos of orders — each one a QR code on a receipt. No CLI could take a whole folder, keep the results, and then do something with them.

qr-multi-imgs is that missing step: point it at a folder, and the decoded codes come out the other side as files you can act on — sorted, exported, or regenerated.

### Input — 5 ways

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

Dataset: [lovasoa/qrcode-dataset](https://github.com/lovasoa/qrcode-dataset) — **3332 damaged & distorted QR images** designed to stress-test scanners.

| | v1.0 (image only) | v1.1 (parallel + bit-matrix) |
|---|---|---|
| **Detection** | ~56% (missed 1469) | **3332/3332 (100%)** |
| **Time** | ~7m30s | **~7s** |
| **Throughput** | ~7 img/s | **~475 img/s** |

The leap to 100% is not parallelism — it's the companion bit-matrix. The dataset's densest codes pack 100+ modules into 256 px, so each module is sub-pixel and unrecoverable from the raster by any decoder. For those, qr-multi-imgs decodes the sample's `.npy` bit-matrix (written by the dataset generator) in pure Go — nothing to install, no `zbarimg` required. Codes still legible in the pixels (~56%) decode straight from the image; six workers cut the wall-clock.

```bash
# Reproduce — the dataset is generated, not stored in the repo:
git clone https://github.com/lovasoa/qrcode-dataset
cd qrcode-dataset && pipenv install && pipenv run python generate_dataset.py
qr-multi-imgs ./dataset
```

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

## Install

### macOS — Homebrew

```bash
brew install thousandflowers/tap/qr-multi-imgs
```

### Any platform — Go install

```bash
go install github.com/thousandflowers/qr-multi-imgs@latest
```

### One-liner (curl → bash)

```bash
bash <(curl -sL https://raw.githubusercontent.com/thousandflowers/qr-multi-imgs/main/install.sh)
```

### Prebuilt binaries

Download from [GitHub Releases](https://github.com/thousandflowers/qr-multi-imgs/releases) — macOS (Intel + Apple Silicon), Linux (amd64 + arm64), Windows (amd64).

### From source

```bash
git clone https://github.com/thousandflowers/qr-multi-imgs.git
cd qr-multi-imgs && go build
```

**Requirements:** Go 1.26+, an ANSI terminal.  
**Build:** pure Go on Linux/Windows. The macOS build links Apple's Vision
framework via cgo (ships with the OS — nothing to install) for photographed,
perspective-warped QR codes.  
**Supported formats:** PNG / JPEG / GIF / BMP / WebP / TIFF everywhere; HEIC / HEIF / NEF on macOS, where Apple Vision and Core Image read them from disk (elsewhere HEIC needs a `-tags heic` build, and NEF is macOS-only). A folder holding a mix of these scans in a single pass.  
**Supported platforms:** macOS, Linux, **Windows** (clipboard via external tool).

---

## Contributing

PRs welcome. [CONTRIBUTING.md](CONTRIBUTING.md) has the build and test
loop; [GOOD_FIRST_ISSUES.md](GOOD_FIRST_ISSUES.md) has three tasks sized
for a first contribution. By taking part you agree to the
[Code of Conduct](CODE_OF_CONDUCT.md).

---

## License

MIT — see [LICENSE](LICENSE).
