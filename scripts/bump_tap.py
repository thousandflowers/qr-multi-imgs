#!/usr/bin/env python3
"""Bump a formula in a Homebrew tap to its source repo's latest release.

Why this exists: goreleaser's `brews:` block needs a long-lived token with push
rights on the tap repo, stored as a secret. That secret has never existed here,
and the failure mode is the worst possible one — the GitHub release publishes,
then the formula push fails, so `brew` silently stays on the old version behind
a release that looks fine. This script does the same job with whatever auth
`gh` already has, so releasing is never blocked on a secret that isn't there.

It knows nothing about any particular project. Everything it needs is read out
of the formula itself:

  * the source repo comes from `homepage`
  * the current version from `version "X.Y.Z"`, or from the tag inside a url
  * every sha256 is recomputed from the file the url immediately above it
    actually points at, so a per-OS formula cannot end up with one arch's
    checksum copied onto another's

Usage:
    scripts/bump_tap.py qr-multi-imgs --dry-run
    scripts/bump_tap.py qr-multi-imgs --tag v1.6.0
    scripts/bump_tap.py --self-test
"""

from __future__ import annotations

import argparse
import base64
import hashlib
import json
import re
import subprocess
import sys
import urllib.request

DEFAULT_TAP = "thousandflowers/homebrew-tap"

VERSION_FIELD = re.compile(r'^\s*version\s+"([^"]+)"', re.M)
HOMEPAGE = re.compile(r'^\s*homepage\s+"https://github\.com/([^/"]+)/([^/"]+)"', re.M)
URL_LINE = re.compile(r'^(\s*)url\s+"([^"]+)"', re.M)
SHA_LINE = re.compile(r'^(\s*)sha256\s+"([0-9a-f]{64})"(.*)$', re.M)
# v1.2.3 as it appears inside a release-download or archive url
TAG_IN_URL = re.compile(r'/(?:download|tags)/v?(\d+\.\d+\.\d+)')


def gh(*args: str) -> str:
    """Run gh and return stdout, failing loudly rather than half-succeeding."""
    out = subprocess.run(["gh", *args], capture_output=True, text=True)
    if out.returncode != 0:
        raise SystemExit(f"gh {' '.join(args)} failed:\n{out.stderr.strip()}")
    return out.stdout


def current_version(text: str) -> str:
    """The version the formula is on, whether or not it has a version field."""
    field = VERSION_FIELD.search(text)
    if field:
        return field.group(1)
    in_url = TAG_IN_URL.search(text)
    if in_url:
        return in_url.group(1)
    raise SystemExit("cannot tell which version this formula is on")


def resolve_urls(text: str, version: str) -> list[str]:
    """Every url in the formula, with Ruby's #{version} filled in."""
    return [u.replace('#{version}', version) for _, u in URL_LINE.findall(text)]


def sha256_of(url: str) -> str:
    h = hashlib.sha256()
    with urllib.request.urlopen(url) as resp:  # noqa: S310 - github.com only
        for chunk in iter(lambda: resp.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def rewrite(text: str, old: str, new: str, shas: list[str]) -> str:
    """Swap the version everywhere, then replace each sha256 in url order.

    The pairing is positional on purpose: shas[i] belongs to the i-th url, so a
    formula with one block per architecture keeps each checksum with its own
    download instead of inheriting the previous one.
    """
    out = text.replace(old, new)
    found = SHA_LINE.findall(out)
    if len(found) != len(shas):
        raise SystemExit(
            f"formula has {len(found)} sha256 lines but {len(shas)} urls; "
            "refusing to guess which goes where")

    it = iter(shas)
    parts: list[str] = []
    pos = 0
    for m in SHA_LINE.finditer(out):
        sha = next(it)
        # keep any trailing comment; some formulae record how the sha was made
        trailing = re.sub(r'\b[0-9a-f]{64}\b', sha, m.group(3)).replace(old, new)
        parts.append(out[pos:m.start()])
        parts.append(f'{m.group(1)}sha256 "{sha}"{trailing}')
        pos = m.end()
    parts.append(out[pos:])
    return "".join(parts)


def bump(formula: str, tap: str, tag: str | None, dry_run: bool) -> int:
    path = f"Formula/{formula}.rb"
    meta = json.loads(gh("api", f"repos/{tap}/contents/{path}"))
    text = base64.b64decode(meta["content"]).decode()

    home = HOMEPAGE.search(text)
    if not home:
        raise SystemExit(f"{path} has no github homepage to take the repo from")
    source = f"{home.group(1)}/{home.group(2)}"

    if tag is None:
        tag = json.loads(gh("api", f"repos/{source}/releases/latest"))["tag_name"]
    new = tag.lstrip("v")
    old = current_version(text)

    print(f"{formula}: tap has {old}, {source} is at {new}")
    if old == new:
        print("  already current, nothing to do")
        return 0

    shas = []
    for u in resolve_urls(text, new):
        print(f"  hashing {u}")
        shas.append(sha256_of(u))

    updated = rewrite(text, old, new, shas)
    if updated == text:
        print("  formula unchanged, nothing to push")
        return 0

    if dry_run:
        print("\n--- would write ---")
        print(updated)
        return 0

    gh("api", "-X", "PUT", f"repos/{tap}/contents/{path}",
       "-f", f"message=chore: {formula} {new}",
       "-f", f"content={base64.b64encode(updated.encode()).decode()}",
       "-f", f"sha={meta['sha']}")
    print(f"  pushed {formula} {new} to {tap}")
    return 0


def self_test() -> int:
    """One runnable check, aimed at what a hand edit actually gets wrong:
    a version left behind somewhere, or a sha landing on the wrong url."""
    formula = '''class Demo < Formula
  homepage "https://github.com/o/demo"
  version "1.0.0"

  on_macos do
    url "https://github.com/o/demo/releases/download/v#{version}/demo_darwin.tar.gz"
    sha256 "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  end
  on_linux do
    url "https://github.com/o/demo/releases/download/v#{version}/demo_linux.tar.gz"
    sha256 "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" # v1.0.0
  end
end
'''
    assert current_version(formula) == "1.0.0"
    assert resolve_urls(formula, "2.0.0") == [
        "https://github.com/o/demo/releases/download/v2.0.0/demo_darwin.tar.gz",
        "https://github.com/o/demo/releases/download/v2.0.0/demo_linux.tar.gz",
    ], "#{version} must be resolved before anything is hashed"

    out = rewrite(formula, "1.0.0", "2.0.0", ["c" * 64, "d" * 64])
    assert 'version "2.0.0"' in out
    assert "1.0.0" not in out, "a stale version was left behind somewhere"
    assert out.index("c" * 64) < out.index("d" * 64), "shas landed on the wrong urls"
    assert "# v2.0.0" in out, "trailing comments must follow the version too"

    # A formula whose sha count does not match its urls must refuse, not guess.
    try:
        rewrite(formula, "1.0.0", "2.0.0", ["c" * 64])
    except SystemExit:
        pass
    else:  # pragma: no cover
        raise AssertionError("a mismatched sha count should have been refused")

    # A source-tarball formula carries no version field; the tag in the url is it.
    tarball = '''class Src < Formula
  homepage "https://github.com/o/src"
  url "https://github.com/o/src/archive/refs/tags/v0.5.0.tar.gz"
  sha256 "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
end
'''
    assert current_version(tarball) == "0.5.0"
    bumped = rewrite(tarball, "0.5.0", "0.6.0", ["f" * 64])
    assert "tags/v0.6.0.tar.gz" in bumped and "f" * 64 in bumped

    print("self-test passed")
    return 0


def main() -> int:
    ap = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("formula", nargs="?", help="formula name, e.g. qr-multi-imgs")
    ap.add_argument("--tap", default=DEFAULT_TAP)
    ap.add_argument("--tag", help="release tag to bump to (default: the latest)")
    ap.add_argument("--dry-run", action="store_true",
                    help="print the formula it would write and push nothing")
    ap.add_argument("--self-test", action="store_true")
    args = ap.parse_args()

    if args.self_test:
        return self_test()
    if not args.formula:
        ap.error("a formula name is required unless --self-test is given")
    return bump(args.formula, args.tap, args.tag, args.dry_run)


if __name__ == "__main__":
    sys.exit(main())
