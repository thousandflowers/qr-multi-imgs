# Discoverability

Settings that live in the GitHub web UI, not in the repo. Nothing here is
applied automatically - copy the values in by hand at
**Settings → General**, or on the repo home page via the ⚙️ next to *About*.

## Name alignment - checked

| Where | Value | Status |
| --- | --- | --- |
| `go.mod` module | `github.com/thousandflowers/qr-multi-imgs` | ✅ |
| README title | `qr-multi-imgs` | ✅ |
| Binary / Homebrew formula | `qr-multi-imgs` | ✅ |
| `.goreleaser.yaml` `project_name` | `qr-multi-imgs` | ✅ |

All install commands in the README resolve against that module path.
Keep the four in sync - a rename means a new module path and a broken
`go install` for everyone.

## About (≤ 350 chars)

Paste into the *About* field. 242 characters:

```
Batch QR code scanner with a terminal UI. Point it at a folder of images: it decodes every QR, then organizes, deletes, exports (JSON/CSV/TXT) or regenerates them as PNG/JPG/SVG. 3332/3332 detection in ~7s. Go, Bubble Tea, nothing to install.
```

Shorter fallback if the field renders cramped (116 characters):

```
Batch QR code scanner with a TUI, scan a folder, decode, organize, export, regenerate. Pure Go, nothing to install.
```

Also set:
- **Website:** leave empty, or point at the latest release.
- ✅ Releases, ✅ Packages - uncheck *Deployments*, *Environments*.

## Topics

GitHub allows 20; these 11 cover the searches that would actually reach
this repo. Paste them one per line:

```
cli
cli-app
cli-tool
qrcode
qrcode-scanner
tui
tui-app
golang
bubbletea
batch-processing
charm
```

Worth adding if you want wider reach (all match real search traffic and
are true of this repo):

```
qr-code
image-processing
terminal
macos
homebrew
```

Avoid topics that are not true of the repo (`qrcode-generator` - it only
regenerates codes it decoded, `zbar` - it works without it). Wrong topics
cost more in bounce than they gain in impressions.

## Other UI-only switches

- **Discussions** - enable it. Q&A that is not a bug does not belong in
  Issues, and Discussions pages are indexed. `.github/ISSUE_TEMPLATE/config.yml`
  links to the Discussions tab, so that link 404s until you turn it on.
- **Social preview image** - Settings → General → Social preview. Without
  one, every link shared on X / LinkedIn / Slack renders as a grey box.
  A single frame of `demo.gif` at 1280×640 is enough.
- **Good first issue labels** - see [GOOD_FIRST_ISSUES.md](GOOD_FIRST_ISSUES.md).
  GitHub surfaces repos in its *good first issue* feed only when issues
  carry that exact label.
- **Repository description on pkg.go.dev** - comes from the README, no
  action needed, but the page only appears after someone fetches the
  module once: https://pkg.go.dev/github.com/thousandflowers/qr-multi-imgs
