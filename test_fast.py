#!/usr/bin/env python3
import os

os.environ["DYLD_LIBRARY_PATH"] = "/opt/homebrew/lib"

import pyzbar.pyzbar as pyzbar
from PIL import Image, ImageEnhance, ImageFilter
from pathlib import Path
import time


def detect_qr_fast(image_path):
    """Fast detection with best methods only."""
    img = Image.open(image_path)

    # Method 1: basic
    decoded = pyzbar.decode(img)
    if decoded:
        img.close()
        return [d.data.decode("utf-8") for d in decoded], []

    # Method 2: grayscale
    gray = img.convert("L")
    decoded = pyzbar.decode(gray)
    if decoded:
        img.close()
        return [d.data.decode("utf-8") for d in decoded], []

    # Method 3: contrast
    enhancer = ImageEnhance.Contrast(gray)
    enhanced = enhancer.enhance(1.5)
    decoded = pyzbar.decode(enhanced)
    if decoded:
        img.close()
        return [d.data.decode("utf-8") for d in decoded], []

    # Method 4: resize 2x
    try:
        scaled = img.resize((img.width * 2, img.height * 2), Image.LANCZOS)
        decoded = pyzbar.decode(scaled)
        scaled.close()
        if decoded:
            img.close()
            return [d.data.decode("utf-8") for d in decoded], []
    except:
        pass

    img.close()
    return [], []


# Get all images
folder = Path("/Users/eugeniozamengopontrelli/Desktop/scontrini")
images = sorted(
    [f for f in folder.iterdir() if f.suffix.lower() in [".jpg", ".jpeg", ".png"]]
)

print(f"Found {len(images)} images")
start = time.time()

found = 0
for i, img_path in enumerate(images):
    contents, _ = detect_qr_fast(img_path)
    if contents:
        print(f"✓ [{i + 1}/{len(images)}] {img_path.name}")
        found += 1
    else:
        print(f"✗ [{i + 1}/{len(images)}] {img_path.name}")

elapsed = time.time() - start
print(f"\\n=== TOTAL: {found}/{len(images)} in {elapsed:.1f}s ===")
