# LinkedIn

One post, no thread. Attach `demo.gif` - LinkedIn plays GIFs inline and
the terminal recording is the whole pitch.

## Post

```
My father catalogs orders by photographing receipts. Every receipt has a QR code
on it. His workflow was: open the photo, point his phone at the screen, type the
code into a spreadsheet. Hundreds of times a week.

Every QR tool I could find decodes one image and prints a string. None of them
could take a folder and then do something with the results.

So I built qr-multi-imgs — a terminal app that scans an entire folder, decodes
every QR, and then acts on what it found: sorts the images into with_qr and
without_qr, deletes the misfires, exports JSON/CSV/TXT, or regenerates the codes
as PNG, JPG or SVG. Drag a folder onto the terminal and it starts.

The unexpected part was the benchmark. Against a public dataset of 3332
deliberately damaged QR images, the first version decoded 56% in seven and a
half minutes. The final one decodes all 3332 in about seven seconds.

The speed came from running six workers in parallel. The detection did not — it
came from realising the hardest images were not decodable at all: 100+ QR modules
squeezed into 256 pixels means a single module lands on ~2 pixels, and after the
dataset's blur the information simply is not in the image anymore. No decoder was
going to get those. The dataset ships the generator's own bit matrix alongside
each image, so for those samples the tool reads the matrix instead of the pixels.

Worth being precise: 100% is a benchmark number that leans on metadata the
benchmark publishes. On a real photo, the honest figure is that 56% path — plus
Apple's Vision framework on macOS for photographed codes.

Written in Go, TUI built with Bubble Tea from the Charm toolkit — which is,
quietly, some of the nicest developer tooling being made right now. MIT licensed.

github.com/thousandflowers/qr-multi-imgs
```

## Notes

- Tag **Charm** (the company page) on the Bubble Tea mention - the team
  reliably engages with things built on their stack, and that is where
  this post's reach actually comes from.
- Do not put the GitHub link in the first two lines; LinkedIn suppresses
  posts that lead with an external URL. Last line, as above, is fine.
- Hashtags, three at most, at the very bottom: #golang #opensource #cli
