# Roadmap — assisted decode: classification, retry, triage, contribution

**Scope:** what happens to an image the decoder cannot read.

**Status:** the *smallest useful slice* is implemented — three buckets, the
bounding box, and the reason surfaced in the CLI and the web app. **Phase 2 Step
A is implemented**: the corpus harness, the mask-ignoring benchmark default and
the per-image metadata the ladder will be measured with. No retry strategy has
been written yet, on purpose — the baseline exists first. Phase 3 is still
design only.

**The box is computed and returned, not drawn.** It reaches the JSON export, the
local API and the wasm result, and nothing renders it: the thumbnail overlay was
built and then removed, because an overlay is the first step of the retry flow
in Phase 2 and shipping it alone put a picture on screen with no action behind
it. Phase 3 is where it earns its place.

~~**And it does not reach the page at all.**~~ **Fixed in Phase 3 Step A.** The
worker used to copy only `res.classification` into `out.reason` and drop
`res.detections` on the floor. The geometry now reaches the row. Nothing draws
it yet — see Phase 3 Step A below for exactly what the page has in hand.

**Phase 4 is not going to be built as written.** Contribution stays out of
scope: no upload endpoint, no hosted backend. A failing case reaches the project
the way any other contribution does — through `corpusgen` and a pull request,
opened by someone who chose to. See the decision note in Phase 4.

## Decisions taken during implementation

- **Three buckets, not five.** `PARTIAL_FINDERS` and `HIGH_EDGE_DENSITY_REGION`
  were dropped. Partial finder centres are reachable — `FindMulti` populates
  `GetPossibleCenters()` before failing, verified empirically — but they fire
  where there is no code at all: **a uniform-noise image with nothing in it
  yields one centre**, and a QR with a single finder corner painted out yields
  two, indistinguishable from it by count. A bucket built on them tells users a
  code is there when nothing is. (An earlier draft of this note said ten centres
  on salt-and-pepper; re-measured against gozxing, the number is one. The
  conclusion is unchanged — one is already more than zero.)
- **The detected bucket is high-precision, not precision-1.** A full
  finder-pattern triple was assembled from **uniform random noise** in testing.
  Structured textures (stripes, checkerboards, text-like rows, gradients) all
  produced nothing. The README states this rather than implying certainty.
- **No `DECODED_PARTIAL`.** Open question 1 below stands: an image with one
  readable and one unreadable code classifies as `DECODED`, and its boxes are
  not kept. Still the most actionable state that goes unreported.
- **The two surfaces sort differently, on purpose.** The web leads with the
  images holding a code nobody could read (`rowRank`, `web/order.js`): there the
  row is the unit of work, carrying the thumbnail, the rename preview and the
  filter tab that selects it. The terminal leads with the images that decoded
  (`resultRank`, `scanner/scanner.go`) and then sorts by name — nothing in the
  TUI is per-row, every action works on the whole set, and `viewList` draws 25
  rows at a time, so ranking `NO_QR_FOUND` above the decoded rows would push
  every payload off the first screen of a real camera roll. Each order is pinned
  by its own test (`web/order.js --test`, `TestSortResultsPutsDecodedFirst`) so a
  change to one cannot quietly move the other.

## The problem (as it was, and what is left of it)

`scanner.ScanResult` used to be binary:

```go
type ScanResult struct {
	FilePath string   `json:"file_path"`
	HasQR    bool     `json:"has_qr"`
	Contents []string `json:"qr_contents,omitempty"`
	Error    string   `json:"error,omitempty"`
	FileSize int64    `json:"file_size"`
}
```

`Error` means *the file could not be read* — an unsupported format, a damaged
file. It did not mean *there is a code here and I failed*. Those two collapsed
into `HasQR: false`, and so did *this is a photo of a cat*.

**That part is fixed.** `ScanResult` (`scanner/scanner.go:34`) now carries
`Classification`, `Detections` and `Metadata` as additive `omitempty` fields,
and the web UI has a fourth stat tile and filter for the actionable bucket
(`fUnread`, `web/index.html:369`) beside images / with / codes / unique /
without / unopened. A receipt whose QR is glare-blown and a landscape photo no
longer sit in the same tile.

What is *not* fixed is the recall behind it. On the hard subset of the benchmark dataset ~70% of
images fail image-only decoding (README, *Benchmark*), and every one of them
tells the user the same nothing. The user has no recourse: no reason, no
retry, no way to point at the code they can see with their own eyes.

## What already exists (and what the phases are actually adding)

Read this before estimating anything — a good part of Phase 1 is *not deleting
data the code already computes*.

Line numbers below are as of the Phase 2 Step A commit. They go stale; the
symbol names do not, so grep for those first.

| Thing | Where | State |
|---|---|---|
| Finder-pattern pre-pass across every colour projection | `detectFinders`, `scanner/scanner.go:405` | **Runs today**, and returns `[]*FinderPatternInfo` — the geometry is kept, not counted and dropped. Was `countCandidates`. |
| `FinderPatternInfo` → 3 × `FinderPattern`, each a `ResultPoint` + `GetEstimatedModuleSize()` | gozxing `qrcode/detector` | Read by `detectionOf` (`scanner/classify.go:140`) into a box, module size and estimated version. |
| Partial centres (1–2 finders, no triple) | `FinderPatternFinder.GetPossibleCenters()` | Verified reachable and **deliberately unused** — see the note in `detectFinders`, `scanner/scanner.go:426-435`. They fire on images holding nothing. |
| Per-hit geometry, dedup by centroid, reading order | `scanner/hits.go` — `hit{text, points}`, `sameCode`, `sortHits` | **Already a geometry layer**, with a written coordinate contract (`hits.go:16-32`). New strategies must map points back to original-image space. |
| Strategy list (10: channel × binarizer × upscale) | `strategies`, `scanner/scanner.go:352` | Greedy cover, ordered, with measured recall notes. Phase 2 extends this list. |
| Evidence-based early exit | `decodeRasterDetail`, `scanner/scanner.go:473` | Stops when decoded count reaches detected count. Phase 2's ladder plugs in after it, not instead of it. |
| Path-free decode seam for wasm | `ScanDecodedImage`, `scanner/scanner.go:603`; `ScanDecodedImageDetail`, `:616` | The browser hands over RGBA, Go finds codes. The `Detail` variant carries classification, boxes and metadata, and `wasm/main.go` marshals all three — **but `web/worker.js:102-104` keeps only `classification`**. |
| Per-image annotation: dimensions, Laplacian variance, local edge density, strategies attempted | `scanner/metadata.go`; `Metadata`, `:45` | **New in Step A.** Recorded for every image, on every outcome. Annotation, never a filter. |
| Companion-mask short circuit, and the switch that ignores it | `npyMaskEnabled`, `scanner/scanner.go:511` | **New in Step A.** The benchmark ignores masks by default; see Phase 2 Step A below for why the headline throughput was never a decoder number. |
| Browser worker pool, thumbnails, filters, search, i18n | `web/worker.js`, `web/index.html`, `web/strings.js` | 6 languages, key-parity self-test (`web/i18n.js:63`). Every new string is 6 strings. Thumbnail object URLs **are** revoked (`web/index.html:1260`). |
| Ground-truth manifest, from a decode or from sidecar files | `cmd/corpusgen/main.go` (`-truth .ext`), `scanner/corpus_test.go` | Without `-truth` it is a bootstrap and says so; with it the payloads come from the dataset's own answers and the manifest is real ground truth. |
| CSP that makes upload structurally impossible | `web/index.html:10-19` | `connect-src 'self' http://127.0.0.1:* http://localhost:*`. See Phase 4 — this is the load-bearing constraint of the whole document. |

There is one more thing worth naming: the codebase already knows this work is
coming. `scanner/scanner.go:734` carries a `TODO(cascade)` saying the
Vision/zbarimg gate is where recall is traded for speed and that finder-pattern
accounting is how to close it provably. `hits.go:28` anticipates "the upcoming
tiling cascade". Phases 1 and 2 are the cash-out of comments already written.

---

# Phase 1 — Failure classification (headless, no UI) — SHIPPED

Give every image a reason, not a boolean.

```
DECODED                       at least one payload came out
QR_DETECTED_DECODE_FAILED     3 finder patterns located, nothing readable
NO_QR_FOUND                   no finder pattern in any colour projection
```

**Three buckets, not the five this section originally proposed.**
`PARTIAL_FINDERS` and `HIGH_EDGE_DENSITY_REGION` were measured and dropped;
`NO_QR_LIKELY` was renamed `NO_QR_FOUND`, since *found* is already a claim about
the search rather than about the image and the `_LIKELY` hedge was doing the
same job twice. The reasons are in "Decisions taken during implementation"
above, and the two dropped buckets are not deferred work — they are rejected
work, and re-proposing either needs new evidence, not a new sprint.

The enum lives in `scanner/classify.go:38-53`. The scope below is the shipped
scope, with the parts that did not ship marked.

## Scope

- **Shipped.** A classification enum on the per-image result, alongside the
  existing `HasQR`/`Contents`/`Error`. `Error` keeps its current meaning
  (unreadable file) and does **not** become a classification; an unreadable
  file has no classification at all.
- **Shipped.** Expose the bounding box whenever a finder-pattern triple is
  located. Coordinates in original-image space, per the contract in
  `hits.go:16-32`. This is the field Phase 3 pre-draws — once `worker.js`
  forwards it. The `PARTIAL_FINDERS` half of this bullet died with its bucket.
- **Shipped in Phase 2 Step A, not in Phase 1.** Per-image metadata, recorded
  for every image regardless of outcome: dimensions, strategies attempted,
  Laplacian variance (global blur proxy), local edge density. It was cut from
  the first slice because the sentence on the row was the whole felt value and
  the numbers had no reader; it came back the moment there was one, which is
  the benchmark. Estimated QR version shipped in Phase 1 with the box, since
  the triple yields it for free.
- **Metadata is annotation, never a filter.** Nothing is skipped, downranked
  or discarded because a number looks bad. A blurry image still runs every
  strategy. This is a hard rule: the moment a threshold gates work, the
  metadata stops describing the scan and starts causing it, and every recall
  number afterwards measures the threshold instead of the decoder.
- **Shipped.** A classification must be derivable *without* re-running
  detection. The finder pass already happens inside `detectFinders` (then
  called `countCandidates`); the change was to return the record instead of an
  `int`.
- Estimated QR version comes free from the triple: modules across ≈
  `distance(topLeft, topRight) / estimatedModuleSize + 7`, version =
  `(modules − 17) / 4`, clamped to 1–40 and reported as an estimate.
- **Shipped, with one change.** Surfaces: CLI JSON export, TXT export, the Go
  API, the local HTTP API and the wasm result. **CSV was deliberately left
  alone** — a new column breaks existing scripts that read by position, and the
  JSON export already carries the field. Adding a field is fine, changing
  `has_qr` is not, and nothing changed it.

## Non-goals

- ~~No UI. Nothing in `web/` changes in this phase.~~ **This non-goal was not
  kept.** The reason string, its stat tile, its filter tab and the row order
  shipped with Phase 1, because a classification nobody can see is not a
  classification anybody has. What stayed out of `web/` is everything with a
  canvas in it.
- No new decode attempts. Classification observes the existing cascade; it
  does not add a strategy. (That is Phase 2, deliberately separated so the
  first recall change can be measured against a classifier that already
  worked.)
- No confidence scores, no probabilities. Five discrete buckets, each with a
  stated rule. A score invites a threshold, and a threshold is a filter.
- No per-code classification. One image, one classification, plus zero or more
  decoded codes. An image with one decoded code and one undecodable one is a
  real case and is **an open question below**, not a solved one.
- No promise that `NO_QR_FOUND` means there is no QR. It means nothing was
  detected by anything we ran. The naming (`_LIKELY`) carries that.

## Cheap vs. project

**Cheap** (a day or two) — *done*:
- `QR_DETECTED_DECODE_FAILED`, the bounding box, estimated version. The
  detector already ran; `countCandidates` discarded `[]*FinderPatternInfo` and
  returned a count. It keeps the slice now, as `detectFinders`.
- Dimensions, strategies attempted. Both were already in hand at the call site.
  Landed in Step A.
- Laplacian variance: a 3×3 convolution over the grayscale projection and a
  variance. It came out at about that size, no dependency, and it shares its
  single pass with the edge measure (`measure`, `scanner/metadata.go:119`).

**Cheap but needs a check** (half a day) — *checked, then dropped*:
- `PARTIAL_FINDERS` via `GetPossibleCenters()`. `FindMulti` does populate the
  centre list before failing, so the cheap route was open. The bucket died on
  the next question instead: those centres fire on images holding no code at
  all — uniform noise yields one — so the bucket would promise a code that is
  not there. Not folded into `NO_QR_FOUND` "and said so" either; there was
  nothing to fold, because the count was never used.

**A project in itself** (a week, mostly measurement) — *not built, and the
measurement it needed now has an instrument*:
- `HIGH_EDGE_DENSITY_REGION`. "A dense-transition region" is not a definition.
  It needs: a window size, a transition-density measure, a threshold, and — the
  expensive part — evidence that the bucket is *useful*, i.e. that images
  landing in it are meaningfully more likely to hold a code than `NO_QR_FOUND`
  ones. Without that evidence it is a bucket that sends users to draw boxes
  around textured carpet.
  Step A shipped the first two — `edgeWindow` and the measure — as a **number
  on every image and no bucket at all**. That is the ordering this section
  asked for: gather the evidence, then decide whether a label is defensible.

**Estimate:** 3–5 days for everything except edge density. *Actual: the
classification slice landed in one pass; the metadata followed in Step A once
the benchmark gave it a reader.*

## Open questions

1. **Mixed images.** Two codes, one decoded, one not. Is that `DECODED`, or a
   sixth state? The `detectFinders` machinery can already see the gap
   (detected 2, decoded 1) — that is exactly what its early-exit uses. A
   `DECODED_PARTIAL` bucket is arguably the single most actionable state in the
   whole taxonomy, and it is not in the list above.
2. **Where does the classification live in the API?** A field on `ScanResult`
   changes a released public struct and the `--export json` schema. A parallel
   `ScanDetail` type keeps `ScanResult` frozen at the cost of two types
   describing one scan.
3. **Vision and zbarimg produce no finder information.** On macOS a photo can
   decode via Vision having never been seen by the gozxing detector. Is such an
   image `DECODED` with no bounding box, or does the finder pass run anyway to
   populate geometry? The second costs time on the success path.
4. **Which image does the metadata describe?** The browser decodes at a 1600px
   cap first and retries at 3000px (`MAX_FAST`/`MAX_SIDE`,
   `web/worker.js:24-25`). Laplacian variance at 1600 and at native resolution
   are different numbers. **Step A answered this for the metadata it records:
   as-scanned**, with the dimensions carried alongside so the two are never
   compared by accident. Whether the page should also report native remains
   open.
5. **Does classification run in the wasm build?** It has to for Phase 3 to
   exist, which means the seam returning it must be `GOOS=js`-safe and the
   extra work must be cheap enough not to undo the caps in `worker.js`.

---

# Phase 2 — Automatic retry ladder

Exhaust the machine before spending the user's attention.

## Step A — the harness and the baseline (SHIPPED)

No strategy was written in Step A, deliberately. A ladder without a
before/after on a fixed corpus is a guess, and the corpus, the instrument and
the baseline all had to exist before the first rung could be judged.

**The corpus.** `lovasoa/qrcode-dataset` v0.1, 3332 images, each shipping the
distorted PNG, the payload as `NNN.txt`, and the generator's bit-matrix as
`NNN.npy`. The manifest is built with `corpusgen -truth .txt`, which reads the
dataset's own answers and decodes nothing — a manifest built by decoding would
make the benchmark measure the decoder against itself, which is exactly what
the BOOTSTRAP header warns about.

**Masks are ignored by default, and that is the point of Step A.** A `.npy`
beside an image is the generator's answer key. `ScanImageDetail` reads it first
and short-circuits, so a benchmark over this dataset with masks on measures the
speed of *parsing answers* — the README's ~475 img/s headline is a number from
a path on which no QR decoding happens, and it says so, but a benchmark should
not need the caveat. `npyMaskEnabled` (`scanner/scanner.go:511`) is off in the
harness by default and back on behind `-corpus.masks`, and the run labels which
it was.

**The baseline, masks ignored** (`go test -tags corpus ./scanner -run TestCorpus`):

| build | recall | wall clock | throughput |
|---|---|---|---|
| pure Go, no Vision, no `zbarimg` (Step A) | **58.43%** (1947/3332) | 97.4 s | 34.2 img/s |
| the same, plus Step C's three rungs | 59.18% (1972/3332) | 136.6 s | 24.4 img/s |
| macOS CLI as shipped, + Vision + `zbarimg` | 75.36% (2511/3332) | 57m27s | 1.0 img/s |

The second row is the same corpus through the whole cascade. Apple Vision was
offered the 1385 images the raster path missed and read **564 of them (40.7%)**.
`zbarimg` was then offered the remaining 821 and read **none** — zero, on this
corpus — while costing a process spawn per image. That is not an argument to
remove it (it exists for photographs, and this corpus has none), but it is the
reason the 57-minute wall clock buys 17 points of recall: the time is Vision's.

That Vision, a detector rather than a binarizer, recovers 40% of what the raster
path cannot is the single most useful signal in this table for choosing rungs.

0 partial, 0 false positives, 1385 missed. This is the regression baseline: a
ladder rung is worth having when it moves 58.43% and does not move 97 s more
than it is worth. The figure is consistent with the README's independently
measured "~57% on pixels alone", which is the cross-check that the harness is
measuring what it claims to.

Per-stage hit rates from the same run, in cascade order.

> **This is a regression baseline on synthetic data. It is NOT the ladder
> ordering, and it must not be used as one.**
>
> Every image in it is a generated render — a `qrcode` library QR put through a
> random albumentations pipeline and resized to 256×256. Nothing here was
> photographed. A stage's hit rate on that distribution says what happens to
> *these renders*, and the failure mode this tool actually exists for is a
> small code in a large photograph, which the corpus contains none of.
>
> Its use is to catch a regression: if `lum/hybrid/1x` stops finding 1414 of
> 3332, something broke. Reordering the cascade from these numbers would be
> tuning against the wrong distribution, and the ordering data does not exist
> yet — it arrives with the HEIC photo set.

| stage | ran | found | hit rate |
|---|---|---|---|
| `lum/hybrid/1x` | 3332 | 1414 | 42.4% |
| `lum/global/1x` | 2009 | 427 | 21.3% |
| `lum/hybrid/2x` | 1587 | 123 | 7.8% |
| `lum/global/2x` | 1568 | 8 | 0.5% |
| `r/global/1x` | 1567 | 29 | 1.9% |
| `g/global/1x` | 1543 | 12 | 0.8% |
| `b/global/1x` | 1536 | 22 | 1.4% |
| `min/global/1x` | 1519 | 8 | 0.5% |
| `max/global/1x` | 1516 | 9 | 0.6% |
| `r/hybrid/2x` | 1512 | 105 | 6.9% |

Read "ran" as the early exit working: only the images the previous stages could
not finish reach the next one. The two 0.5% rows are the obvious thing to point
at, and pointing at them is exactly the mistake the box above warns against:
a colour-channel strategy earning nothing on synthetic renders says nothing
about what it earns on a photograph of a coloured code on a coloured background,
which is the case it was added for.

## Step B — what the misses are (SHIPPED, characterisation only)

Still no strategy code. `-corpus.dump` writes one row per image — outcome,
classification, the metadata, and the *true* QR version read out of the
dataset's own `.npy` — and this is what those rows say about the 1385 misses.

**The generator records no degradation type.** `write_sample` writes three
files: the PNG, the payload as `.txt`, and the matrix as `.npy`. The
augmentation is `Compose(p=0.9)` of blur, noise, dropout, distortion, contrast
and JPEG damage with per-transform probabilities, and the realised parameters
are discarded. Filenames are a zero-padded index. So a cross-tab of misses by
blur-versus-occlusion **cannot be produced from this dataset**, and guessing the
degradation from the pixels would be inventing the labels the experiment needs.
What *is* exact and recoverable is the QR version, from the matrix shape.

**The split, and it is not lopsided:**

| bucket | count | share of misses |
|---|---|---|
| `QR_DETECTED_DECODE_FAILED` | 850 | 61.4% |
| `NO_QR_FOUND` | 535 | 38.6% |

Both halves of the planned ladder have a real target. Neither is dead weight on
the evidence of the split alone.

**But the split moves with density, which is the finding that matters:**

| | n | recall | detected-but-unread | nothing found |
|---|---|---|---|---|
| version 1–4 | 1939 | 72.3% | 72% of its misses | 28% |
| version 15+ | 572 | 22.0% | 43% of its misses | 57% |

Recall falls monotonically with version — 77% at v1, 54% at v5, 34.6% at v16,
7% at v20, 3.4% at v21 — and the failure mode changes on the way down. A sparse
code that fails, fails at *reading*; a dense one fails at *finding*.

**Why: every image is 256×256, and the generator resizes after rendering.** The
quiet zone is 9 modules a side, so the pixels available per module are fixed by
version alone — 6.6 px at v1, 3.4 px at v10, 2.2 px at v20, 1.4 px at v36.
Measured on the detected-but-unread misses, the estimated module size has median
4.6 px and a p10 of 2.1 px, with 25% under 3 px.

**Distributions across hits and misses** (no threshold, no classifier — the
question was what the misses look like):

| measure | hits | misses | reads as |
|---|---|---|---|
| true version | med 2, p90 12 | med 8, p90 24 | the only clean separation |
| payload length | med 13, p90 224 | med 109, p90 731 | the same fact, upstream |
| Laplacian variance | med 1340, mean 3118 | med 1806, mean 5857 | **inverted, and a trap** |
| local edge density | med 0.1310, mean 0.133 | med 0.1279, mean 0.143 | no signal at all |
| source dimensions | 256×256 | 256×256 | dead axis on this corpus |

Two negative results worth keeping:

- **Laplacian variance is behaving as a density proxy here, not a blur proxy.**
  The misses score *higher*, because a dense code has more edges than a sparse
  one and that swamps whatever the blur took away. It is not wrong — it is
  measuring what it says it measures — but anyone reaching for it as "how
  blurred is this image" on this corpus will get the sign backwards. On a
  photograph, where code density and image sharpness are independent, it may
  yet behave as intended. Unverified until the photo set exists.
- **Local edge density separates nothing.** Hits and misses overlap almost
  exactly. It was added to gather evidence for or against a
  `HIGH_EDGE_DENSITY_REGION` bucket; on this corpus the evidence is against.

**One positive result:** on the 850 detected-but-unread misses the estimated
version matches the true version exactly 572 times and within ±1 774 times, and
exactly one image produced no plausible geometry. The box Phase 3 wants to
pre-draw is derived from the same triple, so its geometry is trustworthy enough
to hand to a user.

**What this corpus cannot decide.** Every image is one code, filling a 256×256
frame. There is no small-code-in-a-big-photo case anywhere in it, so tiling,
downscale and the sliding window have *nothing to be measured against here* —
they are not disproved, they are unmeasurable, and running them against this set
would produce a number that means nothing. Same for perspective correction: the
generator's `ShiftScaleRotate` is limited to ±8°, which is not the warp a
photograph of a receipt on a table has.

## Step C — three rungs, and the ones deliberately not written

Three rungs, chosen from Step B's evidence and capped at three. The rest of the
ladder waits for a corpus of photographs, because this one cannot judge it.

### Written

| rung | name in the table | target | how it was judged |
|---|---|---|---|
| Sauvola adaptive threshold | `lum+sauvola/global/1x` | the 850 detected-but-unread | recall delta on lovasoa, per version |
| Otsu global threshold | `lum+otsu/global/1x` | same | recall delta on lovasoa |
| Inversion | `lum+invert/hybrid/1x` | light-on-dark codes | **synthetic fixtures, not a corpus number** |

All three are pixel transforms rather than gozxing `Binarizer` implementations
(`scanner/binarize.go`). A transform that outputs a two-valued image has
already made the decision a binarizer would make, so it can be handed to the
cheapest one and pass through — which keeps each rung a pure function over
pixels, testable on its own.

They are **appended** to the strategy table, after the measured ten. An image
that already decoded therefore takes exactly the path it took before, and the
recall delta is attributable to the new rungs and to nothing else.

**Inversion ships on correctness, not on measured recall, and that is stated
rather than glossed.** lovasoa contains no light-on-dark codes, so its recall
delta on this corpus is exactly zero and would say nothing about whether the
rung works. What the fixtures establish is stronger than a delta anyway: an
inverted code produces **zero finder-pattern triples**
(`TestInvertedQRIsUndecodableUntilInverted`), so it is invisible to the
detector rather than merely unreadable, and no binarizer choice recovers it.
One subtraction per pixel does.

### Otsu was checked for redundancy before being written

`GlobalHistogramBinarizer` is already a global threshold, so the question was
whether Otsu duplicates it. Reading the source
(`gozxing/global_histogram_binarizer.go`): it builds a **32-bucket** histogram
from **four sampled rows** — `height*y/5` for y in 1..4 — across the middle 60%
of each, takes the tallest peak, takes a second peak weighted by squared
distance, and picks a valley between them by a heuristic score.

That is a peak-valley heuristic, not Otsu's criterion, and the two disagree in
four specific places:

1. **Four rows versus every pixel.** A code that does not lie across those rows
   is thresholded from a histogram it barely appears in.
2. **Five-bit versus eight-bit search.** 32 candidate thresholds versus 256.
3. **A valley heuristic assumes bimodality.** This dataset renders codes under
   vertical, radial and horizontal colour gradients, which smear it; between-class
   variance still has a maximum where no valley is visible.
4. **It gives up.** When the two peaks land within two buckets it returns
   `NotFound` and nothing is thresholded at all. Otsu always returns a
   threshold — on a low-contrast image, the difference between a bad attempt
   and no attempt.

So: not redundant, and written. Whether it earns its place is the measurement,
not the argument.

### What the three rungs actually bought

Same corpus, same build, masks ignored, pure Go, nothing else changed:

| | before Step C | after Step C |
|---|---|---|
| per-code recall | 58.43% (1947/3332) | **59.18%** (1972/3332) |
| wall clock | 97.4 s | 136.6 s |
| throughput | 34.2 img/s | 24.4 img/s |
| regressions | — | **0 images lost** |

**+0.75 points for 1.40× the time.** Against the standing rule — *a strategy
buying under 1 point while costing more than 2× is reported and left out* — the
three together clear the cost half of the bar and fail the recall half, so they
stay. That is the rule applied, not the rule dodged: it takes both.

Of the 25 images gained, **Otsu found 13 and Sauvola 12**. Inversion found
**zero**, exactly as predicted, on a corpus with no light-on-dark codes; it
costs roughly a third of the added 39 seconds and buys nothing measurable here.
It ships anyway, because the fixtures show it recovers a case this corpus does
not contain.

**The predicted shape did not appear.** The expectation was that a local
threshold would win at low version and vanish at high, since a dense code fails
for want of pixels rather than for want of contrast. The measurement says
otherwise:

| | n | before | after | delta | images |
|---|---|---|---|---|---|
| version 1–4 | 1939 | 72.3% | 72.9% | +0.62 pts | 12 |
| version 5–9 | 512 | 54.9% | 55.7% | +0.78 pts | 4 |
| version 10–14 | 309 | 45.0% | 45.0% | **+0.00 pts** | 0 |
| version 15+ | 572 | 22.0% | 23.6% | **+1.57 pts** | 9 |

The largest proportional gain is at version 15 and above — the densest codes,
2.7 px per module and below — and the band that gains nothing at all is the
middle. Nine images is a small number and the per-version rows are noisy at
that scale, so this is not a law. It is enough to say the "wins at low version,
vanishes at high" story is not what happened, and nobody should plan the next
rung on it.

**The more useful surprise: 14 of the 25 gained images were `NO_QR_FOUND`
before, and 11 were `QR_DETECTED_DECODE_FAILED`.** A better threshold did not
merely make readable codes readable — on the majority of its wins it made
codes *detectable*, by giving the finder-pattern scan a cleaner image to work
on. That blurs the tidy split Step B drew between "detected-but-unread wants
binarization" and "not-found wants detection": preprocessing feeds the
detector too. It is also a warning about ordering the ladder by bucket, which
Step B's split invited.

**Reading the per-stage `found` column correctly.** It counts attempts that
found *something*, not marginal recall. Sauvola's 78 and Otsu's 59 are mostly
codes earlier stages had already read: when `detectFinders` locates nothing the
early exit is disabled entirely, so every stage runs even on an image that has
already decoded, and every stage that also reads it is counted. The marginal
number is the images-gained column above — 12 and 13 — and it is an order of
magnitude smaller. This applies to the whole per-stage table, including the
original ten.

### Not written, and why

- **4× upscale.** Nearest-neighbour cannot add detail a 256×256 render never
  had, and the existing 2× rungs are already the first stage to find anything on
  only 2.6% and 0.2% of hits. There is no mechanism by which more of the same
  helps.
- **Sharpening.** No measurable target on this corpus. It would be added on
  intuition and measured against a distribution that cannot test it.
- **`HIGH_EDGE_DENSITY_REGION` — dropped permanently, as a negative result.**
  Local edge density on 3332 images: hits median **0.1310**, misses median
  **0.1279**; means 0.133 and 0.143; the whole interquartile range overlaps.
  The bucket was always conditional on evidence that images landing in it are
  likelier to hold a code, and the evidence is against. The **metric stays** as
  annotation on every image — it costs one shared pass and it may behave
  differently on photographs — but the proposed bucket is closed, not deferred.
  Re-proposing it needs new evidence from a different corpus, not a new sprint.
- **Tiling, sliding window, downscale, perspective correction.** Unmeasurable
  here, **not disproved**. Every lovasoa image is one code filling a 256×256
  frame, so there is no small-code-in-a-big-photo case to measure a tiling
  strategy against, and `ShiftScaleRotate` is capped at ±8°, which is not the
  warp a photographed receipt has. These are the rungs that matter for
  photographs and they wait for the real set.

### Nothing was removed

The sub-1% stages — `lum/global/2x`, `min/global/1x`, `max/global/1x` — stay.
Throughput is 34 img/s and nobody is waiting on it; recall is the constraint.
A stage earning 0.5% on synthetic renders is not evidence about what it earns
on a photograph of a coloured code on a coloured background, which is the case
it was added for.

## Findings that outlive this corpus

**Laplacian variance is a density proxy here, and its sign is inverted against
the intuitive reading.** Misses score *higher* than hits — median 1806 against
1340, mean 5857 against 3118, p90 19064 against 8351. A dense code has more
edges than a sparse one and that swamps whatever the blur removed. The metric
is not wrong; it measures what it says it measures. But anyone reaching for it
as "how blurred is this image" on this corpus gets the sign backwards. On
photographs, where code density and image sharpness are independent, it may
behave as intended. **Unverified until the photo set exists**, and it should not
be used for anything before then.

**The remaining recall lives in detection, not in preprocessing, and that is
the ceiling of the zero-cgo path.** Apple Vision was offered the 1385 images
that ten pure-Go channel and binarizer combinations could not read, and it read
**564 of them — 40.7%**. `zbarimg` was then offered the remaining 821 and read
**none**. A detector recovering two fifths of what every binarizer in the
cascade cannot is the plainest statement available of where the ceiling is: it
is not a thresholding problem. Sauvola and Otsu are worth adding because they
are cheap and because some of the 850 detected-but-unread are genuinely
threshold failures, but no stack of preprocessing rungs reaches what a learned
detector reaches. The zero-cgo path tops out below Vision, deliberately, and
the roadmap should stop implying otherwise.

**A second corpus, for real photographs.** `QR_CORPUS_DIR` takes any directory
with a manifest, HEIC included: paths are opened by name and nothing filters by
extension. What Step A added is a guard — a build that cannot read HEIC (no
`-tags heic`, and not macOS-with-cgo) now *skips* such a corpus instead of
scoring it 0%, because a number that describes the build reads like a verdict
on the decoder. The manifest for a photo set is hand-written: the payloads are
read off the codes by a person or another decoder, never by this one.

## Scope

Extend the strategy set with, roughly in the order they are worth trying:

| Rung | Note |
|---|---|
| Otsu binarization | **Written in Step C.** Global, one pass. The existing global-histogram binarizer turned out to be a peak-valley heuristic on a four-row sample, not Otsu — the difference is spelled out in Step C. |
| Adaptive (Sauvola) | **Written in Step C.** Local threshold. The real win on uneven lighting and glare. |
| Inversion | **Written in Step C**, on synthetic fixtures rather than a corpus number: the benchmark set holds no light-on-dark codes. |
| 2× / 4× upscale | **4× dropped in Step C.** Nearest-neighbour cannot add detail a 256×256 render never had, and the 2× rungs already first-find only 2.6% and 0.2%. |
| Downscale | **Waiting for the photo set.** Unmeasurable on a corpus of 256×256 renders. |
| Sharpening | **Dropped in Step C**: no measurable target on this corpus. |
| Perspective correction | **Waiting for the photo set.** The generator's warp is capped at ±8°, which is not a photographed receipt. |
| Sliding-window grid scan, with overlap | **Waiting for the photo set.** Every image here is one code filling the frame, so there is nothing for a tiling strategy to find. |

- The ladder runs after `decodeRaster`'s union has failed, before the user is
  ever told anything. It is not a user-visible feature; the user sees a higher
  success rate.
- **Log which strategy succeeded**, per image, so the ladder can be reordered
  later from real hit rates rather than from intuition. This is the point of
  the phase as much as the recall is. Record it even when the winner is an
  existing strategy, so the baseline is comparable.
- Every tile in the grid scan must add its offset back to result points *at
  capture time*, inside the attempt — not at compare time. This is not style;
  `hits.go:16-32` explains that violating it silently reports one code twice
  and no payload-only test catches it. Overlapping tiles find the same code
  repeatedly **by design**.
- Allocation failures must cost recall, not the scan. `scanner/scanner.go:712`
  already states this as the contract these stages land into.

## Non-goals

- No ML detector. WeChat/CNN means an OpenCV C dependency, which is the thing
  this tool exists to avoid (README, *Benchmark*). The classical ceiling on
  the hard set is ~28% (gozxing) / ~30.3% (zbarimg) across the union of every
  channel, binarizer and scale; the ladder is chasing the gap up to that
  ceiling, not past it. **Say so in the doc, not only here** — a roadmap that
  implies the ladder will fix the 70% is lying.
- No unbounded search. The unbounded strategy union was measured at 122.6s on
  40 photos versus 18.4s with the finder pre-pass (README). Every rung needs a
  stopping rule or the ladder eats the product.
- No reordering of the existing 10 strategies in this phase. They carry
  measured justifications; changing them and adding rungs at once makes the
  measurement unattributable.
- No user-facing controls. No "try harder" button. Either a rung is worth
  running or it is not in the ladder.

## Cheap vs. project

**Cheap** (a day or two each): Otsu, inversion, sharpening, up/downscale. Each
is a `decodeStrategy` variant plus a pixel transform; the harness for "run a
transform, decode, map points back" exists in `decodeAttempt`
(`scanner/scanner.go:236`), and every attempt is now recorded by name in the
per-image metadata, so a new rung is measurable the day it lands.

**Moderate** (3–5 days): Sauvola. An integral-image implementation is standard
and pure Go, but window size and the *k* parameter need tuning against the
corpus or it will lose to the existing hybrid binarizer.

**A project in itself:**
- **Perspective correction** (1–2 weeks). Homography from three finder
  patterns, sampling the fourth corner, resampling, then mapping every result
  point *back* through the inverse transform to satisfy the coordinate
  contract. The inverse mapping is where this will break.
- **Grid scan with overlap** (1–2 weeks). Tile size and stride are a recall/time
  trade with no obvious answer, dedup pressure goes up sharply (every code in
  an overlap region is found n times), and it multiplies the cost of every
  other rung it composes with. This is the rung most likely to make the
  product feel slow. It is also the one that finds small codes in big photos,
  which is the actual failure mode of a camera roll.

**The measurement is not optional and is not free.** A ladder without a
before/after on a fixed corpus is a guess. *Step A settled half of this*: the
generated corpus, the mask-free default and the per-stage instrumentation are
in, with a baseline of 58.43% / 97.4 s. The other half stands — a corpus of
real photographs with ground truth still does not exist, and hand-labelling one
is a week of dull work that must be budgeted. The generated set is adversarial
renders, not camera rolls, and a rung tuned only against it is tuned against
the wrong distribution.

**Estimate:** 2 weeks for the cheap and moderate rungs with measurement; +3–4
weeks for perspective correction and tiling.

## Open questions

1. **Where does the ladder run — CLI, wasm, or both?** In the browser it
   competes with the caps in `worker.js` that make the page feel fast. A ladder
   that triples the time on every code-less image is a regression the user
   feels on a folder of holiday photos.
2. **Does the ladder run automatically, or only on the images Phase 1 flagged
   as `QR_DETECTED_DECODE_FAILED`?** *Step C's answer is: automatically, for
   everything.* The three rungs are appended to the one table and run on any
   image the earlier stages did not finish, with no consultation of the
   classification. That keeps the "metadata never filters" rule intact and
   keeps the recall delta attributable. It also means a code-less photo pays
   for three more attempts, which is the cost this question named and which the
   measurement below prices. Gating on
   classification is exactly the "metadata as filter" that Phase 1 forbids —
   but running an expensive ladder on every landscape photo is the cost that
   made the caps necessary. This tension is real and unresolved. One defensible
   line: *classification may order the ladder, never truncate it.*
3. **`ScanFast` vs `ScanExhaustive`.** Is the ladder a third mode, or does it
   extend `ScanExhaustive`? Its docstring already says exhaustive is "for
   building ground truth, where being right matters more than being quick" —
   which is precisely the ladder's job description.
4. ~~**How is the strategy log surfaced?**~~ **Answered in Step A: a field in
   the JSON export**, `metadata.strategies`, additive and `omitempty`, mirrored
   in the local API and the wasm result. The risk this question named is real
   and now live: diagnostic data in a user-facing export tends to become an
   accidental API, and an image that decodes on the first strategy carries one
   entry while one that fails carries ten. If that becomes a problem it is a
   size problem in the export, not a reason to stop recording — the recording
   is what makes the ladder falsifiable.
5. **Does a rung that only ever wins in combination get to stay?** The existing
   list is a greedy cover chosen by measured marginal recall. New rungs need
   the same bar, or the list becomes folklore.

---

# Phase 3 — Web UI triage

Let the user resolve what the machine could not, for a batch of ~100 images.

## Scope

- **Batch view**, sized for ~100 images. The list already exists
  (`<ul id="list">`, `web/index.html:393`); this is a change to how it is
  ordered and what each
  row says.
- ~~**Default sort is by classification, most-actionable first**~~ — **shipped
  with Phase 1**, and with the buckets that exist rather than the five this
  section was written against. The order is
  `QR_DETECTED_DECODE_FAILED` → `NO_QR_FOUND` → decoded → unopenable →
  still-scanning (`rowRank`, `web/order.js:38-43`), it is implemented as a CSS
  `order` on the row rather than by reordering DOM nodes
  (`web/index.html:1025`), and it is pinned by `web/order.js --test` against
  the terminal's deliberately different order. The reasoning stands: a user
  with 100 images and 20 minutes should spend them where their attention
  converts. Nothing in Phase 3 should change this order; a sort *control* is
  still open (question 4).
- ~~**One-line reason per row**~~ — **shipped with Phase 1**, in six languages,
  enforced by `I18N.selfTest()` (`web/i18n.js:63`). "part of a QR — it may be
  cut off" was never written, because the bucket behind it was dropped. Only
  the actionable state carries a second line; `No code found` stands alone.
- **Manual region selection on a canvas**: the user drags a rectangle over the
  image.
- **When Phase 1 produced a bounding box, pre-draw it.** The user adjusts an
  existing rectangle instead of drawing one from scratch. This is the single
  highest-leverage detail in the phase — it turns the interaction from "find
  the code and box it" into "nudge this box", which is roughly the difference
  between a feature people use and one they try once.
- **The selected region is re-run through the Phase 2 ladder**, at native
  resolution — not through the 1600px fast path. The user has already told us
  where to look and paid attention for it; spend the time.
- The result of an assisted decode is a normal decoded code: same row, same
  copy button, same payload recogniser (`web/payload.js`), same exports. It
  should be indistinguishable downstream, except in the strategy log.

## Non-goals

- No editing tools. No rotate, no brightness slider, no manual threshold. Every
  one of those is a rung of Phase 2 that the user should not be doing by hand;
  if a transform helps, it belongs in the ladder.
- No manual transcription. If the user can read the digits under the code, that
  is not this tool's job.
- No cropping-and-saving of images. The browser cannot touch the disk and
  should not pretend otherwise (README, *Web app*).
- No server round-trip. Assisted decode runs in the same wasm engine as
  everything else. Nothing is uploaded in this phase — that is Phase 4, and it
  is separate precisely so this phase inherits none of its problems.
- No queue/workflow/assignment features. This is one person, one batch, one
  sitting.

## Step A — re-open the seam (SHIPPED, no UI)

The bug this step existed for: the Go side located codes it could not read,
computed a box for each and marshalled them across the wasm boundary, and the
worker copied `res.classification` into `out.reason` and dropped
`res.detections`. Nothing errored. The page simply never had the geometry, and
the pre-drawn box — the highest-leverage detail in this whole phase — was
unbuildable from data that never arrived.

What changed:

- **One shape on every surface.** `wasm/main.go` was flattening `x/y/w/h` into
  the detection while its own docstring claimed the shape matched the CLI export
  and the local API, which marshal a nested `box`. It now emits the nested box,
  so `scanner.Detection` looks the same wherever it is read.
- **`web/result.js`**, a named mapping with a self-test, wired into CI beside
  `order.js`. The hop is a function rather than three assignments in a worker
  because a worker cannot be loaded by node — no `importScripts`, no `self` —
  which is precisely why the original drop was never caught.
- **The frame travels with the boxes.** The page decodes at a 1600 px cap and
  retries at 3000, so a box can be expressed in a shrunk image's coordinates.
  Every row now carries `frame: {w, h, naturalW, naturalH}` — the pixel buffer
  the decode actually ran on, and the file's own size. Geometry without its
  coordinate space cannot be drawn on anything, and only the worker still knows
  the factor.
- **Malformed geometry is dropped, never repaired.** A half-read box would be
  drawn somewhere wrong, and a box in the wrong place is worse than no box: it
  tells the user the tool found something where it did not.

**There is one ingest path, not two.** The `--serve` page and the hosted demo
are the same file: `/api/scan` drives the local disk actions and updates
counters, and it creates no rows. Every row on both surfaces comes from the
worker, so fixing the worker fixed both.

**The CSP was not touched**, and nothing here came near it. `result.js` is
same-origin, so `script-src 'self'` covers the page's `<script>` and
`worker-src 'self'` covers the worker's `importScripts`. No `connect-src`
change was needed or made.

**Verified against the real engine, not just the mapping.** The Go wasm build
was driven under node with the raw pixels of a corpus image that classifies
`QR_DETECTED_DECODE_FAILED`, and the row came back holding
`box {x:54, y:25, w:200, h:185}`, `moduleSize 8.93`, `version 1`, in a 256×256
frame.

**What the page now has in hand, per row:** `detections[]` — each a `box` in the
decoded frame's pixels, a `moduleSize`, and an estimated `version` — plus the
`frame` those pixels belong to. Nothing renders any of it. The next step is the
canvas, and it starts with three coordinate spaces to reconcile: the frame the
decode ran on, the file's own pixels, and whatever size the canvas is displayed
at. EXIF orientation is a fourth and is not yet accounted for anywhere.

## Cheap vs. project

**Cheap** (1–2 days each):
- Sorting by classification. The model is a plain array (`rows`,
  `web/index.html:451`); the render already exists.
- The one-line reason per row. A string per bucket, ×6 languages.

**Moderate** (3–5 days):
- Canvas region selection with a draggable/resizable rectangle. Pointer events,
  handles, touch. Well-trodden, but touch targets and mobile are where it eats
  the time.
- Plumbing geometry from Go to JS. `ScanDecodedImage` returns `[]string`
  (`scanner/scanner.go:603`) and `wasm/main.go` marshals exactly that.
  `ScanDecodedImageDetail` (`:616`) already returns the structured result and
  wasm already marshals it — what is missing is the last hop, `worker.js`. A
  structured
  result — codes plus classification plus box — means a new exported Go
  function, a new wasm signature, and a new message shape in `worker.js`. Not
  hard, but it touches every layer, and the existing seam's docstring warns
  against changing a released signature.
- Coordinate mapping. The box comes from a decode that may have run on a
  shrunk image, is displayed on a canvas at yet another scale, and is sent back
  for a native-resolution re-decode. Three coordinate spaces, one of them
  device-pixel-ratio dependent. This is the most likely source of "the box is
  slightly off" bugs.

**A project in itself:**
- **~100 images in a batch view without the page dying** (1 week). 100 rows
  with 100 object-URL thumbnails is already the current design and it holds;
  100 rows *plus* a full-resolution canvas per opened image is not. Needs one
  canvas, opened on demand, and disciplined `URL.revokeObjectURL`. **The
  thumbnail URLs are revoked** — `web/index.html:1260`, on clear; an earlier
  draft of this line said they were not, and it was wrong. A full-resolution
  canvas is still new lifetime to manage, and it is the part with no precedent
  in the page.
- **The interaction itself, done well** (1–2 weeks). Adjusting a box on a photo
  on a phone, with a keyboard alternative for accessibility, in six languages,
  on a page whose current interaction vocabulary is buttons and filters.

**Estimate:** 2–3 weeks for a competent version; 4+ if mobile and
accessibility are held to the standard of the rest of the page.

## Open questions

0. ~~**Before either: the box does not reach the page.**~~ **Done, Step A.** The
   row now carries `detections` and the `frame` they are measured in.
1. **Does the pre-drawn box come from Phase 1's detection, or from a fresh
   detection at full resolution when the row is opened?** The first is free and
   possibly wrong by the shrink factor; the second is right and costs a second.
   Note the metadata now carries the dimensions the decode actually ran at, so
   the shrink factor is recoverable rather than guessed.
2. **What happens when the user's region still fails?** A third state — "I
   looked where you told me and still could not read it" — needs its own copy,
   and it is the moment Phase 4's offer would most naturally appear. That
   adjacency is convenient and slightly dangerous: an offer to upload lands
   exactly when the user is most frustrated.
3. **Multiple regions per image.** A receipt with three codes, two decoded.
   Does the user get to draw one box or several?
4. **Is the sort order a preference?** The default is opinionated. Users
   comparing against their folder will want upload order back; a sort control
   is a small feature that mostly defends the default.
5. **Does the local-app mode (`--serve`) change any of this?** With the binary
   running, the page gains disk actions (`server.go`). Assisted decode does not
   need them — but a full-resolution re-decode could run natively (with Vision
   on macOS) instead of in wasm, which is materially better and forks the
   implementation.
6. **What is the batch-level exit?** After triage, is there a "re-run
   everything I fixed" step, or is each row resolved independently?

---

# Phase 4 — Optional contribution

> **Decision: not being built. Contribution is out of scope.**
>
> No upload endpoint, no hosted backend, and the web app's
> `connect-src 'self' http://127.0.0.1:* http://localhost:*` stays exactly as it
> is. The claim in the README — that nothing is uploaded and that this is
> *enforced rather than promised* — keeps meaning what it says, for every
> visitor, with no exception carved into it for the ones who opted in.
>
> A failing case still has a route in, and it is the route the project already
> has: run `corpusgen` over the images, get a manifest with the undecoded ones
> pre-labelled `EXPECTED_FAIL`, correct them by hand, open a pull request. That
> route asks more of the contributor and gives more back — a labelled case with
> a person attached to it, rather than an anonymous crop nobody can interpret.
>
> This also disposes of the phase's real costs rather than deferring them: no
> server to run, no retention policy to enforce, no deletion tokens, no lawful
> basis to establish, no unlabelled bucket filling up. The section below is kept
> as the record of what was considered and why it was not worth it.

Let a user who wants to help send the crop that defeated the decoder.

**Off by default. Opt-in. Per image.** Never per batch, never remembered as a
global preference that silently applies to the next folder.

## Scope

- Upload **the cropped region only**, never the full image. The crop is what
  the user selected in Phase 3 — they chose its bounds, which is most of what
  makes this defensible.
- **Strip EXIF client-side, before upload.** GPS above all, but also device,
  timestamps, serials. Client-side is not a detail: stripping server-side means
  the data was transmitted, and a promise not to keep it is not the same as not
  receiving it. The crop should be re-encoded from canvas pixels, which drops
  metadata by construction rather than by a parser that must be right about
  every tag.
- **Show an exact preview of what will be sent, before confirming.** Not a
  description of it — the actual bytes rendered back: this image, these
  dimensions, and the exact list of any accompanying fields. If a field cannot
  be shown, it is not sent.
- **Consent copy states the purpose as: improving detection AND training
  models.** Both, upfront, in the same sentence, in all six languages. Not
  "and related purposes", not "including but not limited to". The purpose
  stated at collection is the ceiling on use — it cannot be broadened later,
  because the people who consented cannot be re-asked. If model training might
  ever happen, it is named now or it never happens.
- **Do not state a retention period unless it will actually be enforced.** A
  written "deleted after 90 days" with no job that deletes anything is a false
  statement to every contributor. Either build the deletion and state it, or
  state honestly that there is no fixed retention.
- **A random deletion token per upload, displayed to the user.** Enough entropy
  to be unguessable; it is the sole credential. Presenting the token deletes
  the crop. This makes deletion possible *without collecting a single
  identifying field* — no account, no email, no device id. The corollary must
  be stated plainly next to it: lose the token and the crop cannot be deleted,
  because there is nothing else tying it to anyone.
- **No IP logging.** Which is a property of the *server* — it means an access
  log configured off, not a policy page. Whatever host is chosen must be able
  to prove this, and if it cannot, the claim does not get made.
- Rate limiting without IP logging is a genuine design problem and belongs in
  the build, not in the small print. (Proof-of-work, a per-session budget, or
  accepting abuse risk — pick one deliberately.)

**Contributed crops have no ground truth.** The decode failed; that is why the
crop exists. Nobody knows what the code says — possibly not even the
contributor. So a contributed crop is not training data on arrival: it is an
unlabelled sample requiring manual labelling before it is usable for anything,
including the "improving detection" purpose. The pipeline the repo already has
matches this exactly — `corpusgen` writes undecoded images as commented-out
`EXPECTED_FAIL` rows "ready to be uncommented and corrected by hand"
(`cmd/corpusgen/main.go:1-14`). Budget the human labelling, or the corpus is a
pile of pictures.

## Non-goals

- No account, no login, no email. The deletion token exists so that identity
  never has to.
- No analytics, no telemetry, no "anonymous usage statistics" travelling
  alongside. One upload, one crop, one token.
- No automatic contribution, no "help improve" default-on toggle, no
  post-facto opt-in for images from before consent was given.
- No full images, ever, no matter how much easier it would make labelling.
- No third-party storage that logs on our behalf and calls it infrastructure.
- No retroactive purpose change. If the purpose needs to grow, it grows for
  crops collected *after* the new copy ships.

## The constraint that dominates this phase

The hosted app's CSP is:

```
connect-src 'self' http://127.0.0.1:* http://localhost:*;
```
(`web/index.html:16`, unchanged)

and the README sells exactly that: *"Nothing is uploaded, and that is enforced
rather than promised... it cannot send a filename or a payload anywhere else
even if the page were tampered with."*

**Adding an upload endpoint to that page means widening `connect-src` for every
visitor, including everyone who will never contribute.** The enforcement
becomes a promise again — for all users — the moment the header changes. That
is not a copy problem, it is the product's main privacy claim.

Options, none free:

1. **A separate page/origin for contribution.** The scanner keeps its CSP and
   its claim; contribution is a different page the user navigates to
   deliberately, with its own CSP naming exactly one endpoint. Costs: a second
   surface, and the crop must cross to it.
2. **Contribution only in the local app** (`--serve`), where a binary the user
   installed does the upload. The CSP already allows loopback, so the hosted
   page is untouched. Costs: contribution requires installing the CLI, which is
   most of the audience gone — arguably fine, since the contributing user is
   the invested one.
3. **Widen the hosted CSP and rewrite the claim honestly.** Cheapest to build,
   most expensive to the product.

**This choice gates the phase and should be made before anything is designed.**
Option 2 is the smallest thing that does not damage anything already shipped.

## Cheap vs. project

**Cheap** (1–2 days each): crop extraction and re-encode from canvas (EXIF
dies by construction); the exact-preview screen; generating and displaying a
token.

**Moderate** (3–5 days): the consent flow and its copy, in six languages —
short, but it is the text that has to be right, and it should be reviewed by
someone who is not the person who wrote the feature.

**A project in itself — this is the real cost of Phase 4:**
- **Server infrastructure that does not exist.** There is no remote backend.
  `server.go` is a loopback server on the user's own machine with an
  origin-and-token guard (`server.go:25-37`, `isAllowedOrigin` at `:111`) —
  it is not a service. Phase 4
  introduces the project's first hosted component: an endpoint, storage, the
  deletion path keyed by token, a host configured not to log IPs, abuse
  handling without IPs, and someone on the hook when it breaks. Call it 2–4
  weeks to build and *ongoing* to run. GitHub Pages is static and cannot host
  any of it.
- **Labelling.** Ongoing human work, forever, proportional to contributions.
  Unlabelled crops accumulating in a bucket is the default outcome of skipping
  this.
- **Legal and licensing.** Contributed crops are user photographs — possibly of
  receipts, tickets, other people's documents. Under what licence or grant are
  they given? What is the answer when a contributor asks what "training models"
  meant? For an EU-based maintainer this touches GDPR (lawful basis, purpose
  limitation, data-subject rights — the deletion token is a partial answer to
  one of them). This needs a real answer, and it is not one to improvise from a
  roadmap.

**Estimate:** 4–6 weeks of build, plus ongoing operational and human cost, plus
a legal question that should be answered before the first line of it.

## Open questions

1. **Which CSP option?** Blocks everything else in the phase.
2. **What accompanies the crop?** Nothing at all is cleanest but nearly useless
   for improving detection — the classification, the strategies attempted, and
   the metadata from Phase 1 are what make a crop diagnostic. Every field is a
   field that must appear in the preview and be justified in the consent copy.
   The minimum defensible set is worth deciding explicitly.
3. **Can a crop be identifying even after EXIF is stripped?** A crop of a
   boarding pass or a payment QR can carry a name, a reference number, or a
   payable code in the pixels. The user sees the preview and chooses — but the
   copy should say plainly *look at what you are sending*, and the possibility
   that a QR crop contains a live payment credential deserves its own line.
4. **Retention, honestly.** Is there a deletion job, or is the honest statement
   "kept indefinitely, delete it yourself with your token"? Pick before writing
   any copy.
5. **Does the token also allow *viewing* the crop?** Useful for a contributor
   deciding whether to delete; another endpoint and another way to leak.
6. **What happens when a contributed crop is later decoded** — by a labeller,
   or by a better ladder? It becomes ground truth, which is the entire value.
   Is the contributor told? (Telling them requires a channel that does not
   exist by design.)
7. **Is a corpus of only-hard-cases even the right training set?** Contributed
   crops are, by selection, the images that fail. A set of exclusively hard
   negatives is a known way to build something that is worse on the easy cases
   nobody sent.

---

# Smallest useful slice — shipped

**Phase 1's classification plus the bounding box, surfaced as a reason string in
the existing web UI list.** What landed:

- `detectFinders` (was `countCandidates`) returns the `[]*FinderPatternInfo` it
  always computed instead of a bare count.
- `scanner.Classification` with three values, `Detection` carrying a box, module
  size and estimated version, and `Detail` tying them together. `ScanResult`
  gains two additive `omitempty` JSON fields; every existing key is untouched.
- `ScanImageDetail` / `ScanDecodedImageDetail` alongside the old entry points,
  which keep their signatures and delegate.
- CLI: `[QR?] photo.png  QR detected, couldn't decode` in the list,
  `2 unread │ 1 no code` as separate figures in the summary, `QR UNREAD` + a
  `reason:` line in the TXT export. CSV was deliberately left alone — a new
  column breaks existing scripts, and the JSON export already carries the field.
- Web: the reason as a translated sentence in all six languages, its own stat
  tile and filter tab, and the actionable images sorted to the top. Only the
  actionable state carries a second line: `No code found` stands alone, because
  *found* is a claim about the search rather than about the image, and an advice
  line under it restated the label without offering anything to do.
- The local API grew `detected` beside `without_qr`, so no client has to merge
  the two states back together to render a number.

**Then Phase 2 Step A, which is measurement and not recall:**

- `corpusgen -truth .ext` labels a manifest from the dataset's own answers
  instead of from this decoder, so the benchmark measures accuracy rather than
  self-consistency.
- The harness ignores companion `.npy` masks by default (`npyMaskEnabled`), and
  labels which mode a run was in. With masks on, the number is the speed of
  parsing an answer key.
- `Metadata` on every result: dimensions, Laplacian variance, local edge
  density, and every decode stage that ran with what it found. Additive
  `omitempty` JSON, in the CLI export, the local API and the wasm result.
- A guard that skips a HEIC corpus on a build that cannot read HEIC, rather
  than reporting the build's limitation as the decoder's score.
- **Baseline: 58.43% per-code recall, 97.4 s over 3332 images**, pure Go, masks
  ignored. Every Phase 2 rung is measured against that pair of numbers.

The original argument, unchanged:

1. `countCandidates`, now `detectFinders` (`scanner/scanner.go:405`), returns
   the `[]*FinderPatternInfo` it already computes instead of a count.
2. A classification is derived from what the cascade already did: finders found
   but nothing decoded → `QR_DETECTED_DECODE_FAILED`; nothing found →
   `NO_QR_FOUND`. Two buckets beyond `DECODED`, not five.
3. That reason crosses the wasm seam and appears as one line on the row.

**Why this is the slice:**

- It is the actual complaint. "No QR found" on a photo of an unmistakable QR
  code is the thing that makes the tool feel broken. "QR detected, couldn't
  decode" is the same failure and a completely different experience: it tells
  the user the tool saw what they see, and it tells them retaking the photo
  might work — which is a recourse, delivered without any UI to select regions
  and without a single byte leaving the tab.
- It costs days, not weeks. The detection runs today; the phase is largely
  about not discarding its output.
- It is the input to everything else. Phase 3's pre-drawn box is this bounding
  box. Phase 2's ordering wants this classification. Phase 4 is unreachable
  without both. Nothing downstream is wasted, and nothing downstream is
  required.
- It touches no public promise. No CSP change, no upload, no new dependency,
  no new hosted anything.

Cut even further if needed: **the two-bucket reason string with no bounding
box** is most of the felt value, because the value is in the sentence, not in
the rectangle. The rectangle only starts paying when there is a canvas to draw
it on.

What is deliberately *not* in the smallest slice: the retry ladder (weeks, and
capped by a measured classical ceiling), region selection (a real UI project),
and contribution (a server, a legal question, and an ongoing obligation).
