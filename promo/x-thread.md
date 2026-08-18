# X thread

Five posts. Post 1 carries the GIF — it is what stops the scroll.
Attribution goes to @charmcli in post 4, not post 1: tagging in the first
post of a thread suppresses reach and reads as begging.

## 1 — hook + GIF

```
my dad photographs order receipts. every one has a QR code. his workflow was:
open photo → point phone at screen → type code into a spreadsheet. hundreds a
week.

so I wrote the tool that should have existed.
```

*Attach `demo.gif`.*

## 2 — what it does

```
qr-multi-imgs: point it at a folder, it decodes every QR in it, then acts on the
results.

split into with_qr/ and without_qr/ · delete the misfires · export JSON/CSV/TXT ·
regenerate the codes as PNG/JPG/SVG

drag the folder onto the terminal. that's the whole setup.
```

## 3 — the benchmark

```
benchmarked it on a public dataset: 3332 deliberately damaged QR images.

v1: 1863 decoded. 56%. 7m30s.
now: 3332/3332. ~7s.

the 100% had nothing to do with the decoder.
```

## 4 — the actual finding

```
the misses weren't decodable. 100+ QR modules rendered into 256px = ~2px per
module. after the dataset's blur, the information isn't in the raster anymore.
no decoder gets those.

but the dataset ships the generator's bit matrix as .npy next to each image. so
for those, read the matrix, not the pixels. 132 lines of Go.

TUI is @charmcli's bubbletea, which made the whole thing a pleasure to build.
```

## 5 — honesty + link

```
being precise: 100% is a benchmark number leaning on metadata the benchmark
publishes. your photos have no .npy. for those it's the 56% raster path, plus
Apple Vision on macOS for photographed codes.

pure Go, nothing to install, MIT:
github.com/thousandflowers/qr-multi-imgs
```

## Notes

- Charm is **@charmcli** on X. Bubble Tea has no separate account.
- Post 5 is the one that gets quote-tweeted by people who like that a
  developer undercut their own headline number. Do not soften it.
- If it moves at all, reply to your own post 5 with the `go install` line
  a few hours later rather than editing the thread.
