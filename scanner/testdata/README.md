# scanner/testdata

`vision_qr.png` is a fixture for `TestDecodeWithVision` (darwin + cgo only).

`corpus/` is the ground-truth benchmark corpus for `TestCorpus`.

## Running the benchmark

```sh
go test -tags corpus ./scanner -run TestCorpus -v
```

It prints total / correct / wrong / missed / errors, the success rate, and one
line per failing file. It never fails on a low score — it measures, it does not
gate. It only fails if the manifest is missing or empty.

Against a private photo set that is never committed:

```sh
QR_CORPUS_DIR=~/Pictures/qr-set go test -tags corpus ./scanner \
  -run TestCorpus -v -timeout 0 -count=1
```

- `QR_CORPUS_DIR` defaults to `testdata/corpus`.
- `-timeout 0` — the harness is sequential and Apple Vision is slow; a few
  thousand photos will blow past the 10-minute default.
- `-count=1` — `go test` caches by source and env, not by corpus content, so a
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
```

- `expected` is the exact decoded payload — no trimming, so trailing whitespace
  in a payload must be quoted and preserved.
- `EXPECTED_FAIL` means the image contains no QR. Decoding anything from it is
  scored as a false positive, not a success.
- Rows starting with `#` are comments. The `path,expected` header is optional.
- Standard CSV quoting, so payloads may contain commas, quotes, and newlines.

Extracting ground truth is manual by design: decoding the image with this tool
to fill in `expected` would make the benchmark measure nothing. Read the payload
with a phone camera, another decoder, or from whatever generated the code.

## Do not commit real photos

`corpus/.gitignore` ignores everything in that directory except the manifest and
the two tiny synthetic samples. Keep private sets outside the repo and point
`QR_CORPUS_DIR` at them.
