# QR Multi IMGs

> Detect QR codes from images - even difficult ones: blurry, small, rotated, low quality

![Version](https://img.shields.io/badge/version-v0.6.0--Enhanced-blue)
![License](https://img.shields.io/badge/license-MIT-green)
![Python](https://img.shields.io/badge/python-3.12+-blue)

QR codes in real photos are often blurry, poorly cut, very small or rotated. QR Multi IMGs uses 11 progressive detection methods to find QR codes that other scanners miss.

---

## Why this tool

QR codes in real photos are often:
- **Blurry** or out of focus
- **Poorly cut**
- **Small** or very large
- **Rotated** at strange angles
- Of **low quality**

QR Multi IMGs addresses these issues with a pipeline of 11 methods that activate progressively.

---

## Installation

### Homebrew (recommended)

```bash
brew tap thousandflowers/qr-multi-imgs
brew install qr-multi-imgs
qr-multi-imgs --path ./images --action list
```

### pip

```bash
pip install -r requirements.txt
python3 qr_multi_imgs.py --path ./images --action list
```

### Dependencies

- Python 3.12+
- zbar (`brew install zbar`)

---

## Quick Start

### Interactive

```bash
python3 qr_multi_imgs.py
```

### CLI Mode

```bash
# Basic scan
python3 qr_multi_imgs.py --path ./images --action list

# With details
python3 qr_multi_imgs.py --path ./images --verbose

# Maximum detection (tries everything)
python3 qr_multi_imgs.py --path ./images --force-deep --verbose

# Export results
python3 qr_multi_imgs.py --path ./images --action export --export-format json
```

---

## Features

| Feature | Description |
|---------|-------------|
| **11 Detection Methods** | Progressive pipeline for difficult QR |
| **10 Actions** | list, export, delete, organize, recreate, extract, decode, filter, batch-rename, verify |
| **Memory Safe** | Proper image resource cleanup |
| **Thread-Safe** | Parallel processing |
| **Path Validation** | Path traversal protection |
| **Deep Scan** | Enhanced detection |
| **Verbose** | Detailed logging for debugging |

---

## QR Code Detection

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
| 8 | QReader | 3 | ML-based detection |
| 9 | Adaptive | 3 | Low quality images |
| 10 | Morphology | 3 | Damaged QR codes |
| 11 | Extreme Scale | Full | Very small/large QR |

### Detection Flow

```
Phase 1 (always)
├── Basic decode
├── Grayscale
├── Contrast + Unsharp
└── Resize 2x

Phase 2 (deep_scan)
├── Sharpen (blur)
├── Deblur (extreme blur)
└── Resize 3x

Phase 3 (force_deep)
├── Rotation (90°/180°/270°)
├── Multi-scale (0.5x-3x)
├── QReader (ML)
├── Adaptive threshold
└── Morphology

Full (fallback)
├── Extreme scale (4x-8x)
└── Method 11
```

---

## CLI Commands

### Actions

| Action | Description | Example |
|--------|-------------|---------|
| `list` | Show results | `--action list` |
| `export` | Save to file | `--action export --export-format json` |
| `delete` | Delete without QR | `--action delete --confirm` |
| `organize` | Move to folders | `--action organize --move` |
| `recreate` | Create new QR | `--action recreate --qr-format png` |
| `extract` | Extract QR regions | `--action extract --padding 20` |
| `decode` | Content only | `--action decode` |
| `filter` | Filter by pattern | `--action filter --filter-pattern "mcdonalds"` |
| `batch-rename` | Batch rename | `--action batch-rename --confirm` |
| `verify` | Verify QR | `--action verify --output ./recreated` |

### Options

| Option | Alias | Default | Description |
|--------|-------|---------|-------------|
| `--path` | `-p` | (required) | Image folder |
| `--action` | `-a` | `list` | Action to perform |
| `--recursive` | `-r` | false | Scan subfolders |
| `--verbose` | `-v` | false | Detailed output |
| `--force-deep` | | false | Maximum detection |
| `--deep-scan` | | true | Enhanced detection |
| `--parallel` | | false | Parallel processing |
| `--output` | `-o` | auto | Output folder |
| `--confirm` | | false | Skip confirmation |
| `--timeout` | `-t` | 15 | Timeout per image |

---

## Configuration

### Timeout

- Default: 15 seconds per image
- Deep: 30 seconds
- Use `--timeout 60` for large images

### Image Formats

jpg, jpeg, png, bmp, gif, webp, tiff, tif (case-insensitive)

---

## Troubleshooting

### "zbar library not found"

```bash
# macOS Apple Silicon
brew install zbar
export DYLD_LIBRARY_PATH=/opt/homebrew/lib

# macOS Intel
brew install zbar
export DYLD_LIBRARY_PATH=/usr/local/lib
```

### Slow scanning

```bash
# Use parallel for many images
python3 qr_multi_imgs.py --path ./images --parallel --progress

# Disable deep scan for fast basic scan
python3 qr_multi_imgs.py --path ./images --deep-scan=false
```

### Many images not detected

```bash
# Try maximum detection
python3 qr_multi_imgs.py --path ./images --force-deep --verbose
```

---

## License

MIT License - see LICENSE file

## Credits

- **Author**: QR Multi IMGS Team
- **GitHub**: https://github.com/thousandflowers/qr-multi-imgs
- **Issues**: https://github.com/thousandflowers/qr-multi-imgs/issues