# scanner/testdata

`vision_qr.png` is a fixture for `TestDecodeWithVision` (darwin + cgo only).

`corpus/` is the ground-truth benchmark corpus for `TestCorpus`.

## Running the benchmark

```sh
go test -tags corpus ./scanner -run TestCorpus -v
```

### Companion masks are ignored by default

Some datasets ship the generator's own bit-matrix beside each image as a `.npy`
file. `ScanImageDetail` reads that first and short-circuits, which is the right
behaviour for a user and the wrong one for a benchmark: it measures the speed of
parsing an answer key, on a path where no QR decoding happens at all.

So the harness forces every image through the raster loop. The mask path is
still reachable, and the run says which mode it was in:

```sh
go test -tags corpus ./scanner -run TestCorpus -v -corpus.masks
```

Use it to check the mask path has not regressed. Do not quote its recall or its
throughput as a decoder number.

It scores each image as exact, partial, missed, false positive, or correct
negative, and separately counts codes so per-code recall is visible next to the
per-image rate - an image with three codes where two decode is neither a pass
nor a plain failure. One line per failing file. It never fails on a low score:
it measures, it does not gate. It only fails if the manifest is missing,
empty, or malformed.

It measures the CLI path (`ScanImage`, i.e. `ScanFast`) deliberately. Ground
truth is built with `ScanExhaustive`, so the gap between the two numbers is
exactly the set of codes the fast path misses and Apple Vision recovers - and
what a browser build can never reach.

Against a private photo set that is never committed:

```sh
QR_CORPUS_DIR=~/Pictures/qr-set go test -tags corpus ./scanner \
  -run TestCorpus -v -timeout 0 -count=1
```

- `QR_CORPUS_DIR` defaults to `testdata/corpus`.
- `-timeout 0` - the harness is sequential and Apple Vision is slow; a few
  thousand photos will blow past the 10-minute default.
- `-count=1` - `go test` caches by source and env, not by corpus content, so a
  rerun after editing images or the manifest otherwise replays a stale result.

## Populating a corpus

A corpus directory is any directory containing a `corpus.csv` manifest plus the
images it names. Images may sit in subdirectories; paths are relative to the
manifest and use `/` separators.

```csv
path,expected
receipts/lidl-2024-03.jpg,https://example.com/r/8fa21
receipts/blurry-boarding-pass.jpg,EXPECTED_FAIL
posters/gradient-qr.png,"hello, world"
photos/two-receipts.jpg,https://example.com/r/aaa
photos/two-receipts.jpg,https://example.com/r/bbb
photos/duplicate-receipts.jpg,https://example.com/r/ccc
photos/duplicate-receipts.jpg,https://example.com/r/ccc
```

- **An image holding several codes gets several rows sharing its path**, one
  row per payload, in reading order (top to bottom, then left to right).
- **Two rows with the same path and the same payload mean two physically
  distinct codes that happen to carry identical content** - two copies of one
  receipt in a frame. They are not a mistake and must not be collapsed: the
  decoder must find both to score both.
- `expected` is the exact decoded payload - no trimming, so trailing whitespace
  in a payload must be quoted and preserved.
- `EXPECTED_FAIL` means the image contains no QR. Decoding anything from it is
  scored as a false positive, not a success.
- Rows starting with `#` are comments. The `path,expected` header is optional.
- Standard CSV quoting, so payloads may contain commas, quotes, and newlines.
- Exactly two fields per row, always. Ragged rows are rejected rather than
  guessed at, so an unescaped comma in a hand-typed payload is caught instead
  of silently becoming an extra expected code.

Extracting ground truth is manual by design: decoding the image with this tool
to fill in `expected` would make the benchmark measure nothing. Read the payload
with a phone camera, another decoder, or from whatever generated the code.

### When the dataset ships its own answers

A generated dataset often writes the string it encoded beside the image. That
is real ground truth and it can be read directly:

```sh
go run ./cmd/corpusgen -truth .txt /path/to/dataset
```

Nothing is decoded on this path. Each payload is the sidecar's bytes verbatim,
no trimming, so a generator that terminates its answers with a newline needs
handling before the manifest is trusted. An image with no sidecar is left out of
the manifest rather than labelled `EXPECTED_FAIL` - a missing answer is an
unknown, not a claim that the image holds no QR.

The reference set is
[lovasoa/qrcode-dataset](https://github.com/lovasoa/qrcode-dataset) v0.1: 3332
images, each with a `.txt` payload and a `.npy` bit-matrix.

### A corpus of real photographs

`QR_CORPUS_DIR` takes any directory with a manifest, whatever the formats in it.
HEIC needs a build that can read it - either `-tags heic`, or macOS with cgo
where Apple Vision reads it from the path. A build with neither **skips** such a
corpus rather than scoring it, because 0% recall caused by the build reads
exactly like 0% recall caused by the decoder.

```sh
QR_CORPUS_DIR=~/Pictures/heic-set go test -tags corpus,heic ./scanner \
  -run TestCorpus -v -timeout 0 -count=1
```

The manifest for a photo set is hand-written. There is no sidecar to read and no
generator to ask, so the payloads come off the codes by hand.

## Do not commit real photos

`corpus/.gitignore` ignores everything in that directory except the manifest and
the two tiny synthetic samples. Keep private sets outside the repo and point
`QR_CORPUS_DIR` at them.
