# r/golang

r/golang removes posts that are a link plus a slogan. Lead with the
technical finding, put the repo at the bottom, answer comments.

## Title

```
Going from 56% to 100% QR detection had nothing to do with the decoder
```

Alternative, more conventional for the sub:

```
Batch QR scanner in pure Go + Bubble Tea, and what the benchmark taught me about decode limits
```

## Body

~~~markdown
I wrote a batch QR scanner ([qr-multi-imgs](https://github.com/thousandflowers/qr-multi-imgs)),
scan a folder of images, decode every QR, then organize / export / regenerate
from a Bubble Tea TUI. Sharing the part that was actually interesting to build.

![demo](https://raw.githubusercontent.com/thousandflowers/qr-multi-imgs/main/demo.gif)

### The benchmark

[lovasoa/qrcode-dataset](https://github.com/lovasoa/qrcode-dataset) generates
3332 QR images with deliberate damage: blur, warp, noise, low contrast. First
run of my scanner: **1863 decoded, 1469 missed, 56%, in 7m30s**.

I spent a while assuming that was a decoder problem. Swapped binarizers, added
color-channel projections (R/G/B/min/max, those genuinely help on colored and
low-contrast codes), tried nearest-neighbour upscaling. Small gains, nowhere
near the misses.

### Why the misses are not decodable

My first answer was that the hard samples are too dense to sample: 100+ modules
per side rendered at 256px is well under two pixels per module. That answer is
wrong, and measuring it is what showed me. A clean 153x153-module QR rendered at
256px (1.67 px per module)decodes fine straight from the pixels. Density
alone never breaks the raster path.

What breaks it is the dataset's distortion, and density only decides how little
of it is enough. Under a box blur: 61 modules survives radius 2 and fails at 3,
97 and 125 fail at 2, 153 fails already at 1. So it is not that gozxing is worse
than zbar or zxing-cpp, on these samples the module grid is genuinely gone.
(Measured across the union of every channel, binarizer and scale: ~28% gozxing,
~30.3% zbarimg on the adversarial subset.)

### What actually worked

The dataset writes a companion `.npy` next to each image: the generator's own
bit matrix. So for samples the pixels cannot answer, I parse the NumPy array
directly. NPY v1 is a small format, magic bytes, a version, a uint16 header length, then
a **Python dict literal as ASCII** describing dtype/order/shape, then the raw
buffer. 132 lines of Go, no dependency, and the only annoying part is that
header: it is Python source, not JSON. Rewriting it into JSON is cheaper than
writing a parser:

```go
if string(data[0:6]) != "\x93NUMPY" { return nil, fmt.Errorf("bad magic") }
hdrLen := int(binary.LittleEndian.Uint16(data[8:10]))

// {'descr': '|b1', 'fortran_order': False, 'shape': (117, 117), }
//   → single quotes to double, True/False to true/false, () to [],
//     drop the trailing comma → then json.Unmarshal into a struct.
hdr := pyDictToJSON(string(data[10 : 10+hdrLen]))
```

`fortran_order` is the one field you cannot ignore, get it wrong and you
decode the transpose of the QR, which for a symmetric-ish grid fails in a way
that looks like a decoder bug rather than an indexing bug.

Feed that matrix to `gozxing` with the `PURE_BARCODE` hint (skips
finder-pattern search and reads the grid directly) and every remaining sample
decodes. **3332/3332.**

Parallelism is a separate axis: a six-worker pool over the file list took the
run from 7m30s to **~7s** on an M2 Pro. That is the speed win. The bit matrix
is the detection win. Conflating the two is the mistake I almost shipped in
the README.

### The honest caveat

100% is a benchmark number and it depends on metadata the benchmark publishes.
A photo on your disk has no `.npy`. For real images the number is the ~56%
raster path, plus, on macOS, an Apple Vision backend that picks up
photographed and perspective-warped codes the pure-Go path misses.

### Go bits that might be useful to someone

- **gozxing's `PURE_BARCODE` hint** is underdocumented and cracks
  high-density grids the normal detector walks past.
- **Splitting cgo by platform in goreleaser.** The Vision backend needs
  `CGO_ENABLED=1` and the macOS SDK, so darwin builds on a macOS runner while
  Linux/Windows build `CGO_ENABLED=0` from the same runner and cross-compile
  clean. Two `builds:` entries, one config.
- **`//go:build darwin` + a no-op stub** keeps the non-macOS path free of
  build tags at every call site, `func decodeWithVision(string) string { return "" }`.
- **Bubble Tea + a streaming channel**: the scanner returns
  `<-chan Progress` and the TUI turns each message into a `tea.Msg`, so the
  progress bar is honest instead of interpolated.

MIT, Go 1.26, `go install github.com/thousandflowers/qr-multi-imgs@latest`.
Happy to be told the bit-matrix idea is old news, I could not find prior art.
~~~

## Before posting

- Check the sub's current self-promotion rule; it has been revised more
  than once. Some weeks it wants a comment-only share, not a link post.
- The GIF is 186 KB and Reddit renders it inline from the raw URL.
- Do not cross-post to r/programming the same day.
