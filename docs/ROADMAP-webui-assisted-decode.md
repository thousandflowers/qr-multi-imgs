# Roadmap — assisted decode: classification, retry, triage, contribution

**Scope:** what happens to an image the decoder cannot read.

**Status:** the *smallest useful slice* is implemented — three buckets, the
bounding box, and the reason surfaced in the CLI and the web app. Phases 2 and 3
are still design only.

**The box is computed and returned, not drawn.** It reaches the JSON export, the
local API and the wasm result, and nothing renders it: the thumbnail overlay was
built and then removed, because an overlay is the first step of the retry flow
in Phase 2 and shipping it alone put a picture on screen with no action behind
it. Phase 2 is where it earns its place.

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

## The problem

`scanner.ScanResult` (`scanner/scanner.go:29`) is binary:

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
file. It does not mean *there is a code here and I failed*. Those two collapse
into `HasQR: false`, and so does *this is a photo of a cat*. The web UI inherits
the collapse: its stat tiles are `with / without / unreadable`
(`web/index.html:363-368`), and "without" is the bucket where a receipt whose QR
is glare-blown and a landscape photo sit together.

That is the whole gap. On the hard subset of the benchmark dataset ~70% of
images fail image-only decoding (README, *Benchmark*), and every one of them
tells the user the same nothing. The user has no recourse: no reason, no
retry, no way to point at the code they can see with their own eyes.

## What already exists (and what the phases are actually adding)

Read this before estimating anything — a good part of Phase 1 is *not deleting
data the code already computes*.

| Thing | Where | State |
|---|---|---|
| Finder-pattern pre-pass across every colour projection | `scanner/scanner.go:339` `countCandidates` | **Runs today.** Returns `len(infos)` and throws the geometry away. |
| `FinderPatternInfo` → 3 × `FinderPattern`, each a `ResultPoint` + `GetEstimatedModuleSize()` | gozxing `qrcode/detector` | Bounding box and module size are one struct read away. |
| Partial centres (1–2 finders, no triple) | `FinderPatternFinder.GetPossibleCenters()`, exported, embedded in `MultiFinderPatternFinder` | Reachable. Needs verifying that `FindMulti` populates centres before returning NotFound. |
| Per-hit geometry, dedup by centroid, reading order | `scanner/hits.go` — `hit{text, points}`, `sameCode`, `sortHits` | **Already a geometry layer**, with a written coordinate contract (`hits.go:16-32`). New strategies must map points back to original-image space. |
| Strategy list (10: channel × binarizer × upscale) | `scanner/scanner.go:296` | Greedy cover, ordered, with measured recall notes. Phase 2 extends this list. |
| Evidence-based early exit | `decodeRaster`, `scanner/scanner.go:390` | Stops when decoded count reaches detected count. Phase 2's ladder plugs in after it, not instead of it. |
| Path-free decode seam for wasm | `ScanDecodedImage`, `scanner/scanner.go:481` | The browser hands over RGBA, Go finds codes. Returns `[]string` — **no geometry crosses to JS today**. |
| Browser worker pool, thumbnails, filters, search, i18n | `web/worker.js`, `web/index.html`, `web/strings.js` | 6 languages, key-parity self-test (`web/i18n.js:63`). Every new string is 6 strings. |
| Bootstrap manifest with `EXPECTED_FAIL` rows for undecoded images | `cmd/corpusgen/main.go`, `scanner/corpus_test.go` | The labelling format Phase 4's contributions would land in already exists. |
| CSP that makes upload structurally impossible | `web/index.html:10-19` | `connect-src 'self' http://127.0.0.1:* http://localhost:*`. See Phase 4 — this is the load-bearing constraint of the whole document. |

There is one more thing worth naming: the codebase already knows this work is
coming. `scanner/scanner.go:582` carries a `TODO(cascade)` saying the
Vision/zbarimg gate is where recall is traded for speed and that finder-pattern
accounting is how to close it provably. `hits.go:28` anticipates "the upcoming
tiling cascade". Phases 1 and 2 are the cash-out of comments already written.

---

# Phase 1 — Failure classification (headless, no UI)

Give every image a reason, not a boolean.

```
DECODED
FINDERS_FOUND_DECODE_FAILED   3 finder patterns in a plausible configuration
PARTIAL_FINDERS               1–2 finders: cropped, or an extreme angle
HIGH_EDGE_DENSITY_REGION      no finders, but a dense-transition region
NO_QR_LIKELY                  clean image, nothing found
```

## Scope

- A classification enum on the per-image result, alongside the existing
  `HasQR`/`Contents`/`Error`. `Error` keeps its current meaning (unreadable
  file) and does **not** become a classification; an unreadable file has no
  classification at all.
- Expose the bounding box whenever finder patterns are located — for
  `FINDERS_FOUND_DECODE_FAILED` from the triple, for `PARTIAL_FINDERS` from
  whatever centres exist. Coordinates in original-image space, per the
  contract in `hits.go:16-32`. This is the field Phase 3 pre-draws.
- Per-image metadata, recorded for every image regardless of outcome:
  dimensions, strategies attempted, Laplacian variance (global blur proxy),
  local edge density, estimated QR version.
- **Metadata is annotation, never a filter.** Nothing is skipped, downranked
  or discarded because a number looks bad. A blurry image still runs every
  strategy. This is a hard rule: the moment a threshold gates work, the
  metadata stops describing the scan and starts causing it, and every recall
  number afterwards measures the threshold instead of the decoder.
- A classification must be derivable *without* re-running detection. The
  finder pass already happens inside `countCandidates`; the change is to
  return a record instead of an `int`.
- Estimated QR version comes free from the triple: modules across ≈
  `distance(topLeft, topRight) / estimatedModuleSize + 7`, version =
  `(modules − 17) / 4`, clamped to 1–40 and reported as an estimate.
- Surfaces: CLI JSON/CSV export, and the Go API. Both are public output
  formats — adding a field is fine, changing `has_qr` is not.

## Non-goals

- No UI. Nothing in `web/` changes in this phase.
- No new decode attempts. Classification observes the existing cascade; it
  does not add a strategy. (That is Phase 2, deliberately separated so the
  first recall change can be measured against a classifier that already
  worked.)
- No confidence scores, no probabilities. Five discrete buckets, each with a
  stated rule. A score invites a threshold, and a threshold is a filter.
- No per-code classification. One image, one classification, plus zero or more
  decoded codes. An image with one decoded code and one undecodable one is a
  real case and is **an open question below**, not a solved one.
- No promise that `NO_QR_LIKELY` means there is no QR. It means nothing was
  detected by anything we ran. The naming (`_LIKELY`) carries that.

## Cheap vs. project

**Cheap** (a day or two):
- `FINDERS_FOUND_DECODE_FAILED`, the bounding box, estimated version. The
  detector already runs; `countCandidates` currently discards
  `[]*FinderPatternInfo` and returns a count. Keep the slice.
- Dimensions, strategies attempted. Both are already in hand at the call site.
- Laplacian variance: a 3×3 convolution over the grayscale projection and a
  variance. ~30 lines, no dependency.

**Cheap but needs a check** (half a day):
- `PARTIAL_FINDERS` via `GetPossibleCenters()` on the embedded finder. The
  method is exported. What must be verified is that `FindMulti` populates the
  centre list before it fails — if it does not, this becomes an own
  implementation of the finder scan, which is not cheap, and the honest
  fallback is to fold 1–2 centres into `NO_QR_LIKELY` and say so.

**A project in itself** (a week, mostly measurement):
- `HIGH_EDGE_DENSITY_REGION`. "A dense-transition region" is not a definition.
  It needs: a window size, a transition-density measure, a threshold, and — the
  expensive part — evidence that the bucket is *useful*, i.e. that images
  landing in it are meaningfully more likely to hold a code than
  `NO_QR_LIKELY` ones. Without that evidence it is a bucket that sends users
  to draw boxes around textured carpet. Measure it on the benchmark's
  `without_qr` subset before shipping the label.

**Estimate:** 3–5 days for everything except edge density; +1 week if edge
density is to be defensible rather than plausible.

## Open questions

1. **Mixed images.** Two codes, one decoded, one not. Is that `DECODED`, or a
   sixth state? The `countCandidates` machinery can already see the gap
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
   cap first and retries at 3000px (`web/worker.js:25-27`). Laplacian variance
   at 1600 and at native resolution are different numbers. Native, or
   as-scanned, or both?
5. **Does classification run in the wasm build?** It has to for Phase 3 to
   exist, which means the seam returning it must be `GOOS=js`-safe and the
   extra work must be cheap enough not to undo the caps in `worker.js`.

---

# Phase 2 — Automatic retry ladder

Exhaust the machine before spending the user's attention.

## Scope

Extend the strategy set with, roughly in the order they are worth trying:

| Rung | Note |
|---|---|
| Otsu binarization | Global, one pass. The existing global-histogram binarizer is close but not the same. |
| Adaptive (Sauvola) | Local threshold. The real win on uneven lighting and glare. |
| Inversion | Light-on-dark codes. Nearly free. |
| 2× / 4× upscale | 2× is already in the list; 4× was measured as not paying (`scanner.go:294`) — it returns here only *after* a first-pass failure, which is a different trade than in the main list. |
| Downscale | For large noisy photos, where sensor noise is the thing destroying module edges. |
| Sharpening | Unsharp mask. |
| Perspective correction | Needs 3 finder patterns → a homography. Phase 1's bounding box is its input. |
| Sliding-window grid scan, with overlap | The "tiling cascade" `hits.go:28` already warns about. |

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
- Allocation failures must cost recall, not the scan. `scanner.go:566` already
  states this as the contract these stages land into.

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
(`scanner.go:180`).

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
before/after on a fixed corpus is a guess. `-tags corpus` plus `corpusgen`
exists; a private photo corpus with ground truth does not, and building one is
a week of dull work that must be budgeted.

**Estimate:** 2 weeks for the cheap and moderate rungs with measurement; +3–4
weeks for perspective correction and tiling.

## Open questions

1. **Where does the ladder run — CLI, wasm, or both?** In the browser it
   competes with the caps in `worker.js` that make the page feel fast. A ladder
   that triples the time on every code-less image is a regression the user
   feels on a folder of holiday photos.
2. **Does the ladder run automatically, or only on the images Phase 1 flagged
   as `FINDERS_FOUND_DECODE_FAILED` / `PARTIAL_FINDERS`?** Gating on
   classification is exactly the "metadata as filter" that Phase 1 forbids —
   but running an expensive ladder on every landscape photo is the cost that
   made the caps necessary. This tension is real and unresolved. One defensible
   line: *classification may order the ladder, never truncate it.*
3. **`ScanFast` vs `ScanExhaustive`.** Is the ladder a third mode, or does it
   extend `ScanExhaustive`? Its docstring already says exhaustive is "for
   building ground truth, where being right matters more than being quick" —
   which is precisely the ladder's job description.
4. **How is the strategy log surfaced?** A field in the JSON export, a separate
   `--stats` output, or a log file? It is diagnostic data, and diagnostic data
   in a user-facing export tends to become an accidental API.
5. **Does a rung that only ever wins in combination get to stay?** The existing
   list is a greedy cover chosen by measured marginal recall. New rungs need
   the same bar, or the list becomes folklore.

---

# Phase 3 — Web UI triage

Let the user resolve what the machine could not, for a batch of ~100 images.

## Scope

- **Batch view**, sized for ~100 images. The list already exists
  (`web/index.html:390`); this is a change to how it is ordered and what each
  row says.
- **Default sort is by classification, most-actionable first — not upload
  order.** Roughly:
  `PARTIAL_FINDERS` → `FINDERS_FOUND_DECODE_FAILED` → `HIGH_EDGE_DENSITY_REGION`
  → `NO_QR_LIKELY` → `DECODED`. The reasoning: a user with 100 images and 20
  minutes should spend them on the images where their attention actually
  converts, and the ones where a box is nearly drawn already convert best. This
  is a behavioural change to the current UI, which appends rows in arrival
  order and only toggles `.hide` (`applyFilters`, `web/index.html:835`) — rows
  are never reordered. Sorting means either reordering DOM nodes or rendering
  from a sorted model.
- **One-line reason per row**, in the user's words, not the enum's: "QR
  detected, couldn't decode" vs "no QR found" vs "part of a QR — it may be cut
  off". Six languages, every key in all of them, enforced by
  `I18N.selfTest()` (`web/i18n.js:63`).
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

## Cheap vs. project

**Cheap** (1–2 days each):
- Sorting by classification. The model is a plain array (`rows`,
  `web/index.html:447`); the render already exists.
- The one-line reason per row. A string per bucket, ×6 languages.

**Moderate** (3–5 days):
- Canvas region selection with a draggable/resizable rectangle. Pointer events,
  handles, touch. Well-trodden, but touch targets and mobile are where it eats
  the time.
- Plumbing geometry from Go to JS. `ScanDecodedImage` returns `[]string`
  (`scanner.go:481`) and `wasm/main.go` marshals exactly that. A structured
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
  canvas, opened on demand, and disciplined `URL.revokeObjectURL`. The current
  code creates thumb URLs and does not obviously revoke them
  (`web/index.html:522`) — worth checking before adding full-size images.
- **The interaction itself, done well** (1–2 weeks). Adjusting a box on a photo
  on a phone, with a keyboard alternative for accessibility, in six languages,
  on a page whose current interaction vocabulary is buttons and filters.

**Estimate:** 2–3 weeks for a competent version; 4+ if mobile and
accessibility are held to the standard of the rest of the page.

## Open questions

1. **Does the pre-drawn box come from Phase 1's detection, or from a fresh
   detection at full resolution when the row is opened?** The first is free and
   possibly wrong by the shrink factor; the second is right and costs a second.
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
(`web/index.html:16`)

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
  origin-and-token guard (`server.go:25-37`) — it is not a service. Phase 4
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

The original argument, unchanged:

1. `countCandidates` (`scanner/scanner.go:339`) returns the
   `[]*FinderPatternInfo` it already computes instead of a count.
2. A classification is derived from what the cascade already did: finders found
   but nothing decoded → `FINDERS_FOUND_DECODE_FAILED`; nothing found →
   `NO_QR_LIKELY`. Two buckets, not five.
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
