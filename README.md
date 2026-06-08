# qr-multi-imgs

**Scan a folder of images. Detect QR codes. Organize, export, recreate — all from a clean TUI.**

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/thousandflowers/qr-multi-imgs)](https://goreportcard.com/report/github.com/thousandflowers/qr-multi-imgs)
[![CI](https://github.com/thousandflowers/qr-multi-imgs/actions/workflows/ci.yml/badge.svg)](https://github.com/thousandflowers/qr-multi-imgs/actions/workflows/ci.yml)

```text ── one install ──
bash <(curl -sL https://raw.githubusercontent.com/thousandflowers/qr-multi-imgs/main/install.sh)
```

Then:

```text
qr-multi-imgs ~/Desktop/qr-test-imgs
```

---

## What it does

Point it at a folder of images. It finds every QR code, shows you what's inside, and lets you act on them.

### Actions

| Key | Action | What happens |
|-----|--------|-------------|
| `l` | **List results** | See every file with its decoded QR content |
| `e` | **Export** | Save results as JSON / CSV / TXT |
| `d` | **Delete** | Remove images that have no QR code |
| `o` | **Organize** | Sort files into `with_qr/` and `without_qr/` folders |
| `r` | **Recreate** | Generate fresh 512×512 PNG QR images from decoded content |
| `q` / `Ctrl+C` | **Quit** | Exit |

### Input methods

| How | What to do |
|-----|-----------|
| **CLI argument** | `qr-multi-imgs ./images` — skips the folder prompt |
| **Type it** | Write the path in the input field |
| **Drag & drop** | Drag a folder from Finder into the terminal |
| **Paste** | `Cmd+V` a copied path |
| **Clipboard shortcut** | `Ctrl+D` — reads current clipboard as path |
| **Empty Enter** | Uses current directory |

---

## Install

### One-liner (any machine with Go)

```bash
bash <(curl -sL https://raw.githubusercontent.com/thousandflowers/qr-multi-imgs/main/install.sh)
```

### From source

```bash
git clone https://github.com/thousandflowers/qr-multi-imgs.git
cd qr-multi-imgs
go build -o qr-multi-imgs .
# optional: mv qr-multi-imgs ~/.local/bin/
```

### Requirements

- **Go 1.26+** ([download](https://go.dev/dl/))
- A terminal that supports ANSI (modern Terminal.app, iTerm2, Kitty, etc.)

No external libraries or system dependencies — everything is a Go module.

---

## Why Go?

The original Python version worked but required `zbar`, OpenCV, and careful Python env management. The Go version is:

- **Single binary** — no runtime, no pip, no virtualenvs
- **Zero system deps** — no `brew install zbar`, no `apt install libzbar0`
- **Pure Go QR scanning** via [gozxing](https://github.com/makiuchi-d/gozxing) — no CGo, no FFI
- **6 MB** on disk — one file, everything included

---

## Demo

```text
┌─────────────────────────────────────────────┐
│                                             │
│   ╔═══ qr-multi-imgs                       ║
│   ║                                         ║
│   ║  Folder path: [/path/to/images]         ║
│   ║  (Ctrl+D to paste clipboard)            ║
│   ║                                         ║
│   ╠═══ Results ─────────────────────────── ═╣
│   ║                                         ║
│   ║  ✓ github_url.png → https://github...  ║
│   ║  ✓ wifi_config.png → WIFI:T:WPA...     ║
│   ║  ✓ vcard_contact.png → BEGIN:VCARD...  ║
│   ║  ✗ no_qr_here.png → (no QR found)      ║
│   ║                                         ║
│   ║  [l] list  [e] export  [d] delete       ║
│   ║  [o] organize  [r] recreate  [q] quit   ║
│   ╚═════════════════════════════════════════╝
│                                             │
└─────────────────────────────────────────────┘
```

---

## Error handling

The TUI is built to survive real use:

- **Panic recovery** — every action wraps in a safe handler. If something crashes, you see the error + stack trace, not a terminal reset.
- **Path validation** — checks that the path exists, is a directory, and is readable before scanning.
- **Write protection** — destructive actions check write permissions first.
- **Disk space check** — warns before generating many QR images.
- **Graceful exit** — `Ctrl+C` or `q` shuts down cleanly, restoring your terminal.

---

## Project structure

```
qr-multi-imgs/
├── main.go                  # TUI application + entry point
├── scanner/
│   ├── scanner.go           # QR detection logic (pure Go)
│   └── scanner_test.go      # Tests with generated QR codes
├── exporter/
│   └── exporter.go          # Export (JSON/CSV/TXT) + QR generation
├── install.sh               # One-command install script
├── .github/workflows/ci.yml # CI: build + vet + test
├── go.mod / go.sum
└── README.md
```

---

## In action

```bash
# Scan a folder
qr-multi-imgs ~/Desktop/photos-with-qrs

# Pass argument and pipe results
qr-multi-imgs /tmp/qr-test-set

# Export as JSON for scripting
# (open the export menu with 'e' inside the TUI)
```

---

## License

MIT — see [LICENSE](LICENSE)

## Credits

Built by [thousandflowers](https://github.com/thousandflowers).  
QR detection powered by [gozxing](https://github.com/makiuchi-d/gozxing).  
TUI powered by [bubbletea](https://github.com/charmbracelet/bubbletea).  
AI-assisted development with [Claude Code](https://docs.anthropic.com/en/docs/claude-code/overview).
