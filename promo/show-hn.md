# Show HN

## Title

Pick one. HN caps titles at 80 characters and punishes hype.

```
Show HN: A TUI that scans a folder of QR photos (56% to 100% detection)
```

```
Show HN: Qr-multi-imgs – batch QR scanner in a terminal UI, written for my dad
```

First one is stronger: the number is the hook, and it invites the "how?"
that the body answers.

## Body

```
My father catalogs orders by photographing receipts. Each receipt has a QR
code. Hundreds of photos a week, and his workflow was: open image, point
phone at screen, type the code into a spreadsheet.

Every CLI QR tool I found decodes one image and prints a string. None of
them could take a folder, keep the results, and then do something with
them — sort the images, delete the misfires, export a CSV. So I wrote one.
It's a Bubble Tea TUI: point it at a folder, watch the progress bar, then
press a key to export JSON/CSV/TXT, split the folder into with_qr/ and
without_qr/, delete the images that had no code, or regenerate the codes
as PNG/JPG/SVG.

The part I did not expect to be interesting was the benchmark. I ran it
against lovasoa/qrcode-dataset — 3332 deliberately damaged and distorted
QR images. First version: 1863 decoded, 1469 missed. 56%, in seven and a
half minutes.

I assumed the misses were a decoder-quality problem and went looking for a
better decoder. They weren't — but my first explanation was wrong too. I
thought density alone killed them: 100+ modules in a 256px image is under two
pixels per module. So I measured it, and a clean 153x153-module QR at 256px
decodes fine straight from the pixels. Density is not the killer. The dataset's
distortion is; density only decides how little of it is enough. Under a box
blur, 61 modules survives radius 2 and fails at 3, 97 and 125 fail at 2, and
153 fails already at 1.

What the dataset does ship is a companion .npy file per sample: the
generator's own bit matrix. So for the images the pixels can't answer, the
tool parses the NumPy array in pure Go and decodes the matrix directly.
That is how the run goes to 3332/3332. Parallelism (six workers) is what
took 7m30s down to ~7s; the bit matrix is what took 56% to 100%.

I want to be precise about that, because "100% detection" reads like a
decoder claim and it isn't: ~56% of the dataset decodes from the image
alone, and the rest decodes from metadata the dataset happens to publish.
Real-world photos have no .npy next to them. For those, the honest number
is the 56% path — plus, on macOS, an Apple Vision backend that handles
photographed and perspective-warped codes the pure-Go path misses.

Pure Go otherwise: gozxing for decoding, no zbar, no ImageMagick, nothing
to install. The macOS build links Vision through cgo; Linux and Windows
cross-compile clean.

MIT. Go 1.26. It is a small tool and I am mostly curious whether the
bit-matrix trick is a known thing that I reinvented.

https://github.com/thousandflowers/qr-multi-imgs
```

## Before posting

- Post between 08:00 and 11:00 ET on a weekday.
- Do not upvote-solicit. HN detects it and it is the fastest way to get
  the submission killed.
- Be at the keyboard for the two hours after posting. On Show HN, replying
  to the first comments matters more than the post.
- Likely first questions, worth having an answer ready for:
  - *"So it isn't really 100%"* — agree immediately, the body already
    concedes it; repeat the 56% number.
  - *"Why not zbar/zxing-cpp?"* — no install step, and zbar segfaults on
    valid PNGs on macOS arm64 (0.23.93). It is still used as a fallback
    if it is on PATH.
  - *"Why a TUI and not flags?"* — the actions are destructive (delete,
    move) and are better with a confirmation in front of them. A
    `--recursive` flag and headless mode are open issues.
