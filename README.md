# QR Multi IMGs

> Detect QR codes from images — even difficult ones: blurry, small, rotated, low quality

![Version](https://img.shields.io/badge/version-v0.8.0-blue)
![License](https://img.shields.io/badge/license-MIT-green)
![Python](https://img.shields.io/badge/python-3.9+-blue)
![CI](https://github.com/thousandflowers/qr-multi-imgs/actions/workflows/ci.yml/badge.svg)

QR codes in real photos are often blurry, poorly cut, very small, or rotated. **QR Multi IMGs** uses a progressive pipeline of 11 detection methods to find QR codes that other scanners miss.

---

## Why This Tool?

| Problem | How QR Multi IMGs solves it |
|---------|-----------------------------|
| QR is **blurry** | Phase 2 sharpening & deblur methods activate automatically |
| QR is **very small** | Multi-scale detection from 0.5× up to 8× |
| QR is **rotated** | Rotation detection at 90°/180°/270° |
| QR is **low quality** | Adaptive threshold & morphology preprocessing |
| QR is **white-on-black** | Inverted image variant detection |
| You have **500 images** to scan | Parallel processing + watch folder mode |
| You need **scriptable output** | JSON + quiet mode + exit codes 0/2/4 |

---

## Installation

### pip (recommended)

```bash
pip install qr-multi-imgs
qr-multi-imgs --path ./images
```

### Homebrew

```bash
brew tap thousandflowers/qr-multi-imgs
brew install qr-multi-imgs
qr-multi-imgs --path ./images
```

### From source

```bash
git clone https://github.com/thousandflowers/qr-multi-imgs.git
cd qr-multi-imgs
pip install -r requirements.txt
python3 qr_multi_imgs.py --path ./images
```

### Dependencies

- Python 3.9+
- zbar (`brew install zbar` on macOS, `apt install libzbar0` on Linux)
- OpenCV (auto-installed via pip)

---

## Quick Start

### Interactive menu (no arguments)

```bash
qr-multi-imgs
```

### Scan a folder

```bash
qr-multi-imgs --path ./images
```

### Scan a single image

```bash
qr-multi-imgs --path image.png
```

### Pipe from another command

```bash
curl -s https://example.com/qr.png | qr-multi-imgs --path -
```

### Scriptable JSON output

```bash
qr-multi-imgs --path ./images --json --quiet
```

### Maximum detection (try everything)

```bash
qr-multi-imgs --path ./images --force-deep --verbose
```

---

## Real-World Use Cases

### "I have a folder with 500 receipts. Delete those without QR codes."

```bash
qr-multi-imgs --path ./receipts --action delete --confirm
```

Deletes every image that doesn't contain a QR code. `--confirm` skips the safety prompt.

### "Find all QR codes containing 'mcdonalds' and save as JSON."

```bash
qr-multi-imgs --path ./images --action filter --filter-pattern mcdonalds --export-format json
```

### "Extract all QR code regions into separate image files."

```bash
qr-multi-imgs --path ./images --action extract --padding 10 --output ./cropped
```

### "Rename files based on their QR content."

```bash
qr-multi-imgs --path ./images --action batch-rename --confirm
```

### "Watch a folder for new images and scan them automatically."

```bash
qr-multi-imgs --path ./incoming --action watch
```

### "Verify that recreated QR codes match the originals."

```bash
qr-multi-imgs --path ./originals --action verify --output ./recreated
```

---

## Features

| Feature | Description |
|---------|-------------|
| **11 Detection Methods** | Progressive pipeline for difficult QR codes (blurry, small, rotated, low quality) |
| **Single file mode** | Scan individual images directly: `qr-multi-imgs --path image.png` |
| **stdin/pipe support** | `cat image.png \| qr-multi-imgs --path -` |
| **Parallel processing** | `--parallel` for scanning thousands of images |
| **10 CLI Actions** | List, export, delete, organize, recreate, extract, decode, filter, batch-rename, verify |
| **Watch folder** | Continuous monitoring of a directory for new images |
| **Webcam mode** | Real-time QR scanning from camera |
| **Barcode support** | Detect 1D barcodes alongside QR codes (`--symbols`) |
| **Structured QR parsing** | Auto-detect WiFi, vCard, URL, email, SMS, phone, geo, calendar events |
| **Scannability scoring** | Quality score (A–F) for each detected QR code |
| **JSON output** | Structured output for scripting (`--json`) |
| **Quiet mode** | Suppress progress, only output results (`--quiet`) |
| **Memory safe** | Proper image resource cleanup — no leaks on large batches |
| **Thread-safe** | Safe parallel processing with result locking |
| **Path validation** | Path traversal protection for security |
| **ANSI color output** | Clear visual feedback with `NO_COLOR` / `--no-color` support |
| **Shell completions** | Bash, Zsh, and Fish completion scripts |

---

## QR Code Detection Pipeline

```
Phase 1 (always, ~10ms–50ms)
├── Basic decode
├── Grayscale
├── Contrast + Unsharp mask
└── Resize 2× (Lanczos / Bicubic)

Phase 2 (deep_scan, enabled by default, ~50ms–200ms)
├── Sharpen (extreme blur recovery)
├── Deblur (gaussian + bilateral filters)
└── Resize 3×

Phase 3 (force_deep, ~500ms–3s)
├── Rotation detection (90°/180°/270°)
├── Multi-scale search (0.5×–3×)
├── QReader (ML-based detector)
├── Adaptive threshold
└── Morphology cleanup

Full (backup, force_deep)
├── Extreme scale (4×–8×)
└── Combined methods

Short-circuit: after 10 consecutive decode failures, expensive Phase 3/Full
are skipped — no wasted time on images without readable QR.
```

### Available Methods

| # | Method | Phase | Best For |
|---|--------|-------|----------|
| 1 | Basic decode | 1 | Normal QR codes |
| 2 | Grayscale | 1 | Low contrast |
| 3 | Contrast+Unsharp | 1 | Basic preprocessing |
| 4 | Sharpen | 2 | Blurry QR codes |
| 5 | Deblur | 2 | Very blurry QR |
| 6 | Rotation | 3 | Rotated QR codes |
| 7 | Multi-scale | 3 | Variable sizes |
| 8 | QReader (ML) | 3 | ML-based detection |
| 9 | Adaptive | 3 | Low quality images |
| 10 | Morphology | 3 | Damaged QR codes |
| 11 | Extreme Scale | Full | Very small/large QR |

---

## CLI Reference

### Actions

| Action | Description | Example |
|--------|-------------|---------|
| `list` (default) | Show results table | `--action list` |
| `export` | Save results to file | `--action export --export-format json` |
| `delete` | Delete images without QR | `--action delete --confirm` |
| `organize` | Sort into folders by QR presence | `--action organize --move` |
| `recreate` | Generate new QR code images | `--action recreate --qr-format png` |
| `extract` | Crop QR regions to separate files | `--action extract --padding 20` |
| `decode` | Print only decoded content | `--action decode` |
| `filter` | Filter by QR content pattern | `--action filter --filter-pattern mcdonalds` |
| `batch-rename` | Rename files by QR content | `--action batch-rename --confirm` |
| `verify` | Validate recreated QR codes | `--action verify --output ./recreated` |
| `webcam` | Real-time scan from webcam | `--action webcam` |
| `watch` | Watch folder for new files | `--action watch` |

### Options

| Option | Alias | Default | Description |
|--------|-------|---------|-------------|
| `--path` | `-p` | (interactive) | File or folder to scan |
| `--action` | `-a` | `list` | Action to perform |
| `--recursive` | `-r` | `false` | Scan subfolders recursively |
| `--verbose` | `-v` | `false` | Show method-level progress |
| `--quiet` | `-q` | `false` | Suppress all output except results/errors |
| `--json` | | `false` | Output JSON to stdout (scriptable) |
| `--force-deep` | | `false` | Maximum detection (slowest, best results) |
| `--timeout` | `-t` | `15s` | Timeout per image (0 = disabled) |
| `--parallel` | | `false` | Process images in parallel |
| `--progress` | | `false` | Show per-file progress indicators |
| `--no-color` | | `false` | Disable ANSI color output |
| `--version` | | | Print version and exit |
| `--dedup` | | `false` | Deduplicate identical QR contents |
| `--symbols` | `-s` | `QRCODE` | Comma-separated symbologies |
| `--score` | | `false` | Show scannability score (A–F) |
| `--show-qr` | | `false` | Render QR as Unicode art |
| `--output` | `-o` | auto | Output folder path |
| `--confirm` | | `false` | Skip confirmation prompts |
| `--completion` | | | Generate shell completion script |

### Exit Codes

| Code | Meaning | When |
|------|---------|------|
| **0** | QR/barcode detected | At least one image has a decodable code |
| **2** | Processing error | Invalid path, missing file, I/O error |
| **4** | No QR/barcode found | All images processed but no codes detected (distinct from error!) |

Use exit codes in scripts:
```bash
qr-multi-imgs --path ./images --action decode --quiet
if [ $? -eq 4 ]; then
  echo "No QR codes found — this is not an error"
fi
```

---

## Safety & Performance

- **Memory cleanup**: All PIL `Image.open()` calls use context managers or explicit `.close()`. Temporary detection images are garbage-collected immediately. No memory leaks even on 10,000+ image batches.
- **Timeout**: Each image has a configurable timeout (`--timeout 30`). If detection exceeds the limit, the image is skipped with a timeout error — no indefinite hangs on corrupted or giant files.
- **Short-circuit**: After 10 consecutive decode failures across all Phase 1 & 2 methods, expensive Phase 3/Full is skipped. The tool doesn't waste CPU/ML inference on images that clearly have no QR code.
- **Path traversal protection**: `--path` is validated against a base directory. Paths like `../../etc/passwd` are rejected.
- **Thread safety**: Parallel mode uses `ThreadPoolExecutor` with per-image timeout. Results collection is protected by a lock. No race conditions on shared state.
- **Signal handling**: SIGINT (Ctrl+C) is caught during parallel scans for clean shutdown.

---

## Benchmarks

Preliminary comparison against `zbarimg` (zbar-tools) on a test set of 100 real-world photos containing QR codes:

| Tool | Detected (out of 100) | Notes |
|------|----------------------|-------|
| `zbarimg` (default) | 68 | Fast but misses blurry/small codes |
| `qr-multi-imgs` (default) | 89 | Phase 1 + Phase 2 active |
| `qr-multi-imgs` (`--force-deep`) | 96 | Includes ML-based QReader + rotation |

*Results vary by image quality. For clean, well-lit QR codes, all tools perform similarly.
The gap widens dramatically on blurry, rotated, or low-quality images.*

---

## Troubleshooting

### "zbar library not found"

```bash
# macOS
brew install zbar

# Linux
sudo apt-get install libzbar0
```

### "qr-multi-imgs: command not found"

After `pip install`, ensure the Python scripts directory is in your `PATH`:
```bash
# macOS/Linux
export PATH="$PATH:$HOME/.local/bin"
```

### Slow scanning

```bash
# Use parallel mode for many images
qr-multi-imgs --path ./images --parallel --progress

# Reduce timeout for quick scans
qr-multi-imgs --path ./images --timeout 5
```

### Many images not detected

```bash
# Try maximum detection
qr-multi-imgs --path ./images --force-deep --verbose
```

---

## License

MIT License — see [LICENSE](LICENSE)

## Credits

- **Author**: thousandflowers
- **GitHub**: https://github.com/thousandflowers/qr-multi-imgs
- **Issues**: https://github.com/thousandflowers/qr-multi-imgs/issues
- **Detection pipeline**: Built on pyzbar + OpenCV + QReader (optional ML backend)
