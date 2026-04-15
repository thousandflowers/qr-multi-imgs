# QR Multi IMG

> QR Code Scanner for Images - Scan a folder of images and detect QR codes

![Version](https://img.shields.io/badge/version-v0.2.0-blue)
![License](https://img.shields.io/badge/license-MIT-green)
![Python](https://img.shields.io/badge/python-3.12+-blue)

**QR Multi IMG** is a powerful tool that scans folders of images, detects QR codes, extracts their content, and offers multiple actions like listing, exporting, organizing, recreating, and extracting QR code regions from original images.

## Features

- **6 Actions**: list, export, delete, organize, recreate, extract
- **Extract QR Regions**: Crop actual QR code regions from images with padding
- **Interactive TUI**: Menu-based interface (default)
- **CLI Mode**: Use `--nomenu` for command-line only
- **Recursive Scanning**: Scan subfolders with `-r` flag
- **Multiple QR Codes**: Extract all QR codes per image
- **Multiple Retry Methods**: Handles difficult/damaged QR codes
- **UTF-8 Encoding**: Supports special characters and emoji
- **Custom Formats**: Support for txt, json, csv export
- **Case-Insensitive**: Works with .JPG, .jpg, .Png, etc.
- **Path Validation**: Security against path traversal attacks

## Installation

### Recommended: Homebrew

#### Option 1: From official tap (once published)
```bash
brew install thousandflowers/qr-multi-img/qr-multi-img
```

#### Option 2: From local tap (for development/testing)
```bash
cd /path/to/qr-multi-img
brew install ./Formula/qr-multi-img.rb
```

Then run with:
```bash
qr-multi-img
```

### Alternative: Clone + pip

```bash
# Clone the repository
git clone https://github.com/thousandflowers/qr-multi-img.git
cd qr-multi-img

# Install dependencies
pip install -r requirements.txt

# Run the program
python3 qr_multi_img.py
```

### Manual Installation

Requires:
- Python 3.12+
- Homebrew (macOS)

```bash
# Install system dependencies
brew install zbar

# Install Python dependencies
pip install textual pyzbar Pillow qrcode

# Run
python3 qr_multi_img.py
```

## Usage

### Interactive TUI Mode (Default)

When you run the program without any arguments, you'll see the interactive menu:

```
┌─────────────────────────────────────┐
│         QR Multi IMG                │
│    QR Code Scanner for Images       │
├─────────────────────────────────────┤
│                                     │
│   > List all images                 │
│     Export results                  │
│     Delete without QR               │
│     Organize into folders           │
│     Recreate QR codes               │
│                                     │
│   Press q to quit                   │
└─────────────────────────────────────┘
```

Use arrow keys to navigate and Enter to select.

### CLI Mode

Use `--nomenu` to skip the interactive menu and use command-line arguments:

```bash
# List all images with QR codes
python3 qr_multi_img.py --nomenu --path /path/to/images --action list

# Export results to JSON
python3 qr_multi_img.py --nomenu --path /path/to/images --action export --export-format json

# Delete images without QR codes
python3 qr_multi_img.py --nomenu --path /path/to/images --action delete --confirm

# Organize images into folders
python3 qr_multi_img.py --nomenu --path /path/to/images --action organize --move

# Generate new QR code images
python3 qr_multi_img.py --nomenu --path /path/to/images --action recreate --qr-format png
```

## CLI Options

| Option | Alias | Description | Default |
|--------|-------|-------------|---------|
| `--path` | `-p` | Folder path to scan | Required |
| `--action` | `-a` | Action to perform | `list` |
| `--recursive` | `-r` | Scan subfolders recursively | `false` |
| `--formats` | `-f` | Image formats (comma-separated) | All supported |
| `--output` | `-o` | Output folder path | Auto |
| `--export-format` | | Export format (txt/json/csv) | `txt` |
| `--qr-format` | | QR image format (png/svg/pdf) | `png` |
| `--move` | | Move files instead of copy | `false` |
| `--confirm` | | Skip confirmation prompt | `false` |
| `--parallel` | | Process images in parallel | `false` |
| `--progress` | | Show progress during scan | `false` |
| `--log` | | Save log to file | `false` |
| `--nomenu` | | Skip interactive menu | `false` |
| `--naming` | | File naming (original/content/sequential) | `original` |

## Actions

### 1. list
Prints results to console showing which images have QR codes and their content.

```bash
qr-multi-img --path /images --action list
```

### 2. export
Saves scan results to a file (txt, json, or csv).

```bash
qr-multi-img --path /images --action export --export-format json
qr-multi-img --path /images --action export --export-format csv --output results.csv
```

### 3. delete
Deletes images WITHOUT QR codes.

```bash
qr-multi-img --path /images --action delete --confirm
```

### 4. organize
Copies or moves images to `with_qr/` and `without_qr/` folders.

```bash
qr-multi-img --path /images --action organize
qr-multi-img --path /images --action organize --move --confirm
```

### 5. recreate
Generates new QR code images from extracted content.

```bash
qr-multi-img --path /images --action recreate --qr-format png
qr-multi-img --path /images --action recreate --qr-format svg --naming content
```

### 6. extract
Extracts QR code regions from original images with padding.

```bash
qr-multi-img --path /images --action extract
qr-multi-img --path /images --action extract --padding 30 --output ./extracted
qr-multi-img --path /images --action extract --naming sequential
```

Options:
- `--padding` - Safe area around QR code (default: 20px)
- `--naming` - File naming: original/content/sequential
- `--output` - Output folder path

## Supported Image Formats

- JPG / JPEG
- PNG
- BMP
- GIF
- WebP
- TIFF

Case-insensitive: `.jpg`, `.JPG`, `.Jpg` all supported.

## Examples

### Scan Desktop Images Folder
```bash
qr-multi-img --path ~/Desktop/images --action list
```

### Scan All Subfolders
```bash
qr-multi-img --path ~/Pictures --action list --recursive
```

### Export to JSON with Progress
```bash
qr-multi-img --path /images --action export --export-format json --progress
```

### Generate QR Code Images
```bash
qr-multi-img --path /photos --action recreate --qr-format png --naming sequential
```

## Homebrew Tap Installation

### Install
```bash
brew install thousandflowers/qr-multi-img/qr-multi-img
```

### Update
```bash
brew upgrade qr-multi-img
```

### Uninstall
```bash
brew uninstall qr-multi-img
```

### Tap Details
- Repository: https://github.com/thousandflowers/qr-multi-img
- Formula: `Formula/qr-multi-img.rb`

## Troubleshooting

### "Cannot read image" Error

The program uses multiple retry methods to handle difficult images:
1. Standard decoding
2. Grayscale conversion
3. Image preprocessing (contrast/sharpness enhancement)
4. Resolution scaling

### "zbar library not found"

If you get this error, install zbar:
```bash
brew install zbar
```

### Slow Scanning

Use parallel processing for large folders:
```bash
qr-multi-img --path /images --action list --parallel --progress
```

### Large Images

For very large images, a timeout of 30 seconds per image is applied. Use `--progress` to see progress.

## Version History

- **v0.2.0** - Bug fixes and improvements
  - Fixed version docstring consistency
  - Added helper methods for code reuse
  - Added path validation for delete/organize actions
  - Fixed zbar library path resolution on macOS

- **v0.1.0** (beta) - Initial release
  - 6 actions (list, export, delete, organize, recreate, extract)
  - Interactive TUI menu
  - Multiple retry methods for difficult QR codes
  - Homebrew tap support

## License

MIT License - See [LICENSE](LICENSE) file.

## Author

QR Multi IMG Team
- GitHub: https://github.com/thousandflowers/qr-multi-img

## Contributing

Contributions are welcome! Please open an issue or submit a PR.
