# awesome-go & awesome-tui Submissions

Material prepared for PRs to add qr-multi-imgs to curated lists.

---

## awesome-go

**Repo:** https://github.com/avelino/awesome-go

**Section:** Image Processing → https://github.com/avelino/awesome-go?tab=readme-ov-file#image-processing

**Entry to add:**

```markdown
* [qr-multi-imgs](https://github.com/thousandflowers/qr-multi-imgs) - Batch QR code scanner with TUI: scan a folder of images, decode QR codes, organize, export (JSON/CSV/TXT), and recreate QR images. Pure Go, zero CGo.
```

**PR body:**

```
Add qr-multi-imgs to Image Processing section.

qr-multi-imgs is a batch QR code scanner with a Bubbletea TUI.
It scans entire folders of images, decodes QR codes using
gozxing, exports results (JSON/CSV/TXT), organizes files by
QR presence, deletes non-QR images, and recreates QR code
images from decoded content (PNG/JPG/SVG). Pure Go, zero CGo.

The README includes a benchmark: 3332/3332 detection at ~142 img/s
on the lovasoa/qrcode-dataset.
```

---

## awesome-tui

**Repo:** https://github.com/rothgar/awesome-tui

**Section:** See CONTRIBUTING.md for placement — likely under "Utilities" or similar.

**Entry to add:**

```markdown
* [qr-multi-imgs](https://github.com/thousandflowers/qr-multi-imgs) - Batch QR code scanner: scan a folder, decode QR codes, organize files, export results, all through a Bubbletea TUI with drag & drop.
```

**PR body:**

```
Add qr-multi-imgs to the list.

qr-multi-imgs is a terminal UI built with Bubbletea for batch
QR code scanning. Users drag a folder onto the terminal (or type
a path), and the TUI shows scan results, lets them export,
delete images without QR, organize files into subfolders, and
recreate QR images in PNG/JPG/SVG format. Supports keyboard
navigation, 5 input methods, and has a parallel scan engine
that processes 3332 images in ~23s.
```
