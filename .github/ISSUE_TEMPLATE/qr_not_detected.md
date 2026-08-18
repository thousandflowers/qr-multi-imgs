---
name: QR code not detected
about: A code you can read by eye that qr-multi-imgs misses
title: ''
labels: detection
assignees: ''

---

**The image**
Attach the image (or one just like it). Without a sample this is very hard
to act on — the decode path taken depends on the pixels.

**What the QR contains**
The payload, if you know it — decoded by a phone or another tool.

**How the image was produced**
- [ ] Photographed (phone / camera)
- [ ] Screenshot
- [ ] Scanned document
- [ ] Generated / rendered
- [ ] Other:

**Output**
```
paste the row qr-multi-imgs showed for this file
```

**Environment**
- OS: [e.g. macOS 15, Ubuntu 24.04]
- Version: `qr-multi-imgs --version`

On macOS the Apple Vision backend also runs, so the same image can behave
differently on Linux and Windows. Please say which you saw it on.
