# awesome-go PR

Repo: <https://github.com/avelino/awesome-go>
Rules: <https://github.com/avelino/awesome-go/blob/main/CONTRIBUTING.md>

## ⚠️ Do not submit yet — one rule is not met

awesome-go requires **at least 5 months of history since the first
commit**. First commit here is **2026-06-09**, so the repo becomes
eligible around **2026-11-09**. Submitting early gets the PR closed and
burns the first impression with the maintainers.

Second item to fix before submitting: coverage. They ask for ≥80% per
non-data package. Measured on `main`:

| Package | Coverage |
| --- | --- |
| `.` (TUI) | 94.4% |
| `exporter` | 100.0% |
| `scanner` | **77.1%** ← under the bar |
| total | 89.2% |

Everything else passes: MIT license, `go.mod` at the root, SemVer release
(`v1.4.1`), English README, pkg.go.dev reachable, active commits.

## Section

**Software Packages → Other Software**, not *Images*. The *Images*
section is described as _"Libraries for manipulating images"_ — this is a
standalone application, not a library, so it belongs with the other
end-user tools.

Alphabetical slot: after `portal`, before `restic`.

## The exact line to add

```md
- [qr-multi-imgs](https://github.com/thousandflowers/qr-multi-imgs) - Terminal UI to scan a folder of images for QR codes, then organize, export, or regenerate them.
```

Placed like this:

```md
- [portal](https://github.com/SpatiumPortae/portal) - Portal is a quick and easy command-line file transfer utility from any computer to another.
- [qr-multi-imgs](https://github.com/thousandflowers/qr-multi-imgs) - Terminal UI to scan a folder of images for QR codes, then organize, export, or regenerate them.
- [restic](https://github.com/restic/restic) - De-duplicating backup program.
```

Checked against their formatting rules: link text is the exact project
name, description is one line, non-promotional, ends with a period, and
the entry adds one item only.

## PR title

```
Add qr-multi-imgs
```

## PR body

Their template asks for these four links; fill the rest of the checklist
in the template as it appears.

```md
Forge link: https://github.com/thousandflowers/qr-multi-imgs
pkg.go.dev: https://pkg.go.dev/github.com/thousandflowers/qr-multi-imgs
goreportcard.com: https://goreportcard.com/report/github.com/thousandflowers/qr-multi-imgs
Coverage: https://app.codecov.io/gh/thousandflowers/qr-multi-imgs

qr-multi-imgs is a terminal UI for batch QR code work. It scans a folder of
images, decodes every QR it finds, and then acts on the results: split the
folder into with_qr/ and without_qr/, delete the images with no code, export
JSON/CSV/TXT, or regenerate the decoded codes as PNG/JPG/SVG.

Decoding is gozxing (pure Go); on macOS it additionally uses Apple's Vision
framework for photographed codes. There is no system dependency to install
on any platform.

MIT licensed, Go 1.26, released as v1.4.1 with prebuilt binaries for macOS,
Linux and Windows.
```

## Before opening it

- [ ] Repo is ≥5 months old (2026-11-09).
- [ ] `scanner` package coverage ≥80%.
- [ ] Go Report Card grade is A- or better — check the live page, the
      badge in the README links straight to it.
- [ ] Rebase on their `main` right before opening; that file changes
      hourly and the insertion point moves.
- [ ] One item only in the diff.

---

# Appendix — awesome-tui

Kept from the earlier draft. Different list, different rules; check
`CONTRIBUTING.md` in <https://github.com/rothgar/awesome-tui> for the
current section names before using this.

**Entry:**

```md
- [qr-multi-imgs](https://github.com/thousandflowers/qr-multi-imgs) - Batch QR code scanner: scan a folder, decode QR codes, organize files, export results, all through a Bubble Tea TUI with drag & drop.
```

**PR body:**

```md
Add qr-multi-imgs to the list.

qr-multi-imgs is a terminal UI built with Bubble Tea for batch QR code
scanning. Drag a folder onto the terminal (or type a path), and the TUI
shows scan results, then exports them, deletes images without a QR,
organizes files into subfolders, or regenerates QR images as PNG/JPG/SVG.
Five input methods, a parallel scan engine, and no system dependencies.
```
