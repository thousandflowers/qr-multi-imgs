# Good first issues

Five tasks that are real, small, and grounded in the current code. Open
them by hand at
[New issue](https://github.com/thousandflowers/qr-multi-imgs/issues/new)
and label each **`good first issue`** + **`help wanted`** - GitHub's
*good first issue* feed keys off those exact labels, which is most of
the point of writing them.

Line numbers are against `main` at the time of writing; check before
pasting if the file has moved on.

---

## 1. Add TIFF to the supported image formats

**Title**

```
Support .tif / .tiff images
```

**Body**

```markdown
Scanned receipts and documents very often come out as TIFF, and right now
qr-multi-imgs skips them silently — the file is not counted, not listed,
not reported as an error.

The format list lives in one place:

- `scanner/scanner.go:48` — `supportedExtensions`
- `scanner/scanner.go:19` — the blank imports that register decoders

TIFF is already reachable: `golang.org/x/image` is an existing direct
dependency and ships `golang.org/x/image/tiff`, so this adds **no new
module** to go.mod.

### What to do

1. Add `_ "golang.org/x/image/tiff"` next to the bmp/webp imports.
2. Add `".tif": true, ".tiff": true` to `supportedExtensions`.
3. Update the format lists that are user-visible:
   - `printHelp()` in `main.go:938` ("Supported formats:")
   - README → **Supported formats**
4. Add a test alongside the existing format tests in
   `scanner/scanner_test.go` that writes a QR as TIFF and asserts it
   decodes.

### Done when

`go test -count=1 ./...` passes and a folder holding a `.tiff` QR is
scanned instead of ignored.

Good first issue — one map entry, one import, one test.
```

---

## 2. `--recursive` flag to scan subfolders

**Title**

```
Add --recursive / -r to scan subfolders
```

**Body**

```markdown
Both scan entry points read a single directory level:

- `scanner/scanner.go:83` in `ScanFolder`
- `scanner/scanner.go:207` in `ScanFolderStream`

Anyone whose photos are filed per-day or per-order (`orders/2026-08/…`)
has to run the tool once per folder. A `-r` / `--recursive` flag should
walk the tree instead.

### What to do

1. Extend `parseArgs` in `main.go:926` — it already handles `-h/--help`
   and `-v/--version`, so this is the same shape.
2. Thread the flag down to the scanner. The lazy version: keep
   `os.ReadDir` for the default path and switch to `filepath.WalkDir`
   when recursive, so the non-recursive behaviour cannot regress.
3. Skip hidden directories (`.git`, `.cache`) while walking.
4. Document it in `printHelp()` and the README.
5. Add a test with a nested temp tree: one QR at the top level, one a
   directory down; assert 1 result without the flag and 2 with it.

### Careful about

- The organize (`o`) and delete (`d`) actions write next to the source
  images. Decide and state what recursive means for those — the safe
  answer is to keep writing relative to the root folder.
- Symlinked directories can loop. `filepath.WalkDir` does not follow
  them by default; keep it that way.

### Done when

`qr-multi-imgs -r ./nested` finds QRs at every depth, and the default
run still finds only top-level ones.
```

---

## 3. Honour `NO_COLOR` and adapt to light terminals

**Title**

```
Honour NO_COLOR and use adaptive colors for light terminals
```

**Body**

~~~markdown
Every style in `main.go:58-67` is a hardcoded hex value:

```go
titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#52525B"))
```

Two consequences:

1. On a light terminal, `helpStyle` (#52525B) and `mutedStyle` (#A1A1AA)
   are close to unreadable.
2. `NO_COLOR=1`, which [no-color.org](https://no-color.org) has made the
   convention for CLIs, is ignored. That also affects anyone piping the
   TUI output somewhere it will be read as plain text.

### What to do

1. Swap `lipgloss.Color` for `lipgloss.AdaptiveColor{Light: …, Dark: …}`
   on the styles that carry meaning — muted, help, title, cursor. Keep
   the current hexes as the `Dark` values so the default look is unchanged.
2. Respect `NO_COLOR`: lipgloss already exposes a renderer whose
   color profile can be forced to `termenv.Ascii`. One check at startup
   is enough — do not scatter `if noColor` through the views.
3. Add a test that the styles resolve without panicking under both
   profiles.

### Done when

The TUI is readable on a light background, and `NO_COLOR=1
qr-multi-imgs ./folder` renders without escape sequences.

Good first issue — contained to the style block, no scanner logic touched.
~~~

---

## 4. Make the zbarimg test hermetic

**Title**

```
Make TestDecodeWithZbarimg independent of the host's zbarimg
```

**Body**

~~~markdown
`TestDecodeWithZbarimg` in `scanner/scanner_test.go` passes, skips, or
fails depending on which zbarimg happens to be installed on the machine
running it. On macOS/arm64, zbar 0.23.93 segfaults on a perfectly valid
PNG, so the test currently skips with "zbarimg present but unusable on
this host". That skip is a patch for a broken host, not a fix — on most
machines the test now asserts nothing at all.

The fix is to stop calling the real binary.

### What to do

1. Write a fake zbarimg into `t.TempDir()`: a tiny shell script that
   prints a known payload and exits 0. Remember to `chmod +x` it.
2. Prepend that directory to `PATH` with `t.Setenv("PATH", ...)`.
3. Assert `decodeWithZbarimg`'s own behaviour against it — argument
   construction, trailing-newline trimming, and the empty-output path —
   with no dependency on a real zbar install.

### Careful about

`zbarimgPath` is resolved once in `init()` (`scanner/scanner.go`) via
`exec.LookPath`, so a `PATH` override set inside a test will never be
seen. Making that lookup re-resolvable — or injectable — is the substance
of this task, not an aside. Keep it small and keep production behaviour
identical: zbarimg is an optional last-resort fallback and must stay one.

### Done when

`go test -count=1 ./scanner -run TestDecodeWithZbarimg` asserts real
behaviour on a machine with no zbar installed at all, and still passes on
one with a working zbar.

Good first issue — one fake binary, one seam, no new dependency.
~~~

---

## 5. Strip newlines before truncating a payload in the TUI list

**Title**

```
Multi-line payloads (vCard, vEvent) break the results list layout
```

**Body**

```markdown
The results list gives each image one row, but a payload containing
newlines is written into that row verbatim, so one image takes three
lines and the alignment of everything under it goes with it.

A vCard is the common case, and it is not exotic — every "add me to your
contacts" QR code is one:

    [QR] IMG_0412.png  BEGIN:VCARD
    FN:R. Bianchi
    END:VCARD
    [QR] IMG_0418.png  https://example.com/menu

`truncate` in `main.go` shortens by length only; it has no opinion about
control characters. The fix is to collapse whitespace — newlines, tabs,
carriage returns — into single spaces before the length cut, so the row
shows `BEGIN:VCARD FN:R. Bianchi END:VCARD` and stays one row.

Do it in `truncate` rather than at the call sites: every caller wants a
single line, and one of them already forgot.

### Done when

A test builds a `ScanResult` whose content contains `\n` and asserts
`viewList()` emits one line per image. The detail view, which has room
for the whole payload, must keep its line breaks.
```

Good first issue — one function, one test, no new dependency.
