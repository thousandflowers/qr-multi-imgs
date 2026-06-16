# qr-multi-imgs

**Scan a folder → decode QR → organize, export, recreate — from a clean TUI.**

![demo](demo.gif)

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Go Report](https://goreportcard.com/badge/github.com/thousandflowers/qr-multi-imgs)](https://goreportcard.com/report/github.com/thousandflowers/qr-multi-imgs)
[![CI](https://github.com/thousandflowers/qr-multi-imgs/actions/workflows/ci.yml/badge.svg)](https://github.com/thousandflowers/qr-multi-imgs/actions/workflows/ci.yml)
![Zero Deps](https://img.shields.io/badge/system%20deps-zero-brightgreen)
![Speed](https://img.shields.io/badge/~1300%20imgs%2Fsec-M2%20Pro-orange)

```bash
bash <(curl -sL https://raw.githubusercontent.com/thousandflowers/qr-multi-imgs/main/install.sh)
qr-multi-imgs ~/Desktop/qr-test
```

**~1300 images/second** on an M2 Pro. Single Go binary. Zero system dependencies.

---

## Why

Plenty of CLI QR scanners exist, but none ship a full TUI built for batch work:

| | qr-multi-imgs | qr-scanner-cli | qrtool |
| --- | --- | --- | --- |
| Interactive TUI | ✅ | ❌ | ❌ |
| Organize files (with/without QR) | ✅ | ❌ | ❌ |
| Delete images without QR | ✅ | ❌ | ❌ |
| Recreate QR from content | ✅ | ❌ | ✅ |
| Export (JSON/CSV/TXT) | ✅ | ❌ | ✅ |
| Drag & drop + clipboard | ✅ | ❌ | ❌ |
| Pure Go, zero CGo | ✅ | ❌ (npm) | ✅ |

---

## TUI actions

| Key | Action |
| --- | --- |
| `l` | List results |
| `e` | Export (JSON/CSV/TXT) |
| `d` | Delete images without a QR code |
| `o` | Organize into `with_qr/` and `without_qr/` |
| `r` | Recreate 512×512 QR codes from decoded content |
| `q` / `^C` | Quit |

### 5 input methods

CLI argument, type a path, drag & drop, paste (`Cmd+V`), clipboard (`Ctrl+D`).

---

## Install

```bash
# One-liner
bash <(curl -sL https://raw.githubusercontent.com/thousandflowers/qr-multi-imgs/main/install.sh)

# From source
git clone https://github.com/thousandflowers/qr-multi-imgs.git
cd qr-multi-imgs && go build
```

**Requirements:** Go 1.26+, an ANSI terminal. Supports PNG / JPEG / GIF / BMP / WebP.

---

## License

MIT — see [LICENSE](LICENSE).
