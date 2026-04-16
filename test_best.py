#!/usr/bin/env python3
import os

os.environ["DYLD_LIBRARY_PATH"] = "/opt/homebrew/lib"

import pyzbar.pyzbar as pyzbar
from PIL import Image, ImageEnhance, ImageFilter
from pathlib import Path
import time


def detect_qr_best(image_path):
    """Best detection - more methods for higher success."""
    img = Image.open(image_path)

    # Method 1: basic
    decoded = pyzbar.decode(img)
    if decoded:
        img.close()
        return [d.data.decode("utf-8") for d in decoded]

    # Method 2: grayscale
    gray = img.convert("L")
    decoded = pyzbar.decode(gray)
    if decoded:
        img.close()
        return [d.data.decode("utf-8") for d in decoded]

    # Method 3: contrast + grayscale
    enhancer = ImageEnhance.Contrast(gray)
    enhanced = enhancer.enhance(1.5)
    decoded = pyzbar.decode(enhanced)
    if decoded:
        img.close()
        return [d.data.decode("utf-8") for d in decoded]

    # Method 4: unsharp mask
    sharp = enhanced.filter(ImageFilter.UnsharpMask(radius=2, percent=150, threshold=3))
    decoded = pyzbar.decode(sharp)
    if decoded:
        img.close()
        return [d.data.decode("utf-8") for d in decoded]

    # Method 5: resize 2x LANCZOS
    try:
        scaled = img.resize((img.width * 2, img.height * 2), Image.LANCZOS)
        decoded = pyzbar.decode(scaled)
        scaled.close()
        if decoded:
            img.close()
            return [d.data.decode("utf-8") for d in decoded]
    except:
        pass

    # Method 6: resize 2x BICUBIC
    try:
        scaled = img.resize((img.width * 2, img.height * 2), Image.BICUBIC)
        decoded = pyzbar.decode(scaled)
        scaled.close()
        if decoded:
            img.close()
            return [d.data.decode("utf-8") for d in decoded]
    except:
        pass

    # Method 7: resize 3x
    try:
        scaled = img.resize((img.width * 3, img.height * 3), Image.BICUBIC)
        decoded = pyzbar.decode(scaled)
        scaled.close()
        if decoded:
            img.close()
            return [d.data.decode("utf-8") for d in decoded]
    except:
        pass

    # Method 8: grayscale + resize 3x
    try:
        scaled = gray.resize((gray.width * 3, gray.height * 3), Image.LANCZOS)
        decoded = pyzbar.decode(scaled)
        scaled.close()
        if decoded:
            img.close()
            return [d.data.decode("utf-8") for d in decoded]
    except:
        pass

    img.close()
    return []


# Get all images
folder = Path("/Users/eugeniozamengopontrelli/Desktop/scontrini")
images = sorted(
    [f for f in folder.iterdir() if f.suffix.lower() in [".jpg", ".jpeg", ".png"]]
)

print(f"Found {len(images)} images")
start = time.time()

found = 0
for i, img_path in enumerate(images):
    contents = detect_qr_best(img_path)
    if contents:
        print(f"✓ [{i + 1}/{len(images)}] {img_path.name}")
        found += 1
    else:
        print(f"✗ [{i + 1}/{len(images)}] {img_path.name}")

elapsed = time.time() - start
print(f"\\n=== TOTAL: {found}/{len(images)} in {elapsed:.1f}s ===")
