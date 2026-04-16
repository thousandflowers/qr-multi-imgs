#!/usr/bin/env python3
import os

os.environ["DYLD_LIBRARY_PATH"] = "/opt/homebrew/lib"

import sys

sys.path.insert(0, "/Users/eugeniozamengopontrelli/qr-multi-imgs")

from qr_multi_imgs import QRMultiIMGS

scanner = QRMultiIMGS(
    folder_path="/Users/eugeniozamengopontrelli/Desktop/scontrini",
    deep_scan=False,
    verbose=False,
)

images = scanner._get_images()
print(f"Found {len(images)} images")

results = []
found = 0
for i, img in enumerate(images):
    result = scanner.detect_qr(img)
    status = "OK" if result.has_qr else "FAIL"
    print(f"[{i + 1}/24] {img.name} {status}")
    if result.has_qr:
        found += 1
        print(f"    -> {result.qr_contents[0][:50]}")
    results.append(
        f"{img.name},{result.has_qr},{result.qr_contents[0] if result.qr_contents else ''}"
    )

print(f"\\n=== TOTAL: {found}/24 ===")

# Save to file
with open("/Users/eugeniozamengopontrelli/Desktop/scontrini_results.txt", "w") as f:
    f.write("\n".join(results))
print("Saved to scontrini_results.txt")
