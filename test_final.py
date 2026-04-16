#!/usr/bin/env python3
import os

os.environ["DYLD_LIBRARY_PATH"] = "/opt/homebrew/lib"

import sys

sys.path.insert(0, "/Users/eugeniozamengopontrelli/qr-multi-imgs")

from qr_multi_imgs import QRMultiIMGS
import time

scanner = QRMultiIMGS(
    folder_path="/Users/eugeniozamengopontrelli/Desktop/scontrini",
    deep_scan=True,
    verbose=False,
)

images = scanner._get_images()
print(f"Found {len(images)} images")
start = time.time()

found = 0
for i, img in enumerate(images):
    result = scanner.detect_qr(img)
    if result.has_qr:
        print(f"✓ [{i + 1}/{len(images)}] {img.name}")
        found += 1
    else:
        print(f"✗ [{i + 1}/{len(images)}] {img.name}")

elapsed = time.time() - start
print(f"\\n=== TOTAL: {found}/{len(images)} in {elapsed:.1f}s ===")
