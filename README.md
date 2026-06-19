# qr-multi-imgs

**Scan a folder → decode QR → organize, export, recreate — from a clean TUI.**

![demo](demo.gif)

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Go Report](https://goreportcard.com/badge/github.com/thousandflowers/qr-multi-imgs)](https://goreportcard.com/report/github.com/thousandflowers/qr-multi-imgs)
[![CI](https://github.com/thousandflowers/qr-multi-imgs/actions/workflows/ci.yml/badge.svg)](https://github.com/thousandflowers/qr-multi-imgs/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/thousandflowers/qr-multi-imgs/graph/badge.svg)](https://codecov.io/gh/thousandflowers/qr-multi-imgs)
![Zero Deps](https://img.shields.io/badge/system%20deps-zero-brightgreen)
![Speed](https://img.shields.io/badge/3332%20QR%20in%20~23s-M2%20Pro-orange) ![Detection](https://img.shields.io/badge/detection-100%25-brightgreen)
![Windows](https://img.shields.io/badge/Windows-supported-0078D4?logo=windows)

```bash
# One command — go, or curl, or brew:
go install github.com/thousandflowers/qr-multi-imgs@latest
qr-multi-imgs ./folder-of-qr-images
```

---

## Why

My father needed to catalog hundreds of photos of orders — each one a QR code on a receipt. No existing CLI tool could handle a folder, organize results, or do anything useful with the output.

qr-multi-imgs started as a fix for that. It kept growing.

Plenty of CLI QR scanners exist, but none ship a full TUI built for batch work:

| | qr-multi-imgs | qr-scanner-cli | qrtool |
| --- | --- | --- | --- |
| Interactive TUI | ✅ | ❌ | ❌ |
| Organize files (with/without QR) | ✅ | ❌ | ❌ |
| Delete images without QR | ✅ | ❌ | ❌ |
| Recreate QR (PNG/JPG/SVG) | ✅ | ❌ | bitmap only |
| Export (JSON/CSV/TXT) | ✅ | ❌ | ✅ |
| Drag & drop + clipboard | ✅ | ❌ | ❌ |
| Pure Go, zero CGo | ✅ | ❌ (npm) | ✅ |

### Input — 5 ways

CLI argument · **drag & drop a folder onto the terminal** · type a path · paste (`Cmd+V`) · clipboard (`Ctrl+D`)

---

## Benchmark

Dataset: [lovasoa/qrcode-dataset](https://github.com/lovasoa/qrcode-dataset) — **3332 damaged & distorted QR images** designed to stress-test scanners.

| | v1.0 (sequential) | v1.1 (parallel, 6 workers) |
|---|---|---|
| **Detection** | ~56% (missed 1469) | **3332/3332 (100%)** |
| **Time** | ~7m30s | **~23s** |
| **Throughput** | ~7 img/s | **~142 img/s** |

Parallel mode uses the same decoding pipeline as sequential — accuracy is identical, only throughput differs.

```bash
# Reproduce:
git clone https://github.com/lovasoa/qrcode-dataset
qr-multi-imgs ./qrcode-dataset
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
**Supported formats:** PNG / JPEG / GIF / BMP / WebP.  
**Supported platforms:** macOS, Linux, **Windows** (clipboard via external tool).

---

## License

MIT — see [LICENSE](LICENSE).
