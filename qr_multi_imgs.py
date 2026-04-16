#!/usr/bin/env python3
# =============================================================================
# QR MULTI IMGS - QR Code Scanner for Images
# IMPROVED VERSION - Enhanced detection for blurry, miscut, difficult QR codes
# =============================================================================
"""
QR Multi IMGS - QR Code Scanner for Images
Version: v0.6.0-Enhanced
Author: QR Multi IMGS Team
License: MIT

Enhanced Detection:
- Extended preprocessing (10+ methods)
- Sharpening for blurry QR codes
- Deblur for very blurry QR codes
- Rotation detection for mis-rotated QR codes
- Multi-scale detection for different sizes
- Auto-escalation for failed images
- Verbose error reporting
"""

import os
import sys
import json
import csv
import logging
import argparse
import signal
import platform
import re
from pathlib import Path
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor, as_completed

if platform.system() == "Darwin":
    import ctypes.util

    possible_paths = [
        "/opt/homebrew/lib",
        "/usr/local/lib",
        "/usr/lib",
    ]
    for lib_path in possible_paths:
        if os.path.exists(lib_path):
            if hasattr(os, "add_dll_directory"):
                os.add_dll_directory(lib_path)
            else:
                os.environ.setdefault("DYLD_LIBRARY_PATH", lib_path)

try:
    from textual.app import App, ComposeResult
    from textual.widgets import Header, Footer, Static, Button, Input
    from textual.containers import Container
    from textual.screen import Screen
    from textual.binding import Binding

    TEXTUAL_AVAILABLE = True
except ImportError:
    TEXTUAL_AVAILABLE = False

import qrcode
from PIL import Image, ImageEnhance, ImageFilter
import pyzbar.pyzbar as pyzbar

SUPPORTED_FORMATS = {".jpg", ".jpeg", ".png", ".bmp", ".gif", ".webp", ".tiff", ".tif"}
DEFAULT_QR_FORMAT = "png"
DEFAULT_PADDING = 20

DEFAULT_TIMEOUT = 60
DEFAULT_DEEP_TIMEOUT = 120

CONTRAST_FACTOR = 1.5
SHARPNESS_FACTOR = 1.5
VERSION = "v0.6.0-Enhanced"


class QRCodeResult:
    """
    Result of QR code detection on a single image.

    Attributes:
        file_path: Path to the image file
        has_qr: True if at least one QR code was found
        qr_contents: List of decoded QR code strings
        qr_bboxes: List of bounding boxes (x, y, width, height) for each QR
        error: Error message if processing failed
        file_size: Size in bytes
        timestamp: ISO format timestamp
        attempts_made: List of detection methods tried
        methods_failed: List of methods that failed
        detection_method: The method that successfully detected the QR
    """

    def __init__(
        self,
        file_path: str,
        has_qr: bool,
        qr_contents: list = None,
        qr_bboxes: list = None,
        error: str = None,
    ):
        self.file_path = file_path
        self.has_qr = has_qr
        self.qr_contents = qr_contents or []
        self.qr_bboxes = qr_bboxes or []
        self.error = error
        self.file_size = 0
        self.timestamp = datetime.now().isoformat()
        self.attempts_made = []
        self.methods_failed = []
        self.detection_method = None

        if os.path.exists(file_path):
            self.file_size = os.path.getsize(file_path)

    def to_dict(self):
        return {
            "file_path": self.file_path,
            "has_qr": self.has_qr,
            "qr_contents": self.qr_contents,
            "qr_bboxes": self.qr_bboxes,
            "file_size": self.file_size,
            "timestamp": self.timestamp,
            "error": self.error,
            "attempts_made": self.attempts_made,
            "detection_method": self.detection_method,
        }


class QRMultiIMGS:
    """
    QR Code Scanner for Images - Enhanced Version.

    Scans a folder of images and detects QR codes using pyzbar.
    Supports multiple detection methods for difficult/blurry/miscut QR codes.

    Enhanced Detection Phases:
    - Phase 1: Standard methods (1-3)
    - Phase 2: Sharpening + Deblur (4-5)
    - Phase 3: Rotation + Multi-scale (6-7)
    - Phase 4: Full aggressive (all methods combined)

    Attributes:
        folder_path: Path to folder to scan
        recursive: Whether to scan subfolders
        formats: Set of allowed image extensions
        parallel: Use thread pool for faster scanning
        log_file: Enable logging to file
        qr_format: Output format for recreated QR codes
        timeout: Seconds per image before timeout (0=disabled)
        deep_scan: Enable enhanced detection
        verbose: Show detailed progress and errors
        force_deep: Use maximum detection methods
        results: List of QRCodeResult objects
    """

    def __init__(
        self,
        folder_path: str,
        recursive: bool = False,
        formats: list = None,
        parallel: bool = False,
        log_file: bool = False,
        qr_format: str = "png",
        timeout: int = DEFAULT_TIMEOUT,
        deep_scan: bool = True,
        deep_timeout: int = DEFAULT_DEEP_TIMEOUT,
        verbose: bool = False,
        force_deep: bool = False,
    ):
        self.folder_path = Path(folder_path)
        self.recursive = recursive
        self.formats = formats or SUPPORTED_FORMATS
        self.parallel = parallel
        self.log_file = log_file
        self.qr_format = qr_format.lower()
        self.timeout = timeout
        self.deep_scan = deep_scan
        self.deep_timeout = deep_timeout
        self.verbose = verbose
        self.force_deep = force_deep
        self.results: list[QRCodeResult] = []
        self.logger = None
        self._scan_count = 0
        self._total_count = 0
        self._failed_images = []

        if log_file:
            self._setup_logger()

    def _setup_logger(self):
        logging.basicConfig(
            filename="qr_scanner.log",
            level=logging.INFO,
            format="%(asctime)s - %(levelname)s - %(message)s",
            filemode="a",
        )
        self.logger = logging.getLogger(__name__)

    def _log(self, message: str):
        if self.logger:
            self.logger.info(message)
        if self.verbose:
            print(f"[LOG] {message}")
        else:
            print(message)

    def _get_with_qr(self) -> list:
        return [r for r in self.results if r.has_qr]

    def _get_without_qr(self) -> list:
        return [r for r in self.results if not r.has_qr and not r.error]

    def _get_failed(self) -> list:
        return [r for r in self.results if r.error is not None]

    def _get_total_qr_count(self, with_qr_list: list = None) -> int:
        if with_qr_list is None:
            with_qr_list = self._get_with_qr()
        return sum(len(r.qr_contents) for r in with_qr_list)

    def _normalize_format(self, ext: str) -> str:
        ext = ext.lower()
        if not ext.startswith("."):
            ext = "." + ext
        return ext

    def _get_images(self) -> list[Path]:
        images = []
        if self.recursive:
            for ext in self.formats:
                images.extend(self.folder_path.rglob(f"*{ext}"))
        else:
            for ext in self.formats:
                images.extend(self.folder_path.glob(f"*{ext}"))
        return [f for f in images if f.is_file()]

    def _preprocess_image(self, image: Image.Image) -> Image.Image:
        enhancer = ImageEnhance.Contrast(image)
        image = enhancer.enhance(CONTRAST_FACTOR)
        enhancer = ImageEnhance.Sharpness(image)
        image = enhancer.enhance(SHARPNESS_FACTOR)
        if image.mode not in ("RGB", "L"):
            image = image.convert("RGB")
        return image

    def _extract_qr_data(self, decoded: list) -> tuple[list, list]:
        contents = [d.data.decode("utf-8", errors="ignore") for d in decoded]
        bboxes = [
            (d.rect.left, d.rect.top, d.rect.width, d.rect.height) for d in decoded
        ]
        return contents, bboxes

    # =========================================================================
    # DETECTION METHODS
    # =========================================================================

    def _detect_qr_method1(self, image: Image.Image) -> tuple[list, list]:
        """Method 1: Standard direct decode."""
        decoded = pyzbar.decode(image)
        return self._extract_qr_data(decoded)

    def _detect_qr_method2(self, image: Image.Image) -> tuple[list, list]:
        """Method 2: Grayscale conversion."""
        gray = image.convert("L")
        decoded = pyzbar.decode(gray)
        return self._extract_qr_data(decoded)

    def _detect_qr_method3_extended(
        self, img: Image.Image, attempt: int = 0
    ) -> tuple[list, list, str]:
        """Method 3: Extended preprocessing - 10+ variations for general difficult QR."""
        methods = []

        methods.append(lambda i: i)
        methods.append(lambda i: i.convert("RGB"))
        methods.append(lambda i: self._preprocess_image(i))

        try:
            methods.append(
                lambda i: i.resize((i.width * 2, i.height * 2), Image.LANCZOS)
            )
        except Exception:
            pass

        try:
            methods.append(
                lambda i: i.resize((i.width * 2, i.height * 2), Image.BICUBIC)
            )
        except Exception:
            pass

        try:
            methods.append(
                lambda i: i.resize((i.width * 3, i.height * 3), Image.BICUBIC)
            )
        except Exception:
            pass

        methods.append(lambda i: i.convert("L"))
        methods.append(lambda i: self._preprocess_image(i.convert("L")))

        methods.append(
            lambda i: i.filter(
                ImageFilter.UnsharpMask(radius=2, percent=150, threshold=3)
            )
        )
        methods.append(lambda i: i.filter(ImageFilter.MedianFilter(size=3)))

        try:
            methods.append(lambda i: i.filter(ImageFilter.MinFilter(3)))
            methods.append(lambda i: i.filter(ImageFilter.MaxFilter(3)))
        except Exception:
            pass

        if attempt >= len(methods):
            return [], [], "all_methods_exhausted"

        try:
            processed_img = methods[attempt](img)
            decoded = pyzbar.decode(processed_img)
            contents, bboxes = self._extract_qr_data(decoded)
            return contents, bboxes, None
        except Exception as e:
            return [], [], str(e)

    def _detect_qr_method4_sharpen(self, image: Image.Image) -> tuple[list, list]:
        """Method 4: Sharpening for blurry QR codes using OpenCV."""
        try:
            import cv2
            import numpy as np

            img_array = np.array(image.convert("RGB"))
            results = []

            # Sharpening kernels
            kernels = [
                np.array([[-1, -1, -1], [-1, 9, -1], [-1, -1, -1]]),
                np.array([[0, -1, 0], [-1, 5, -1], [0, -1, 0]]),
                np.array([[-1, -1, -1], [-1, 8, -1], [-1, -1, -1]]) / -1,
            ]

            for kernel in kernels:
                try:
                    blurred = cv2.filter2D(img_array, -1, kernel)
                    result_img = Image.fromarray(blurred)
                    try:
                        decoded = pyzbar.decode(result_img)
                        if decoded:
                            contents, bboxes = self._extract_qr_data(decoded)
                            if contents:
                                return contents, bboxes
                    finally:
                        result_img.close()
                except Exception:
                    continue

            for percent in [200, 250, 300]:
                try:
                    blurred = cv2.GaussianBlur(img_array, (0, 0), percent / 100)
                    sharpened = cv2.addWeighted(img_array, 1.5, blurred, -0.5, 0)
                    result_img = Image.fromarray(sharpened)
                    try:
                        decoded = pyzbar.decode(result_img)
                        if decoded:
                            contents, bboxes = self._extract_qr_data(decoded)
                            if contents:
                                return contents, bboxes
                    finally:
                        result_img.close()
                except Exception:
                    continue

            return [], []
        except ImportError:
            return [], []
        except Exception:
            return [], []
        return [], []

    def _detect_qr_method5_deblur(self, image: Image.Image) -> tuple[list, list]:
        """Method 5: Deblur for very blurry QR codes using OpenCV."""
        try:
            import cv2
            import numpy as np

            img_array = np.array(image.convert("RGB"))
            gray = cv2.cvtColor(img_array, cv2.COLOR_RGB2GRAY)

            for kernel_size in [3, 5, 7]:
                for sigma in [1, 2, 3]:
                    try:
                        blurred = cv2.GaussianBlur(
                            gray, (kernel_size, kernel_size), sigma
                        )
                        sharpened = cv2.addWeighted(gray, 1.5, blurred, -0.5, 0)
                        result_img = Image.fromarray(sharpened)
                        try:
                            decoded = pyzbar.decode(result_img)
                            if decoded:
                                contents, bboxes = self._extract_qr_data(decoded)
                                if contents:
                                    return contents, bboxes
                        finally:
                            result_img.close()
                    except Exception:
                        continue

            try:
                deblurred = cv2.equalizeHist(gray)
                result_img = Image.fromarray(deblurred)
                try:
                    decoded = pyzbar.decode(result_img)
                    if decoded:
                        contents, bboxes = self._extract_qr_data(decoded)
                        if contents:
                            return contents, bboxes
                finally:
                    result_img.close()
            except Exception:
                pass

            try:
                bilateral = cv2.bilateralFilter(gray, 9, 75, 75)
                result_img = Image.fromarray(bilateral)
                enhancer = ImageEnhance.Sharpness(result_img)
                try:
                    enhanced = enhancer.enhance(2.0)
                    decoded = pyzbar.decode(enhanced)
                    if decoded:
                        contents, bboxes = self._extract_qr_data(decoded)
                        if contents:
                            return contents, bboxes
                finally:
                    result_img.close()
                    enhanced.close()
            except Exception:
                pass

            return [], []
        except ImportError:
            return [], []
        except Exception:
            return [], []

    def _detect_qr_method6_rotation(self, image: Image.Image) -> tuple[list, list]:
        """Method 6: Rotation attempts for mis-rotated QR codes."""
        methods = []

        for angle in [90, 180, 270]:
            try:
                methods.append(lambda i, a=angle: i.rotate(a, expand=True))
            except Exception:
                pass

        methods.append(lambda i: i.transpose(Image.FLIP_LEFT_RIGHT))
        methods.append(lambda i: i.transpose(Image.FLIP_TOP_BOTTOM))

        try:
            methods.append(lambda i: i.transpose(Image.TRANSPOSE))
            methods.append(lambda i: i.transpose(Image.TRANSVERSE))
        except Exception:
            pass

        for processed in methods:
            try:
                img = processed(image)
                try:
                    decoded = pyzbar.decode(img)
                    if decoded:
                        contents, bboxes = self._extract_qr_data(decoded)
                        if contents:
                            return contents, bboxes
                finally:
                    img.close()
            except Exception:
                continue

        return [], []

    def _detect_qr_method7_multiscale(self, image: Image.Image) -> tuple[list, list]:
        """Method 7: Multi-scale detection for different sizes."""
        scales = [0.5, 0.75, 1.5, 2.0, 2.5, 3.0]

        for scale in scales:
            try:
                new_width = int(image.width * scale)
                new_height = int(image.height * scale)
                scaled = image.resize((new_width, new_height), Image.LANCZOS)
                try:
                    decoded = pyzbar.decode(scaled)
                    if decoded:
                        contents, bboxes = self._extract_qr_data(decoded)
                        if contents:
                            return [c for c in contents], [
                                (
                                    b[0] // scale,
                                    b[1] // scale,
                                    b[2] // scale,
                                    b[3] // scale,
                                )
                                for b in bboxes
                            ]
                finally:
                    scaled.close()
            except Exception:
                continue

        return [], []

    def _detect_qr_method8_qreader(self, image: Image.Image) -> tuple[list, list]:
        """Method 8: Use QReader for difficult QR codes (requires qreader package)."""
        try:
            import numpy as np
            import cv2
            from qreader import QReader

            qreader = QReader()
            img_array = np.array(image)
            if len(img_array.shape) == 2:
                img_array = cv2.cvtColor(img_array, cv2.COLOR_GRAY2RGB)
            elif img_array.shape[2] == 4:
                img_array = cv2.cvtColor(img_array, cv2.COLOR_RGBA2RGB)

            results = qreader.detect_and_decode(image=img_array)

            contents = [r for r in results if r is not None]
            return contents, []
        except ImportError:
            return [], []
        except Exception:
            return [], []

    # =========================================================================
    # MAIN DETECTION WITH AUTO-ESCALATION
    # =========================================================================

    def _detect_phase1(self, img: Image.Image) -> tuple[list, list, str]:
        """Phase 1: Standard detection methods (1-3)."""
        contents, bboxes = self._detect_qr_method1(img)
        if contents:
            return contents, bboxes, "method1"

        contents, bboxes = self._detect_qr_method2(img)
        if contents:
            return contents, bboxes, "method2"

        for attempt in range(10):
            contents, bboxes, error = self._detect_qr_method3_extended(img, attempt)
            if contents:
                return contents, bboxes, f"method3_attempt{attempt}"
            if error == "all_methods_exhausted":
                break

        return [], [], "phase1_exhausted"

    def _detect_phase2(self, img: Image.Image) -> tuple[list, list, str]:
        """Phase 2: Sharpening and deblur (methods 4-5)."""
        contents, bboxes = self._detect_qr_method4_sharpen(img)
        if contents:
            return contents, bboxes, "method4_sharpen"

        contents, bboxes = self._detect_qr_method5_deblur(img)
        if contents:
            return contents, bboxes, "method5_deblur"

        return [], [], "phase2_exhausted"

    def _detect_phase3(self, img: Image.Image) -> tuple[list, list, str]:
        """Phase 3: Rotation and multi-scale (methods 6-7)."""
        contents, bboxes = self._detect_qr_method6_rotation(img)
        if contents:
            return contents, bboxes, "method6_rotation"

        contents, bboxes = self._detect_qr_method7_multiscale(img)
        if contents:
            return contents, bboxes, "method7_multiscale"

        return [], [], "phase3_exhausted"

    def _detect_full(self, img: Image.Image) -> tuple[list, list, str]:
        """Phase 4: Full aggressive detection (all methods)."""
        contents, bboxes = self._detect_qr_method8_qreader(img)
        if contents:
            return contents, bboxes, "method8_qreader"

        combined_methods = [
            lambda i: self._preprocess_image(i).filter(
                ImageFilter.UnsharpMask(radius=2, percent=200, threshold=3)
            ),
            lambda i: self._preprocess_image(i.convert("L")),
            lambda i: i.filter(ImageFilter.MedianFilter(size=5)),
        ]

        for processed in combined_methods:
            try:
                img_processed = processed(img)
                decoded = pyzbar.decode(img_processed)
                if decoded:
                    contents, bboxes = self._extract_qr_data(decoded)
                    if contents:
                        return contents, bboxes, "method9_combined"
            except Exception:
                continue

        return [], [], "all_methods_exhausted"

    def detect_qr(self, image_path: Path) -> QRCodeResult:
        """Main detection with automatic escalation for failed images."""
        contents = []
        bboxes = []
        detection_method = None

        effective_timeout = (
            self.deep_timeout if (self.deep_scan or self.force_deep) else self.timeout
        )
        use_signal = effective_timeout > 0 and platform.system() != "Windows"

        def timeout_handler(signum, frame):
            raise TimeoutError(f"Timeout processing {image_path}")

        try:
            if use_signal:
                signal.signal(signal.SIGALRM, timeout_handler)
                signal.alarm(effective_timeout)

            img = Image.open(image_path)

            if self.verbose:
                print(f"    Processing: {image_path.name}")

            contents, bboxes, method = self._detect_phase1(img)
            if contents:
                detection_method = method
                if self.verbose:
                    print(f"    ✓ Detected in Phase 1: {method}")

            if not contents and (self.deep_scan or self.force_deep):
                contents, bboxes, method = self._detect_phase2(img)
                if contents:
                    detection_method = method
                    if self.verbose:
                        print(f"    ✓ Detected in Phase 2: {method}")

            if not contents and self.force_deep:
                contents, bboxes, method = self._detect_phase3(img)
                if contents:
                    detection_method = method
                    if self.verbose:
                        print(f"    ✓ Detected in Phase 3: {method}")

            if not contents:
                contents, bboxes, method = self._detect_full(img)
                if contents:
                    detection_method = method
                    if self.verbose:
                        print(f"    ✓ Detected in Full Phase: {method}")

            if use_signal:
                signal.alarm(0)

            img.close()

            result = QRCodeResult(
                str(image_path),
                has_qr=len(contents) > 0,
                qr_contents=contents,
                qr_bboxes=bboxes,
            )
            result.detection_method = detection_method
            result.attempts_made = (
                ["phase1", "phase2", "phase3", "full"]
                if self.force_deep
                else ["phase1"]
            )

            return result

        except TimeoutError as e:
            if self.log_file:
                self._log(
                    f"Timeout processing {image_path}: {effective_timeout}s exceeded"
                )
            return QRCodeResult(str(image_path), has_qr=False, error="Timeout error")
        except Exception as e:
            if self.log_file:
                self._log(f"Error processing {image_path}: {e}")
            return QRCodeResult(str(image_path), has_qr=False, error=str(e))

    def _retry_failed_images(self):
        """Auto-retry failed images with enhanced detection."""
        if not self._failed_images:
            return

        if self.verbose:
            print(f"\n=== Auto-retrying {len(self._failed_images)} failed images ===")

        original_deep_scan = self.deep_scan
        original_force_deep = self.force_deep

        self.deep_scan = True
        self.force_deep = True

        for result in self._failed_images:
            if self.verbose:
                print(f"  Retrying: {result.file_path}")

            image_path = Path(result.file_path)
            if image_path.exists():
                retry_result = self.detect_qr(image_path)
                for i, r in enumerate(self.results):
                    if r.file_path == result.file_path:
                        self.results[i] = retry_result
                        break

        self.deep_scan = original_deep_scan
        self.force_deep = original_force_deep

    def scan(self, progress: bool = True) -> list[QRCodeResult]:
        images = self._get_images()
        self._total_count = len(images)

        if not images:
            return []

        if self.parallel:
            with ThreadPoolExecutor() as executor:
                futures = {executor.submit(self.detect_qr, img): img for img in images}
                completed_count = 0
                for future in as_completed(futures):
                    completed_count += 1
                    self._scan_count = completed_count
                    img_path = futures[future]
                    result = future.result()
                    self.results.append(result)

                    if progress:
                        status = "✓" if result.has_qr else "✗"
                        print(
                            f"[{completed_count}/{len(images)}] {img_path.name} {status}"
                        )

                    if not result.has_qr and not result.error:
                        self._failed_images.append(result)
        else:
            for i, img in enumerate(images):
                self._scan_count = i + 1
                result = self.detect_qr(img)
                self.results.append(result)

                if progress:
                    status = "✓" if result.has_qr else "✗"
                    print(f"[{self._scan_count}/{len(images)}] {img.name} {status}")

                if not result.has_qr and not result.error:
                    self._failed_images.append(result)

        if self.force_deep and self._failed_images:
            self._retry_failed_images()

        return self.results

    # =========================================================================
    # ACTION METHODS (Preserved from original)
    # =========================================================================

    def export_list(self, format: str = "txt", output_path: str = None) -> str:
        if not output_path:
            output_path = f"qr_results.{format}"

        with_qr = self._get_with_qr()
        without_qr = self._get_without_qr()

        os.makedirs(
            os.path.dirname(output_path) if os.path.dirname(output_path) else ".",
            exist_ok=True,
        )

        if format == "json":
            with open(output_path, "w") as f:
                json.dump(
                    {
                        "with_qr": [r.to_dict() for r in with_qr],
                        "without_qr": [r.to_dict() for r in without_qr],
                        "total": len(self.results),
                        "detected": len(with_qr),
                        "failed": len(self._failed_images),
                    },
                    f,
                    indent=2,
                )

        elif format == "csv":
            with open(output_path, "w", newline="") as f:
                writer = csv.writer(f)
                writer.writerow(
                    [
                        "file_path",
                        "has_qr",
                        "qr_contents",
                        "file_size",
                        "timestamp",
                        "detection_method",
                    ]
                )
                for r in self.results:
                    writer.writerow(
                        [
                            r.file_path,
                            r.has_qr,
                            "|".join(r.qr_contents),
                            r.file_size,
                            r.timestamp,
                            r.detection_method or "",
                        ]
                    )

        else:
            with open(output_path, "w") as f:
                f.write(f"QR Multi IMGS Enhanced Scan Results\n")
                f.write(f"{'=' * 60}\n\n")
                f.write(f"Total images scanned: {len(self.results)}\n")
                f.write(f"With QR codes: {len(with_qr)}\n")
                f.write(f"Without QR codes: {len(without_qr)}\n")
                f.write(
                    f"Detection rate: {len(with_qr) / len(self.results) * 100:.1f}%\n\n"
                )

                if with_qr:
                    f.write(f"{'-' * 60}\n")
                    f.write(f"IMAGES WITH QR CODES:\n")
                    f.write(f"{'-' * 60}\n")
                    for r in with_qr:
                        f.write(f"\nFile: {r.file_path}\n")
                        f.write(f"Size: {r.file_size} bytes\n")
                        f.write(f"Method: {r.detection_method}\n")
                        f.write(f"QR Contents: {', '.join(r.qr_contents)}\n")

                if without_qr:
                    f.write(f"\n{'-' * 60}\n")
                    f.write(f"IMAGES WITHOUT QR CODES:\n")
                    f.write(f"{'-' * 60}\n")
                    for r in without_qr:
                        f.write(f"File: {r.file_path}\n")

        return output_path

    def action_delete(self, output_folder: str = None, confirm: bool = False) -> int:
        if output_folder:
            is_valid, error = _validate_path(output_folder)
            if not is_valid:
                raise ValueError(f"Invalid output path: {error}")

        without_qr = self._get_without_qr()

        if not confirm:
            print(f"\nFound {len(without_qr)} images without QR codes.")
            response = input("Delete these images? (yes/no): ").strip().lower()
            if response not in ("yes", "y"):
                print("Cancelled.")
                return 0

        deleted = 0
        for r in without_qr:
            try:
                os.remove(r.file_path)
                deleted += 1
            except Exception as e:
                if self.log_file:
                    self._log(f"Error deleting {r.file_path}: {e}")

        print(f"Deleted {deleted} images.")
        return deleted

    def action_organize(
        self, output_folder: str = None, move: bool = False, confirm: bool = False
    ) -> dict:
        if output_folder:
            is_valid, error = _validate_path(output_folder)
            if not is_valid:
                raise ValueError(f"Invalid output path: {error}")

        base_path = Path(output_folder) if output_folder else self.folder_path

        with_qr_folder = base_path / "with_qr"
        without_qr_folder = base_path / "without_qr"

        with_qr_folder.mkdir(parents=True, exist_ok=True)
        without_qr_folder.mkdir(parents=True, exist_ok=True)

        with_qr = self._get_with_qr()
        without_qr = self._get_without_qr()

        if not confirm:
            print(f"\nWith QR: {len(with_qr)}, Without QR: {len(without_qr)}")
            response = input("Organize these images? (yes/no): ").strip().lower()
            if response not in ("yes", "y"):
                print("Cancelled.")
                return {"with_qr": 0, "without_qr": 0}

        action = os.replace if move else os.copy2

        count = {"with_qr": 0, "without_qr": 0}

        for r in with_qr:
            try:
                src = Path(r.file_path)
                dst = with_qr_folder / src.name
                action(str(src), str(dst))
                count["with_qr"] += 1
            except Exception as e:
                if self.log_file:
                    self._log(f"Error organizing {r.file_path}: {e}")

        for r in without_qr:
            try:
                src = Path(r.file_path)
                dst = without_qr_folder / src.name
                action(str(src), str(dst))
                count["without_qr"] += 1
            except Exception as e:
                if self.log_file:
                    self._log(f"Error organizing {r.file_path}: {e}")

        print(
            f"Organized: {count['with_qr']} with QR, {count['without_qr']} without QR."
        )
        return count

    def action_recreate(
        self, output_folder: str = None, naming: str = "original"
    ) -> int:
        output_path = (
            Path(output_folder) if output_folder else self.folder_path / "recreated_qr"
        )

        if output_folder:
            is_valid, error = _validate_path(output_folder)
            if not is_valid:
                raise ValueError(f"Invalid output path: {error}")

        output_path.mkdir(parents=True, exist_ok=True)

        with_qr = self._get_with_qr()

        if not with_qr:
            print("No QR codes found to recreate.")
            return 0

        total_qr = self._get_total_qr_count(with_qr)
        print(f"Recreating {total_qr} QR code images...")

        qr_count = 0

        for r in with_qr:
            src_path = Path(r.file_path)
            base_name = src_path.stem

            for i, content in enumerate(r.qr_contents):
                qr = qrcode.QRCode(
                    version=1, error_correction=qrcode.constants.ERROR_CORRECT_H
                )
                qr.add_data(content)
                qr.make(fit=True)

                img = qr.make_image(fill_color="black", back_color="white")
                filename = self._get_output_filename(
                    base_name,
                    f".{self.qr_format}",
                    naming,
                    content,
                    i,
                    len(r.qr_contents),
                    qr_count,
                )

                img.save(str(output_path / filename))
                qr_count += 1

        print(f"Created {qr_count} QR code images in {output_path}")
        return qr_count

    def _get_output_filename(
        self,
        base_name: str,
        extension: str,
        naming: str,
        content: str,
        index: int,
        total: int,
        qr_index: int,
    ) -> str:
        if naming == "sequential":
            return f"qr_{qr_index:04d}{extension}"
        elif naming == "content":
            safe_content = (
                content[:50].replace("/", "_").replace(":", "_")
                if content
                else "empty_qr"
            )
            return f"{safe_content}{extension}"
        else:
            suffix = f"_qr{index + 1}{extension}" if total > 1 else f"_qr{extension}"
            return f"{base_name}{suffix}"

    def action_extract(
        self,
        output_folder: str = None,
        naming: str = "original",
        padding: int = DEFAULT_PADDING,
    ) -> int:
        output_path = (
            Path(output_folder) if output_folder else self.folder_path / "extracted_qr"
        )

        if output_folder:
            is_valid, error = _validate_path(output_folder)
            if not is_valid:
                raise ValueError(f"Invalid output path: {error}")

        output_path.mkdir(parents=True, exist_ok=True)

        with_qr = self._get_with_qr()

        if not with_qr:
            print("No QR codes found to extract.")
            return 0

        total_qr = self._get_total_qr_count(with_qr)
        print(f"Extracting {total_qr} QR code regions...")

        qr_count = 0

        for r in with_qr:
            src_path = Path(r.file_path)
            base_name = src_path.stem
            src_ext = src_path.suffix

            try:
                src_img = Image.open(r.file_path)
                img_width, img_height = src_img.size
            except Exception as e:
                if self.log_file:
                    self._log(f"Error opening {r.file_path}: {e}")
                continue

            for i, (content, bbox) in enumerate(zip(r.qr_contents, r.qr_bboxes)):
                x, y, w, h = bbox

                x1 = max(0, x - padding)
                y1 = max(0, y - padding)
                x2 = min(img_width, x + w + padding)
                y2 = min(img_height, y + h + padding)

                cropped = src_img.crop((x1, y1, x2, y2))

                filename = self._get_output_filename(
                    base_name, src_ext, naming, content, i, len(r.qr_contents), qr_count
                )

                cropped.save(str(output_path / filename))
                qr_count += 1

            src_img.close()

        print(f"Extracted {qr_count} QR code regions in {output_path}")
        return qr_count

    def action_list(self) -> None:
        with_qr = self._get_with_qr()
        without_qr = self._get_without_qr()
        failed = self._get_failed()

        detection_rate = (len(with_qr) / len(self.results) * 100) if self.results else 0

        print(f"\n{'=' * 60}")
        print(f"QR Multi IMGS - Enhanced Scan Results")
        print(f"{'=' * 60}")
        print(f"Total images scanned: {len(self.results)}")
        print(f"With QR codes: {len(with_qr)}")
        print(f"Without QR codes: {len(without_qr)}")
        print(f"Failed/Error: {len(failed)}")
        print(f"Detection rate: {detection_rate:.1f}%")

        if with_qr:
            print(f"\n--- Images WITH QR codes ---")
            for r in with_qr:
                qr_preview = (
                    r.qr_contents[0][:50] + "..."
                    if len(r.qr_contents[0]) > 50
                    else r.qr_contents[0]
                )
                method_info = (
                    f" ({r.detection_method})"
                    if r.detection_method and self.verbose
                    else ""
                )
                print(f"  ✓ {r.file_path}{method_info}")
                print(f"    QR: {qr_preview}")
                if len(r.qr_contents) > 1:
                    print(f"    +{len(r.qr_contents) - 1} more QR codes")

        if without_qr:
            print(f"\n--- Images WITHOUT QR codes ---")
            for r in without_qr:
                print(f"  ✗ {r.file_path}")

        if failed and self.verbose:
            print(f"\n--- Failed images (errors) ---")
            for r in failed:
                print(f"  ! {r.file_path}: {r.error}")

    def action_decode(self, output_format: str = "text") -> list:
        with_qr = self._get_with_qr()

        if not with_qr:
            print("No QR codes found to decode.")
            return []

        if output_format == "json":
            results = []
            for r in with_qr:
                results.append(
                    {
                        "file": r.file_path,
                        "qr_codes": r.qr_contents,
                        "count": len(r.qr_contents),
                        "method": r.detection_method,
                    }
                )
            print(json.dumps(results, indent=2))
        else:
            for r in with_qr:
                for i, content in enumerate(r.qr_contents):
                    method_info = (
                        f" [{r.detection_method}]" if r.detection_method else ""
                    )
                    if len(r.qr_contents) > 1:
                        print(f"{r.file_path} [{i + 1}]{method_info}: {content}")
                    else:
                        print(f"{r.file_path}{method_info}: {content}")

        print(
            f"\nTotal: {len(with_qr)} images with {sum(len(r.qr_contents) for r in with_qr)} QR codes"
        )
        return with_qr

    def action_filter(
        self, pattern: str, case_sensitive: bool = False, exclude: bool = False
    ) -> list:
        with_qr = self._get_with_qr()

        if case_sensitive:
            regex = re.compile(pattern)
        else:
            regex = re.compile(pattern, re.IGNORECASE)

        matching = []
        non_matching = []

        for r in with_qr:
            matched = any(regex.search(content) for content in r.qr_contents)
            if matched:
                matching.append(r)
            else:
                non_matching.append(r)

        if exclude:
            results = non_matching
            print(f"\n--- Images NOT matching '{pattern}' ---")
        else:
            results = matching
            print(f"\n--- Images matching '{pattern}' ---")

        if results:
            for r in results:
                print(f"  {r.file_path}")
                for content in r.qr_contents:
                    match = regex.search(content)
                    if match:
                        highlight = f"  → {match.group()[:80]}"
                        print(highlight)
        else:
            print("  No images match the filter.")

        print(f"\nTotal: {len(results)} images")

        return results

    def action_batch_rename(
        self, prefix: str = "", suffix: str = "", dry_run: bool = True
    ) -> dict:
        with_qr = self._get_with_qr()

        if not with_qr:
            print("No QR codes found for renaming.")
            return {"renamed": 0, "errors": 0, "changes": []}

        safe_pattern = re.compile(r"[^\w\-]")

        changes = []
        errors = 0

        print(f"\n--- Batch Rename {'(DRY RUN)' if dry_run else ''} ---")

        for r in with_qr:
            src = Path(r.file_path)
            content = r.qr_contents[0] if r.qr_contents else "unknown"

            safe_name = safe_pattern.sub("_", content[:50])

            if prefix:
                safe_name = prefix + safe_name
            if suffix:
                safe_name = safe_name + suffix

            new_name = f"{safe_name}{src.suffix}"
            dst = src.parent / new_name

            if os.path.exists(src):
                if dry_run:
                    print(f"  {src.name} → {new_name}")
                else:
                    try:
                        src.rename(dst)
                        print(f"  Renamed: {src.name} → {new_name}")
                    except Exception as e:
                        print(f"  Error renaming {src.name}: {e}")
                        errors += 1
                        continue

                changes.append({"from": str(src), "to": str(dst)})
            else:
                print(f"  File not found: {src}")
                errors += 1

        print(
            f"\nTotal: {len(changes)} files would be renamed{'' if dry_run else ' (applied)'}"
        )

        if dry_run:
            print("\nNote: This was a dry run. Use --confirm to apply changes.")

        return {"renamed": len(changes), "errors": errors, "changes": changes}

    def action_verify(
        self, originals_folder: str = None, recreated_folder: str = None
    ) -> dict:
        if not originals_folder:
            originals_folder = str(self.folder_path)

        if not recreated_folder:
            print("Error: --output folder is required for verify action")
            return {"matched": 0, "mismatched": 0, "errors": 0}

        originals_path = Path(originals_folder)
        recreated_path = Path(recreated_folder)

        if not recreated_path.exists():
            print(f"Error: Recreated folder not found: {recreated_folder}")
            return {"matched": 0, "mismatched": 0, "errors": 0}

        matched = 0
        mismatched = 0
        errors = 0

        print(f"\n--- Verify QR Codes ---")
        print(f"Originals: {originals_folder}")
        print(f"Recreated: {recreated_folder}")
        print()

        original_qr_contents = {}
        for original_img in originals_path.glob("*"):
            if original_img.suffix.lower() not in SUPPORTED_FORMATS:
                continue
            try:
                orig_decoded = pyzbar.decode(Image.open(original_img))
                if orig_decoded:
                    content = orig_decoded[0].data.decode("utf-8")
                    original_qr_contents[content] = str(original_img)
            except Exception:
                continue

        print(f"Pre-scanned {len(original_qr_contents)} original images")

        recreated_files = list(recreated_path.glob("*"))

        for recreated_file in recreated_files:
            if recreated_file.suffix.lower() not in SUPPORTED_FORMATS:
                continue

            try:
                decoded = pyzbar.decode(Image.open(recreated_file))
                if not decoded:
                    print(f"  ❌ {recreated_file.name}: No QR code found")
                    errors += 1
                    continue

                recreated_content = decoded[0].data.decode("utf-8")

                if recreated_content in original_qr_contents:
                    print(f"  ✅ {recreated_file.name}: MATCH")
                    matched += 1
                else:
                    print(f"  ❌ {recreated_file.name}: MISMATCH")
                    print(f"     Content: {recreated_content[:50]}...")
                    mismatched += 1

            except Exception as e:
                print(f"  ❌ {recreated_file.name}: Error - {e}")
                errors += 1

        print(f"\n--- Results ---")
        print(f"  Matched: {matched}")
        print(f"  Mismatched: {mismatched}")
        print(f"  Errors: {errors}")

        if mismatched == 0 and errors == 0:
            print(f"\n  ✅ All QR codes verified successfully!")
        else:
            print(f"\n  ⚠️ Verification issues found!")

        return {"matched": matched, "mismatched": mismatched, "errors": errors}


def _validate_path(path: str, base_dir: str = None) -> tuple[bool, str]:
    try:
        input_path = Path(path)
        resolved = input_path.resolve()

        if not resolved.exists():
            return False, f"Path does not exist: {path}"

        if not resolved.is_dir():
            return False, f"Path is not a directory: {path}"

        if base_dir:
            base_resolved = Path(base_dir).resolve()
            try:
                resolved.relative_to(base_resolved)
            except ValueError:
                return False, f"Path is outside allowed directory tree: {path}"

        return True, ""

    except Exception as e:
        return False, f"Invalid path: {path} - {str(e)}"


def run_cli(args):
    is_valid, error = _validate_path(args.path)
    if not is_valid:
        print(f"Error: {error}")
        sys.exit(1)

    formats = None
    if args.formats:
        formats = {f".{f.strip('.')}" for f in args.formats.split(",")}

    verbose = getattr(args, "verbose", False)
    force_deep = getattr(args, "force_deep", False)

    scanner = QRMultiIMGS(
        folder_path=args.path,
        recursive=args.recursive,
        formats=formats,
        parallel=args.parallel,
        log_file=args.log,
        qr_format=args.qr_format or "png",
        timeout=args.timeout or DEFAULT_TIMEOUT,
        deep_scan=args.deep_scan or True,
        deep_timeout=args.deep_timeout or DEFAULT_DEEP_TIMEOUT,
        verbose=verbose,
        force_deep=force_deep,
    )

    print(f"Scanning folder: {args.path}")
    if verbose:
        print(
            f"Detection mode: {'Full (force-deep)' if force_deep else 'Enhanced (deep-scan)'}"
        )
    print("-" * 50)

    results = scanner.scan(progress=args.progress)

    if args.action == "list":
        scanner.action_list()
    elif args.action == "export":
        scanner.export_list(format=args.export_format, output_path=args.output)
    elif args.action == "delete":
        scanner.action_delete(output_folder=args.output, confirm=args.confirm)
    elif args.action == "organize":
        scanner.action_organize(
            output_folder=args.output, move=args.move, confirm=args.confirm
        )
    elif args.action == "recreate":
        scanner.action_recreate(output_folder=args.output, naming=args.naming)
    elif args.action == "extract":
        scanner.action_extract(
            output_folder=args.output, naming=args.naming, padding=args.padding
        )
    elif args.action == "decode":
        scanner.action_decode(output_format=args.export_format)
    elif args.action == "filter":
        if not args.filter_pattern:
            print("Error: --filter-pattern is required for filter action")
            sys.exit(1)
        scanner.action_filter(
            pattern=args.filter_pattern,
            case_sensitive=args.filter_case_sensitive,
            exclude=args.filter_exclude,
        )
    elif args.action == "batch-rename":
        result = scanner.action_batch_rename(
            prefix=args.rename_prefix or "",
            suffix=args.rename_suffix or "",
            dry_run=not args.confirm,
        )
        if not args.confirm:
            print("\nNo files were renamed (dry run). Use --confirm to apply changes.")
    elif args.action == "verify":
        scanner.action_verify(originals_folder=args.path, recreated_folder=args.output)


QRMultiIMG = QRMultiIMGS


try:
    from tui_screens import (
        FolderScreen,
        SubfolderScreen,
        ActionScreen,
        OutputScreen,
        RunScreen,
    )
except ImportError:
    FolderScreen = None
    SubfolderScreen = None
    ActionScreen = None
    OutputScreen = None
    RunScreen = None


def _run_interactive_menu(args, parser):
    print("\n" + "=" * 50)
    print("  QR Multi IMGS - Enhanced Interactive Menu")
    print("=" * 50)
    print()

    folder = input("Enter folder path to scan: ").strip()
    if not folder:
        print("Error: No folder path provided")
        return

    if not os.path.isdir(folder):
        print(f"Error: Folder not found: {folder}")
        return

    print("\nScan subfolders too? (Y/n): ", end="")
    recursive_input = input().strip().lower()
    recursive = recursive_input in ("", "y", "yes")

    print("\nSelect action:")
    print("  1. Show   - Display which images have QR codes")
    print("  2. Save   - Export results to a file")
    print("  3. Delete - Remove images without QR codes")
    print("  4. Sort   - Organize into folders")
    print("  5. Create - Generate new QR code images")
    print("  6. Crop   - Extract QR code regions")
    print("  7. Read   - Just decode, don't save")
    print("  8. Filter - Find images by QR content")
    print("  9. Rename - Batch rename files by QR")
    print(" 10. Check  - Verify recreated QR codes")
    print("\nEnter number (1-10): ", end="")

    action_map = {
        "1": "list",
        "2": "export",
        "3": "delete",
        "4": "organize",
        "5": "recreate",
        "6": "extract",
        "7": "decode",
        "8": "filter",
        "9": "batch-rename",
        "10": "verify",
    }

    action_input = input().strip()
    action = action_map.get(action_input, "list")

    new_args = argparse.Namespace(
        path=folder,
        action=action,
        recursive=recursive,
        formats=None,
        output=None,
        export_format="txt",
        qr_format="png",
        move=False,
        confirm=False,
        parallel=False,
        progress=True,
        log=False,
        naming="original",
        timeout=DEFAULT_TIMEOUT,
        padding=DEFAULT_PADDING,
        deep_scan=True,
        deep_timeout=DEFAULT_DEEP_TIMEOUT,
        verbose=False,
        force_deep=False,
        filter_pattern=None,
        filter_case_sensitive=False,
        filter_exclude=False,
        rename_prefix=None,
        rename_suffix=None,
        nomenu=True,
    )

    print(f"\nRunning: {action} on {folder} (recursive={recursive})")
    print("-" * 50)

    try:
        run_cli(new_args)
    except Exception as e:
        print(f"Error: {e}")


def main():
    parser = argparse.ArgumentParser(
        description="QR Multi IMGS - Enhanced QR Code Scanner for Images"
    )
    parser.add_argument("--path", "-p", help="Folder path to scan")
    parser.add_argument(
        "--action",
        "-a",
        choices=[
            "list",
            "export",
            "delete",
            "organize",
            "recreate",
            "extract",
            "decode",
            "filter",
            "batch-rename",
            "verify",
        ],
        default="list",
        help="Action to perform",
    )
    parser.add_argument(
        "--recursive", "-r", action="store_true", help="Scan subfolders recursively"
    )
    parser.add_argument("--formats", "-f", help="Image formats (comma-separated)")
    parser.add_argument("--output", "-o", help="Output folder path")
    parser.add_argument("--rename-prefix", help="Prefix for batch-rename action")
    parser.add_argument("--rename-suffix", help="Suffix for batch-rename action")
    parser.add_argument(
        "--filter-pattern", help="Pattern to filter QR codes (for filter action)"
    )
    parser.add_argument(
        "--filter-case-sensitive", action="store_true", help="Case sensitive filter"
    )
    parser.add_argument(
        "--filter-exclude", action="store_true", help="Exclude matching"
    )
    parser.add_argument(
        "--export-format",
        choices=["txt", "json", "csv"],
        default="txt",
        help="Export format",
    )
    parser.add_argument(
        "--qr-format",
        choices=["png", "svg", "pdf"],
        default="png",
        help="QR image format",
    )
    parser.add_argument(
        "--move", action="store_true", help="Move files instead of copy"
    )
    parser.add_argument(
        "--confirm", action="store_true", help="Skip confirmation prompt"
    )
    parser.add_argument(
        "--parallel", action="store_true", help="Process images in parallel"
    )
    parser.add_argument(
        "--progress", action="store_true", help="Show progress during scan"
    )
    parser.add_argument("--log", action="store_true", help="Save log to file")
    parser.add_argument("--nomenu", action="store_true", help="Skip interactive menu")
    parser.add_argument(
        "--naming", choices=["original", "content", "sequential"], default="original"
    )
    parser.add_argument(
        "--timeout", "-t", type=int, default=DEFAULT_TIMEOUT, help="Timeout per image"
    )
    parser.add_argument(
        "--padding", type=int, default=20, help="Padding for extract action"
    )
    parser.add_argument(
        "--deep-scan", action="store_true", help="Enable enhanced detection"
    )
    parser.add_argument(
        "--deep-timeout",
        type=int,
        default=DEFAULT_DEEP_TIMEOUT,
        help="Deep scan timeout",
    )
    parser.add_argument(
        "--verbose", "-v", action="store_true", help="Show detailed progress and errors"
    )
    parser.add_argument(
        "--force-deep", action="store_true", help="Use maximum detection methods"
    )

    args = parser.parse_args()

    if args.path:
        run_cli(args)
        return

    _run_interactive_menu(args, parser)


if __name__ == "__main__":
    main()
