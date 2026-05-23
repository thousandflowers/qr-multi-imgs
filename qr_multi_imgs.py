#!/usr/bin/env python3
# =============================================================================
# QR MULTI IMGS - QR Code Scanner for Images
# IMPROVED VERSION - Enhanced detection for blurry, miscut, difficult QR codes
# =============================================================================
"""
QR Multi IMGS - QR Code Scanner for Images
Version: v0.7.0
Author: QR Multi IMGS Team
License: MIT

Detection pipeline:
- Phase 1: Basic decode, grayscale, contrast, resize (always)
- Phase 2: Sharpen, deblur, resize 3x (deep_scan)
- Phase 3: Rotation, multiscale, QReader, adaptive, morphology (force_deep)
- Full: Extreme scale 4x-8x (fallback)
"""

import os
import sys
import json
import csv
import logging
import argparse
import shutil
import platform
import re
import time
from pathlib import Path
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor, as_completed
from threading import Lock

# ── ANSI color helpers (stdlib, zero dependencies) ─────────────
_G = "\033[32m"
_R = "\033[31m"
_Y = "\033[33m"
_C = "\033[36m"
_B = "\033[1m"
_D = "\033[2m"
_E = "\033[0m"

# Color support: respects NO_COLOR env var (https://no-color.org/) and --no-color flag
_COLOR_ENABLED = None  # None = uninitialized, set on first call


def _color_enabled() -> bool:
    """Return True if ANSI color output is supported."""
    global _COLOR_ENABLED
    if _COLOR_ENABLED is None:
        _COLOR_ENABLED = (
            "NO_COLOR" not in os.environ
            and sys.stdout.isatty()
        )
    return _COLOR_ENABLED


def _set_color(enabled: bool) -> None:
    """Override color detection (used by --no-color flag)."""
    global _COLOR_ENABLED
    _COLOR_ENABLED = enabled


def _clr(text: str, *codes: str) -> str:
    """Wrap *text* in ANSI SGR *codes*; respects NO_COLOR / --no-color / piped output."""
    if not _color_enabled():
        return text
    code_map = {"green": _G, "red": _R, "yellow": _Y, "cyan": _C, "bold": _B, "dim": _D}
    return "".join(code_map[c] for c in codes) + text + _E


# Braille spinner frames — CLI analogue of a loading animation
_SPINNER = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"


def _spinner_text(current: int, completed: int, total: int) -> str:
    """Render a braille spinner + progress counter."""
    idx = completed % len(_SPINNER)
    return f"\r  {_SPINNER[idx]} {current}/{total} "


# ── Box-drawing helpers for bold CLI output ───────────────────
_MIN_BOX_W = 40
_MAX_BOX_W = 120


def _get_box_width() -> int:
    """Detect terminal width, clamped to [_MIN_BOX_W, _MAX_BOX_W]."""
    try:
        cols = shutil.get_terminal_size().columns
        return max(_MIN_BOX_W, min(cols - 2, _MAX_BOX_W))
    except Exception:
        return _MIN_BOX_W


_BOX_W = _get_box_width()


def _bar(value: int, max_val: int, width: int = 10) -> str:
    """Return a 10-segment filled/empty bar string."""
    filled = round(value / max_val * width) if max_val else 0
    return "█" * filled + "░" * (width - filled)


def _box_top() -> str:
    return f"  ╭{'─' * (_BOX_W - 2)}╮"


def _box_mid() -> str:
    return f"  ├{'─' * (_BOX_W - 2)}┤"


def _box_bot() -> str:
    return f"  ╰{'─' * (_BOX_W - 2)}╯"


def _panel(text: str) -> str:
    """Wrap *text* inside a │ │ panel line."""
    # Strip ANSI codes for visual-width calculation
    clean = re.sub(r"\033\[[0-9;]*m", "", text)
    pad = _BOX_W - 4 - len(clean)
    return f"  │ {text}{' ' * pad} │"


def _section(text: str, color: str = "dim") -> str:
    """Full-width rule with label: ── label ──────────────────"""
    label = f"─ {text} ─"
    filler = "─" * max(0, _BOX_W - 4 - len(label))
    return _clr(f"  {label}{filler}", color, "bold")


if platform.system() == "Darwin":
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

    # Ensure zbar library path is set
    zbar_path = "/opt/homebrew/lib"
    if os.path.exists(zbar_path) and "DYLD_LIBRARY_PATH" in os.environ:
        if zbar_path not in os.environ["DYLD_LIBRARY_PATH"]:
            os.environ["DYLD_LIBRARY_PATH"] = (
                zbar_path + ":" + os.environ.get("DYLD_LIBRARY_PATH", "")
            )
    elif os.path.exists(zbar_path):
        os.environ["DYLD_LIBRARY_PATH"] = zbar_path

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

# ── Exit codes ────────────────────────────────────────────────
EC_OK = 0       # QR/barcode found
EC_NO_QR = 1    # No QR/barcode detected
EC_ERROR = 2    # Processing error

# ── Supported symbologies ─────────────────────────────────────
ALL_SYMBOLS = {
    "QRCODE", "EAN13", "EAN8", "CODE128", "CODE39", "CODE93",
    "I25", "DATABAR", "DATABAR_EXP", "PDF417", "AZTEC", "MAXICODE",
}
DEFAULT_SYMBOLS = {"QRCODE"}

SUPPORTED_FORMATS = {".jpg", ".jpeg", ".png", ".bmp", ".gif", ".webp", ".tiff", ".tif"}
DEFAULT_QR_FORMAT = "png"
DEFAULT_PADDING = 20

DEFAULT_TIMEOUT = 15
DEFAULT_DEEP_TIMEOUT = 30
SHORT_CIRCUIT_ATTEMPTS = 10  # skip expensive Phase 3/Full after this many failed decode attempts

CONTRAST_FACTOR = 1.5
SHARPNESS_FACTOR = 1.5
VERSION = "v0.8.0"


class QRCodeResult:
    """Result of barcode/QR detection on a single image."""

    def __init__(
        self,
        file_path: str,
        has_qr: bool,
        qr_contents: list = None,
        qr_bboxes: list = None,
        error: str = None,
        symbologies: list = None,
    ):
        self.file_path = file_path
        self.has_qr = has_qr
        self.qr_contents = qr_contents or []
        self.qr_bboxes = qr_bboxes or []
        self.symbologies = symbologies or []
        self.structured = []  # list of dicts from parse_structured_content
        self.scannability = []  # list of score dicts
        self.error = error
        self.file_size = 0
        self.timestamp = datetime.now().isoformat()
        self.attempts_made = []
        self.methods_failed = []
        self.detection_method = None

        if os.path.exists(file_path):
            self.file_size = os.path.getsize(file_path)

    def parse_structured(self):
        """Populate self.structured from raw qr_contents."""
        self.structured = [QRMultiIMGS.parse_structured_content(c) for c in self.qr_contents]

    def to_dict(self, structured: bool = False):
        d = {
            "file_path": self.file_path,
            "has_qr": self.has_qr,
            "qr_contents": self.qr_contents,
            "qr_bboxes": self.qr_bboxes,
            "symbologies": self.symbologies,
            "file_size": self.file_size,
            "timestamp": self.timestamp,
            "error": self.error,
            "attempts_made": self.attempts_made,
            "detection_method": self.detection_method,
        }
        if structured:
            d["structured"] = [s.get("formatted", s["raw"]) for s in self.structured]
        if self.scannability:
            d["scannability"] = self.scannability
        return d


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
        self._results_lock = Lock()
        self._symbols = None
        self._compute_score = False

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

    def _try_decode(
        self, img: Image.Image, close_img: bool = False, bbox_scale: float = 1.0
    ) -> tuple[list, list]:
        """Decode *img* with pyzbar, return (contents, bboxes).

        Single try/except wrapper around decode + extract + check.
        If *close_img*, closes *img* in a finally block.
        If *bbox_scale* != 1.0, divides bbox coords by scale (for rescaled images).
        Returns ([], []) on failure.
        """
        self._decode_attempts = getattr(self, "_decode_attempts", 0) + 1
        try:
            decoded = pyzbar.decode(img)
            if decoded:
                contents, bboxes = self._extract_qr_data(decoded)
                if contents:
                    if bbox_scale != 1.0:
                        bboxes = [
                            (
                                int(b[0] / bbox_scale),
                                int(b[1] / bbox_scale),
                                int(b[2] / bbox_scale),
                                int(b[3] / bbox_scale),
                            )
                            for b in bboxes
                        ]
                    return contents, bboxes
        except Exception:
            pass
        finally:
            if close_img:
                try:
                    img.close()
                except Exception:
                    pass
        return [], []

    def _extract_all_symbols(self, decoded: list) -> tuple[list, list, list]:
        """Extract data, bboxes, and symbology types from pyzbar results."""
        contents = []
        bboxes = []
        symbologies = []
        for d in decoded:
            contents.append(d.data.decode("utf-8", errors="ignore"))
            bboxes.append((d.rect.left, d.rect.top, d.rect.width, d.rect.height))
            symbologies.append(d.type)
        return contents, bboxes, symbologies

    # ── Structured QR/barcode content parser ────────────────────
    @staticmethod
    def parse_structured_content(content: str) -> dict:
        """Parse QR/barcode content into structured type and formatted display.

        Recognizes: WiFi, vCard, email, SMS, geo, tel, URL, calendar, bitcoin.
        Returns dict with keys: type, raw, formatted, fields.
        """
        result = {"type": "unknown", "raw": content, "formatted": content, "fields": {}}

        # WiFi: WIFI:S:ssid;T:WPA;P:password;;
        wifi_match = re.match(r"^WIFI:S:(.+?);T:(.+?);P:(.+?);+$", content, re.I)
        if wifi_match:
            result["type"] = "wifi"
            result["fields"] = {"ssid": wifi_match.group(1), "encryption": wifi_match.group(2), "password": wifi_match.group(3)}
            pw = result["fields"]["password"]
            masked = pw[:1] + "••••" + pw[-1:] if len(pw) > 4 else "••••"
            result["formatted"] = f"WiFi: {result['fields']['ssid']} ({result['fields']['encryption']}) — pw: {masked}"
            return result

        # vCard: BEGIN:VCARD...VERSION:...FN:...TEL:...END:VCARD
        vcard_match = re.match(r"^BEGIN:VCARD", content, re.I)
        if vcard_match:
            result["type"] = "vcard"
            fn = re.search(r"\nFN[:;](.+)", content, re.I)
            tel = re.search(r"\nTEL[:;](.+)", content, re.I)
            email = re.search(r"\nEMAIL[:;](.+)", content, re.I)
            org = re.search(r"\nORG[:;](.+)", content, re.I)
            fields = {}
            if fn: fields["name"] = fn.group(1).strip()
            if tel: fields["tel"] = tel.group(1).strip()
            if email: fields["email"] = email.group(1).strip()
            if org: fields["org"] = org.group(1).strip()
            result["fields"] = fields
            name = fields.get("name", "?")
            details = ", ".join(v for k, v in fields.items() if k != "name")
            result["formatted"] = f"vCard: {name}" + (f" ({details})" if details else "")
            return result

        # Calendar: BEGIN:VEVENT...DTSTART:...SUMMARY:...END:VEVENT
        vevent_match = re.match(r"^BEGIN:VEVENT", content, re.I)
        if vevent_match:
            result["type"] = "calendar"
            summary = re.search(r"\nSUMMARY[:;](.+)", content, re.I)
            dtstart = re.search(r"\nDTSTART[:;](.+)", content, re.I)
            dtend = re.search(r"\nDTEND[:;](.+)", content, re.I)
            loc = re.search(r"\nLOCATION[:;](.+)", content, re.I)
            fields = {}
            if summary: fields["summary"] = summary.group(1).strip()
            if dtstart: fields["start"] = dtstart.group(1).strip()
            if dtend: fields["end"] = dtend.group(1).strip()
            if loc: fields["location"] = loc.group(1).strip()
            result["fields"] = fields
            evt = fields.get("summary", "?")
            when = fields.get("start", "")
            result["formatted"] = f"Event: {evt}" + (f" @ {when}" if when else "")
            return result

        # Email: mailto:user@example.com?subject=...
        email_match = re.match(r"^mailto:(.+?)(\?|$)", content, re.I)
        if email_match:
            result["type"] = "email"
            addr = email_match.group(1)
            qs = {}
            if "?" in content:
                for part in content.split("?")[1].split("&"):
                    if "=" in part:
                        k, v = part.split("=", 1)
                        qs[k.lower()] = v
            result["fields"] = {"to": addr, **qs}
            subj = qs.get("subject", "")
            result["formatted"] = f"Email: {addr}" + (f" — subj: {subj}" if subj else "")
            return result

        # SMS: smsto:number?body=...
        sms_match = re.match(r"^smsto:(.+?)(\?|$)", content, re.I)
        if sms_match:
            result["type"] = "sms"
            num = sms_match.group(1)
            body = ""
            if "?" in content and "body=" in content:
                body = content.split("body=", 1)[1].split("&")[0]
            result["fields"] = {"number": num, "body": body}
            result["formatted"] = f"SMS: {num}" + (f" — {body[:40]}" if body else "")
            return result

        # tel: number
        tel_match = re.match(r"^tel:(.+)$", content, re.I)
        if tel_match:
            result["type"] = "tel"
            num = tel_match.group(1)
            result["fields"] = {"number": num}
            result["formatted"] = f"Tel: {num}"
            return result

        # geo: latitude,longitude
        geo_match = re.match(r"^geo:([-\d.]+),([-\d.]+)", content, re.I)
        if geo_match:
            result["type"] = "geo"
            lat, lon = geo_match.group(1), geo_match.group(2)
            result["fields"] = {"lat": lat, "lon": lon}
            result["formatted"] = f"Geo: {lat}, {lon}"
            return result

        # URL / URI
        url_match = re.match(r"^https?://", content, re.I)
        if url_match:
            result["type"] = "url"
            result["formatted"] = f"URL: {content[:80]}"
            result["fields"] = {"url": content}
            return result

        # Bitcoin: bitcoin:address?amount=...
        btc_match = re.match(r"^bitcoin:(.+?)(\?|$)", content, re.I)
        if btc_match:
            result["type"] = "bitcoin"
            addr = btc_match.group(1)
            result["fields"] = {"address": addr}
            amt = ""
            if "?" in content and "amount=" in content:
                amt = content.split("amount=", 1)[1].split("&")[0]
                result["fields"]["amount"] = amt
            result["formatted"] = f"Bitcoin: {addr[:20]}...{addr[-6:]}" + (f" ({amt} BTC)" if amt else "")
            return result

        return result

    # ── Deduplication helper ────────────────────────────────────
    @staticmethod
    def deduplicate_contents(contents: list) -> list:
        seen = set()
        result = []
        for c in contents:
            if c not in seen:
                seen.add(c)
                result.append(c)
        return result

    # =========================================================================
    # DETECTION METHODS
    # =========================================================================

    def _detect_barcodes(self, image: Image.Image, symbols: set = None) -> tuple[list, list, list]:
        """Detect barcodes (non-QR) using pyzbar with symbol filtering."""
        if symbols is None:
            symbols = ALL_SYMBOLS - {"QRCODE"}
        try:
            decoded = pyzbar.decode(image, symbols=symbols)
            return self._extract_all_symbols(decoded)
        except Exception:
            return [], [], []

    def _detect_qr_plus_barcodes(self, image: Image.Image, symbols: set = None) -> tuple[list, list, list]:
        """Detect QR + barcodes with optional symbol filtering."""
        if symbols is None:
            symbols = ALL_SYMBOLS
        try:
            decoded = pyzbar.decode(image, symbols=symbols)
            return self._extract_all_symbols(decoded)
        except Exception:
            return [], [], []

    # =========================================================================
    # DETECTION METHODS
    # =========================================================================

    def _detect_qr_method1(self, image: Image.Image) -> tuple[list, list]:
        """Method 1: Standard direct decode."""
        return self._try_decode(image)

    def _detect_qr_method2(self, image: Image.Image) -> tuple[list, list]:
        """Method 2: Grayscale conversion."""
        gray = image.convert("L")
        return self._try_decode(gray, close_img=True)

    def _detect_qr_method4_sharpen(self, image: Image.Image) -> tuple[list, list]:
        """Method 4: Sharpening for blurry QR codes using OpenCV."""
        try:
            import cv2
            import numpy as np

            img_array = np.array(image.convert("RGB"))

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
                    contents, bboxes = self._try_decode(result_img, close_img=True)
                    if contents:
                        return contents, bboxes
                except Exception:
                    continue

            for percent in [200, 250, 300]:
                try:
                    blurred = cv2.GaussianBlur(img_array, (0, 0), percent / 100)
                    sharpened = cv2.addWeighted(img_array, 1.5, blurred, -0.5, 0)
                    result_img = Image.fromarray(sharpened)
                    contents, bboxes = self._try_decode(result_img, close_img=True)
                    if contents:
                        return contents, bboxes
                except Exception:
                    continue

            return [], []
        except ImportError:
            return [], []
        except Exception:
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
                        contents, bboxes = self._try_decode(result_img, close_img=True)
                        if contents:
                            return contents, bboxes
                    except Exception:
                        continue

            try:
                deblurred = cv2.equalizeHist(gray)
                result_img = Image.fromarray(deblurred)
                contents, bboxes = self._try_decode(result_img, close_img=True)
                if contents:
                    return contents, bboxes
            except Exception:
                pass

            try:
                bilateral = cv2.bilateralFilter(gray, 9, 75, 75)
                result_img = Image.fromarray(bilateral)
                enhancer = ImageEnhance.Sharpness(result_img)
                enhanced = enhancer.enhance(2.0)
                result_img.close()
                contents, bboxes = self._try_decode(enhanced, close_img=True)
                if contents:
                    return contents, bboxes
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
            methods.append(lambda i, a=angle: i.rotate(a, expand=True))

        methods.append(lambda i: i.transpose(Image.FLIP_LEFT_RIGHT))
        methods.append(lambda i: i.transpose(Image.FLIP_TOP_BOTTOM))

        methods.append(lambda i: i.transpose(Image.TRANSPOSE))
        methods.append(lambda i: i.transpose(Image.TRANSVERSE))

        for processed in methods:
            try:
                img = processed(image)
                contents, bboxes = self._try_decode(img, close_img=True)
                if contents:
                    return contents, bboxes
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
                contents, bboxes = self._try_decode(scaled, close_img=True, bbox_scale=scale)
                if contents:
                    return contents, bboxes
            except Exception:
                continue

        return [], []

    def _detect_qr_method8_qreader(self, image: Image.Image) -> tuple[list, list]:
        """Method 8: Use QReader for difficult QR codes (requires qreader package)."""
        try:
            import numpy as np
            import cv2
            from qreader import QReader

            # Lazy singleton — QReader carica modello ML, costa ~2-5s la prima volta
            if not hasattr(self, "_qreader_instance"):
                self._qreader_instance = QReader()

            qreader = self._qreader_instance
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

    def _detect_qr_method9_adaptive(self, image: Image.Image) -> tuple[list, list]:
        """Method 9: Adaptive thresholding for difficult images."""
        try:
            import cv2
            import numpy as np

            img_array = np.array(image.convert("RGB"))
            gray = cv2.cvtColor(img_array, cv2.COLOR_RGB2GRAY)

            methods = [
                ("adaptive_mean", cv2.ADAPTIVE_THRESH_MEAN_C, 11, 2),
                ("adaptive_gaussian", cv2.ADAPTIVE_THRESH_GAUSSIAN_C, 11, 2),
                ("adaptive_large", cv2.ADAPTIVE_THRESH_GAUSSIAN_C, 21, 5),
                ("otsu", 0, 0, 0),
            ]

            for method_name, block_size, c, _ in methods:
                try:
                    if method_name == "otsu":
                        _, binary = cv2.threshold(
                            gray, 0, 255, cv2.THRESH_BINARY + cv2.THRESH_OTSU
                        )
                    else:
                        if block_size % 2 == 0:
                            block_size += 1
                        binary = cv2.adaptiveThreshold(
                            gray, 255, block_size, cv2.THRESH_BINARY, block_size, c
                        )

                    result_img = Image.fromarray(binary)
                    contents, bboxes = self._try_decode(result_img, close_img=True)
                    if contents:
                        return contents, bboxes
                except Exception:
                    continue

            return [], []
        except ImportError:
            return [], []
        except Exception:
            return [], []

    def _detect_qr_method10_morphology(self, image: Image.Image) -> tuple[list, list]:
        """Method 10: Morphological operations to enhance QR patterns."""
        try:
            import cv2
            import numpy as np

            img_array = np.array(image.convert("RGB"))
            gray = cv2.cvtColor(img_array, cv2.COLOR_RGB2GRAY)

            kernels = [
                np.ones((3, 3), np.uint8),
                np.ones((5, 5), np.uint8),
            ]

            operations = [
                ("dilate", cv2.MORPH_DILATE),
                ("erode", cv2.MORPH_ERODE),
                ("open", cv2.MORPH_OPEN),
                ("close", cv2.MORPH_CLOSE),
            ]

            for kernel in kernels:
                for op_name, op in operations:
                    try:
                        result = cv2.morphologyEx(gray, op, kernel)
                        result_img = Image.fromarray(result)
                        contents, bboxes = self._try_decode(result_img, close_img=True)
                        if contents:
                            return contents, bboxes
                    except Exception:
                        continue

            return [], []
        except ImportError:
            return [], []
        except Exception:
            return [], []

    def _detect_qr_method11_extreme_scale(
        self, image: Image.Image
    ) -> tuple[list, list]:
        """Method 11: Extreme scaling for very small or very large QR."""
        try:
            scales = [0.25, 0.35, 0.45, 4.0, 5.0, 6.0, 8.0]

            for scale in scales:
                try:
                    new_width = int(image.width * scale)
                    new_height = int(image.height * scale)
                    scaled = image.resize((new_width, new_height), Image.LANCZOS)
                    contents, bboxes = self._try_decode(scaled, close_img=True, bbox_scale=scale)
                    if contents:
                        return contents, bboxes
                except Exception:
                    continue

            return [], []
        except Exception:
            return [], []

    # =========================================================================
    # FAST DETECTION - OPTIMIZED FOR SPEED & ACCURACY
    # =========================================================================

    def _detect_phase1(self, img: Image.Image) -> tuple[list, list, str]:
        """Phase 1: Fast detection methods only."""

        # Method 1: Basic direct decode
        contents, bboxes = self._detect_qr_method1(img)
        if contents:
            return contents, bboxes, "basic"

        # Method 2: Grayscale
        contents, bboxes = self._detect_qr_method2(img)
        if contents:
            return contents, bboxes, "grayscale"

        # Methods 3-4: Contrast + Unsharp - with proper cleanup
        gray = None
        enhanced = None
        sharp = None
        try:
            gray = img.convert("L")
            enhancer = ImageEnhance.Contrast(gray)
            enhanced = enhancer.enhance(1.5)
            contents, bboxes = self._detect_qr_method1(enhanced)
            if contents:
                return contents, bboxes, "contrast"

            # Method 4: Unsharp mask
            sharp = enhanced.filter(
                ImageFilter.UnsharpMask(radius=2, percent=150, threshold=3)
            )
            contents, bboxes = self._detect_qr_method1(sharp)
            if contents:
                return contents, bboxes, "unsharp"
        finally:
            if sharp is not None:
                try:
                    sharp.close()
                except Exception:
                    pass
            if enhanced is not None:
                try:
                    enhanced.close()
                except Exception:
                    pass
            if gray is not None:
                try:
                    gray.close()
                except Exception:
                    pass

        # Method 5: Resize 2x LANCZOS
        scaled = None
        try:
            scaled = img.resize((img.width * 2, img.height * 2), Image.LANCZOS)
            contents, bboxes = self._detect_qr_method1(scaled)
            if contents:
                return contents, bboxes, "resize_2x_lanczos"
        except Exception:
            pass
        finally:
            if scaled is not None:
                try:
                    scaled.close()
                except Exception:
                    pass

        # Method 6: Resize 2x BICUBIC
        scaled = None
        try:
            scaled = img.resize((img.width * 2, img.height * 2), Image.BICUBIC)
            contents, bboxes = self._detect_qr_method1(scaled)
            if contents:
                return contents, bboxes, "resize_2x_bicubic"
        except Exception:
            pass
        finally:
            if scaled is not None:
                try:
                    scaled.close()
                except Exception:
                    pass

        return [], [], "phase1_exhausted"

    def _detect_phase2(self, img: Image.Image) -> tuple[list, list, str]:
        """Phase 2: Extra methods for more detection."""
        if not self.deep_scan:
            return [], [], "deep_disabled"

        contents, bboxes = self._detect_qr_method4_sharpen(img)
        if contents:
            return contents, bboxes, "method4_sharpen"

        contents, bboxes = self._detect_qr_method5_deblur(img)
        if contents:
            return contents, bboxes, "method5_deblur"

        gray = None
        try:
            gray = img.convert("L")

            # Method 7: Resize 3x
            scaled = None
            try:
                scaled = img.resize((img.width * 3, img.height * 3), Image.BICUBIC)
                contents, bboxes = self._detect_qr_method1(scaled)
                if contents:
                    return contents, bboxes, "resize_3x"
            except Exception:
                pass
            finally:
                if scaled is not None:
                    try:
                        scaled.close()
                    except Exception:
                        pass

            # Method 8: Gray + resize 3x
            scaled = None
            try:
                scaled = gray.resize((gray.width * 3, gray.height * 3), Image.LANCZOS)
                contents, bboxes = self._detect_qr_method1(scaled)
                if contents:
                    return contents, bboxes, "gray_resize_3x"
            except Exception:
                pass
            finally:
                if scaled is not None:
                    try:
                        scaled.close()
                    except Exception:
                        pass
        finally:
            if gray is not None:
                try:
                    gray.close()
                except Exception:
                    pass

        return [], [], "phase2_exhausted"

    def _detect_phase3(self, img: Image.Image) -> tuple[list, list, str]:
        """Phase 3: Heavy methods - only if force_deep enabled."""
        if not self.force_deep:
            return [], [], "force_deep_disabled"

        contents, bboxes = self._detect_qr_method6_rotation(img)
        if contents:
            return contents, bboxes, "method6_rotation"

        contents, bboxes = self._detect_qr_method7_multiscale(img)
        if contents:
            return contents, bboxes, "method7_multiscale"

        contents, bboxes = self._detect_qr_method8_qreader(img)
        if contents:
            return contents, bboxes, "qreader"

        contents, bboxes = self._detect_qr_method9_adaptive(img)
        if contents:
            return contents, bboxes, "method9_adaptive"

        contents, bboxes = self._detect_qr_method10_morphology(img)
        if contents:
            return contents, bboxes, "method10_morphology"

        return [], [], "phase3_exhausted"

    def _detect_full(self, img: Image.Image) -> tuple[list, list, str]:
        """Fallback: Extra extreme scales - only if force_deep enabled."""
        if not self.force_deep:
            return [], [], "full_disabled"

        for scale in [4, 5]:
            try:
                scaled = img.resize(
                    (img.width * scale, img.height * scale), Image.BICUBIC
                )
                try:
                    contents, bboxes = self._detect_qr_method1(scaled)
                    if contents:
                        return contents, bboxes, f"resize_{scale}x"
                finally:
                    scaled.close()
            except Exception:
                continue

        contents, bboxes = self._detect_qr_method11_extreme_scale(img)
        if contents:
            return contents, bboxes, "method11_extreme_scale"

        return [], [], "all_methods_exhausted"

    def detect_qr(self, image_path: Path, symbols: set = None, compute_score: bool = False) -> QRCodeResult:
        """Main detection with automatic escalation for failed images.

        Enforces *timeout* per image (0 = disabled).
        Short-circuits after SHORT_CIRCUIT_ATTEMPTS failed decode attempts
        to skip expensive Phase 3/Full when image likely has no QR.

        If *symbols* includes non-QRCODE types, barcode detection is also attempted.
        """
        contents = []
        bboxes = []
        symbologies = []
        detection_method = None
        all_symbols = symbols or self._symbols if hasattr(self, "_symbols") and self._symbols else DEFAULT_SYMBOLS
        self._decode_attempts = 0  # reset per image
        start_time = time.perf_counter()

        def _timeout_exceeded() -> bool:
            return self.timeout > 0 and (time.perf_counter() - start_time) > self.timeout

        def _check_timeout():
            if _timeout_exceeded():
                raise TimeoutError("Detection timed out")

        def _should_short_circuit() -> bool:
            return self._decode_attempts >= SHORT_CIRCUIT_ATTEMPTS

        try:
            with Image.open(image_path) as img:
                if self.verbose:
                    print(f"    Processing: {image_path.name}")

                if all_symbols == DEFAULT_SYMBOLS or "QRCODE" in all_symbols:
                    # Normal QR pipeline
                    _check_timeout()
                    contents, bboxes, method = self._detect_phase1(img)
                    if contents:
                        detection_method = method
                        if self.verbose:
                            print(f"    ✓ Detected in Phase 1: {method}")

                    if not contents and not _should_short_circuit():
                        _check_timeout()
                        contents, bboxes, method = self._detect_phase2(img)
                        if contents:
                            detection_method = method
                            if self.verbose:
                                print(f"    ✓ Detected in Phase 2: {method}")

                    if not contents and not _should_short_circuit():
                        _check_timeout()
                        contents, bboxes, method = self._detect_phase3(img)
                        if contents:
                            detection_method = method
                            if self.verbose:
                                print(f"    ✓ Detected in Phase 3: {method}")

                    if not contents and not _should_short_circuit():
                        _check_timeout()
                        contents, bboxes, method = self._detect_full(img)
                        if contents:
                            detection_method = method
                            if self.verbose:
                                print(f"    ✓ Detected in Full Phase: {method}")

                # Barcode pass (non-QR or combined)
                barcode_symbols = all_symbols - {"QRCODE"} if "QRCODE" in all_symbols else all_symbols
                if barcode_symbols and not contents:
                    _check_timeout()
                    bc_contents, bc_bboxes, bc_syms = self._detect_barcodes(img, symbols=barcode_symbols)
                    if bc_contents:
                        contents = bc_contents
                        bboxes = bc_bboxes
                        symbologies = bc_syms
                        detection_method = "barcode"
                        if self.verbose:
                            print(f"    ✓ Barcode detected: {set(bc_syms)}")

            result = QRCodeResult(
                str(image_path),
                has_qr=len(contents) > 0,
                qr_contents=contents,
                qr_bboxes=bboxes,
                symbologies=symbologies,
            )
            result.detection_method = detection_method
            result.attempts_made = (
                ["phase1", "phase2", "phase3", "full"]
                if self.force_deep
                else ["phase1"]
            )

            # Structured parsing & scannability
            if result.has_qr:
                result.parse_structured()
                if compute_score:
                    for i, c in enumerate(result.qr_contents):
                        result.scannability.append(
                            _compute_scannability_score(c)
                        )

            return result

        except TimeoutError:
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
            print(f"\n{_clr('⟳', 'cyan')} Auto-retrying {_clr(str(len(self._failed_images)), 'yellow', 'bold')} {_clr('failed images', 'yellow')} with enhanced detection")

        original_deep_scan = self.deep_scan
        original_force_deep = self.force_deep

        self.deep_scan = True
        self.force_deep = True

        with self._results_lock:
            failed_snapshot = list(self._failed_images)

        for result in failed_snapshot:
            if self.verbose:
                print(f"  Retrying: {result.file_path}")

            image_path = Path(result.file_path)
            if image_path.exists():
                retry_result = self.detect_qr(image_path)
                with self._results_lock:
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
                futures = {executor.submit(self.detect_qr, img, None, self._compute_score): img for img in images}
                completed_count = 0
                for future in as_completed(futures):
                    completed_count += 1
                    with self._results_lock:
                        self._scan_count = completed_count
                    img_path = futures[future]
                    result = future.result(timeout=None)
                    with self._results_lock:
                        self.results.append(result)

                    if progress:
                        if result.has_qr:
                            status = _clr("✓", "green", "bold")
                        else:
                            status = _clr("✗", "red") if result.error else _clr("✗", "yellow")
                        print(
                            f"[{completed_count}/{len(images)}] {img_path.name} {status}"
                        )
                    elif sys.stdout.isatty():
                        print(_spinner_text(completed_count, completed_count, len(images)), end="", flush=True)

                    if not result.has_qr and not result.error:
                        with self._results_lock:
                            self._failed_images.append(result)
        else:
            for i, img in enumerate(images):
                self._scan_count = i + 1
                result = self.detect_qr(img, compute_score=self._compute_score)
                self.results.append(result)

                if progress:
                    if result.has_qr:
                        status = _clr("✓", "green", "bold")
                    else:
                        status = _clr("✗", "red") if result.error else _clr("✗", "yellow")
                    print(f"[{self._scan_count}/{len(images)}] {img.name} {status}")
                elif sys.stdout.isatty():
                    print(_spinner_text(i + 1, i + 1, len(images)), end="", flush=True)

                if not result.has_qr and not result.error:
                    with self._results_lock:
                        self._failed_images.append(result)

        # Clear spinner line when done (if not showing per-file progress)
        if not progress and sys.stdout.isatty():
            print("\r" + " " * 50 + "\r", end="", flush=True)

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

        action = os.replace if move else shutil.copy2

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
                with Image.open(r.file_path) as src_img:
                    img_width, img_height = src_img.size
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
            except Exception as e:
                if self.log_file:
                    self._log(f"Error opening {r.file_path}: {e}")
                continue

        print(f"Extracted {qr_count} QR code regions in {output_path}")
        return qr_count

    def action_list(self, json_output: bool = False) -> None:
        with_qr = self._get_with_qr()
        without_qr = self._get_without_qr()
        failed = self._get_failed()

        total = len(self.results)
        detection_rate = (len(with_qr) / total * 100) if total else 0

        if json_output:
            data = {
                "total": total,
                "with_qr": len(with_qr),
                "without_qr": len(without_qr),
                "failed": len(failed),
                "detection_rate": round(detection_rate, 1),
                "results": [r.to_dict(structured=True) for r in self.results],
            }
            print(json.dumps(data, indent=2))
            return

        print()
        print(_box_top())
        print(_panel(f"{_clr('⏺', 'cyan')}  {_clr('QR Multi IMGS', 'cyan', 'bold')} {_clr('─', 'dim')} {_clr('Scan Results', 'dim')}"))
        print(_box_mid())
        print(_panel(f"{_clr('Total images:', 'bold')}       {total}"))
        if with_qr:
            w_bar = _clr(_bar(len(with_qr), total, 10), "green")
            print(_panel(f"{_clr('With QR codes:', 'green', 'bold')}  {w_bar}  {_clr(str(len(with_qr)), 'green', 'bold')}"))
        if without_qr:
            wo_bar = _clr(_bar(len(without_qr), total, 10), "yellow")
            print(_panel(f"{_clr('Without QR codes:', 'yellow')}  {wo_bar}  {len(without_qr)}"))
        if failed:
            f_bar = _clr(_bar(len(failed), total, 10), "red")
            print(_panel(f"{_clr('Failed/Error:', 'red')}      {f_bar}  {len(failed)}"))
        rate_filled = int(detection_rate / 10)
        rate_bar = _clr("█", "cyan") * rate_filled + _clr("░", "dim") * (10 - rate_filled)
        print(_panel(f"{_clr('Detection rate:', 'bold')}   {rate_bar}  {_clr(f'{detection_rate:.1f}%', 'cyan', 'bold')}"))
        print(_box_bot())

        if with_qr:
            print(f"\n{_section('WITH QR codes', 'green')}")
            for r in with_qr:
                qr_preview = (
                    r.qr_contents[0][:80] + "..."
                    if len(r.qr_contents[0]) > 80
                    else r.qr_contents[0]
                )
                method_info = (
                    f" ({r.detection_method})"
                    if r.detection_method and self.verbose
                    else ""
                )
                print(f"  {_clr('✓', 'green', 'bold')} {r.file_path}{method_info}")
                # Show structured decode if available
                if r.structured:
                    s = r.structured[0]
                    if s["type"] != "unknown":
                        print(f"    {_clr('type:', 'dim')} {_clr(s['type'], 'cyan')} → {s['formatted'][:100]}")
                    else:
                        print(f"    {_clr('QR:', 'dim')} {qr_preview}")
                else:
                    print(f"    {_clr('QR:', 'dim')} {qr_preview}")
                # Show scannability score
                if r.scannability:
                    sc = r.scannability[0]
                    grade_color = {"A": "green", "B": "cyan", "C": "yellow", "D": "red", "F": "red"}
                    gc = grade_color.get(sc["grade"], "dim")
                    score_text = "{}/100 ({})".format(sc["score"], sc["grade"])
                    print(f"    {_clr('score:', 'dim')} {_clr(score_text, gc)}")
                # Render QR in terminal if requested
                show_qr_flag = getattr(self, "_show_qr", False)
                if show_qr_flag and r.qr_contents:
                    print(f"    {_clr(_qr_to_terminal(r.qr_contents[0]), 'dim')}")
                if len(r.qr_contents) > 1:
                    print(f"    {_clr(f'+{len(r.qr_contents) - 1} more codes', 'dim')}")

        if without_qr:
            print(f"\n{_section('WITHOUT QR codes', 'yellow')}")
            for r in without_qr:
                print(f"  {_clr('✗', 'yellow')} {r.file_path}")

        if failed and self.verbose:
            print(f"\n{_section('Failed images (errors)', 'red')}")
            for r in failed:
                print(f"  {_clr('!', 'red', 'bold')} {r.file_path}: {_clr(r.error, 'red')}")

    def action_decode(self, output_format: str = "text", json_output: bool = False) -> list:
        with_qr = self._get_with_qr()

        if not with_qr:
            if json_output:
                print(json.dumps({"error": "No QR codes found", "count": 0}))
            else:
                print(f"{_clr('◇', 'yellow')} No QR codes found to decode")
            return []

        if json_output or output_format == "json":
            results = []
            for r in with_qr:
                r.parse_structured()
                entry = {
                    "file": r.file_path,
                    "qr_codes": r.qr_contents,
                    "structured": [s.get("formatted", s["raw"]) for s in r.structured],
                    "symbologies": r.symbologies,
                    "count": len(r.qr_contents),
                    "method": r.detection_method,
                }
                results.append(entry)
            print(json.dumps(results, indent=2))
        else:
            for r in with_qr:
                for i, content in enumerate(r.qr_contents):
                    method_info = (
                        f" [{r.detection_method}]" if r.detection_method else ""
                    )
                    sym = f" ({r.symbologies[i]})" if i < len(r.symbologies) and r.symbologies[i] != "QRCODE" else ""
                    parsed = QRMultiIMGS.parse_structured_content(content)
                    display = parsed.get("formatted", content)
                    if len(r.qr_contents) > 1:
                        print(f"{r.file_path} [{i + 1}]{sym}{method_info}: {display}")
                    else:
                        print(f"{r.file_path}{sym}{method_info}: {display}")

        total_qr = sum(len(r.qr_contents) for r in with_qr)
        if not json_output:
            print(f"\nTotal: {len(with_qr)} images with {total_qr} codes")
        return with_qr

    def action_filter(
        self, pattern: str, case_sensitive: bool = False, exclude: bool = False, json_output: bool = False
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

        results = non_matching if exclude else matching

        if json_output:
            label = "exclude" if exclude else "include"
            print(json.dumps({
                "pattern": pattern,
                "mode": label,
                "count": len(results),
                "results": [
                    {"file": r.file_path, "qr_contents": r.qr_contents} for r in results
                ],
            }, indent=2))
            return results

        label = "NOT matching" if exclude else "matching"
        print(f"\n--- Images {label} '{pattern}' ---")

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
        is_valid, error = _validate_path(str(self.folder_path))
        if not is_valid:
            raise ValueError(f"Invalid folder path: {error}")

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

        is_valid, error = _validate_path(recreated_folder)
        if not is_valid:
            print(f"Error: {error}")
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
                with Image.open(original_img) as orig_img:
                    orig_decoded = pyzbar.decode(orig_img)
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
                with Image.open(recreated_file) as recon_img:
                    decoded = pyzbar.decode(recon_img)
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


# ── Webcam scanning ───────────────────────────────────────────
def _run_webcam(symbols: set = None, dedup: bool = False, json_output: bool = False):
    """Scan QR/barcodes from webcam in real-time (opencv required)."""
    try:
        import cv2
    except ImportError:
        print("Error: opencv-python required for webcam scanning")
        print("  pip install opencv-python")
        sys.exit(2)

    print(f"{_clr('◇', 'cyan')} Webcam scanning — press {_clr('q', 'bold')} to quit")
    print(_clr("─" * 40, "dim"))

    cap = cv2.VideoCapture(0)
    if not cap.isOpened():
        print("Error: Could not open webcam")
        sys.exit(2)

    seen = set()
    try:
        while True:
            ret, frame = cap.read()
            if not ret:
                break

            rgb = cv2.cvtColor(frame, cv2.COLOR_BGR2RGB)
            pil_img = Image.fromarray(rgb)
            try:
                decoded = pyzbar.decode(pil_img, symbols=symbols or ALL_SYMBOLS)
                for d in decoded:
                    content = d.data.decode("utf-8", errors="ignore")
                    sym = d.type
                    if dedup and content in seen:
                        continue
                    seen.add(content)
                    parsed = QRMultiIMGS.parse_structured_content(content)
                    display = parsed.get("formatted", content)
                    if json_output:
                        print(json.dumps({
                            "type": sym, "content": content,
                            "structured": parsed.get("formatted"),
                            "timestamp": datetime.now().isoformat(),
                        }))
                    else:
                        print(f"  {_clr('✓', 'green', 'bold')} [{sym}] {display}")
            finally:
                pil_img.close()

            if cv2.waitKey(1) & 0xFF == ord("q"):
                break
    except KeyboardInterrupt:
        pass
    finally:
        cap.release()
        cv2.destroyAllWindows()

    count = len(seen)
    if not json_output:
        print(f"\n{_clr('◇', 'cyan')} Scanned {count} unique code(s)")


# ── Folder watcher ────────────────────────────────────────────
def _watch_folder(
    folder_path: str,
    recursive: bool = False,
    formats: set = None,
    symbols: set = None,
    dedup: bool = False,
    json_output: bool = False,
    verbose: bool = False,
):
    """Watch a folder for new images and scan them automatically."""
    from time import sleep

    formats = formats or SUPPORTED_FORMATS
    path = Path(folder_path)
    if not path.exists():
        print(f"Error: Folder not found: {folder_path}")
        sys.exit(2)

    seen = set()
    print(f"{_clr('◇', 'cyan')} Watching: {_clr(str(path), 'bold')}")
    print(f"  New files will be scanned automatically. Press Ctrl+C to stop.")
    print(_clr("─" * 40, "dim"))

    try:
        while True:
            if recursive:
                candidates = [f for ext in formats for f in path.rglob(f"*{ext}")]
            else:
                candidates = [f for ext in formats for f in path.glob(f"*{ext}")]

            for img_path in candidates:
                str_path = str(img_path.resolve())
                if str_path in seen:
                    continue
                seen.add(str_path)
                try:
                    with Image.open(img_path) as img:
                        decoded = pyzbar.decode(img, symbols=symbols or ALL_SYMBOLS)
                        for d in decoded:
                            content = d.data.decode("utf-8", errors="ignore")
                            sym = d.type
                            parsed = QRMultiIMGS.parse_structured_content(content)
                            display = parsed.get("formatted", content)
                            ts = datetime.now().strftime("%H:%M:%S")
                            if json_output:
                                print(json.dumps({
                                    "file": str_path, "type": sym,
                                    "content": content,
                                    "structured": parsed.get("formatted"),
                                    "timestamp": datetime.now().isoformat(),
                                }))
                            else:
                                print(f"  [{ts}] {_clr('✓', 'green', 'bold')} {img_path.name} [{sym}] {display}")
                except Exception as e:
                    if verbose:
                        print(f"  [{ts}] {_clr('✗', 'red')} {img_path.name}: {e}")

            sleep(1.0)
    except KeyboardInterrupt:
        if not json_output:
            print(f"\n{_clr('◇', 'cyan')} Watcher stopped. Scanned {len(seen)} file(s).")


# ── Terminal QR output (Unicode blocks) ──────────────────────
def _qr_to_terminal(content: str, version: int = 1) -> str:
    """Render a QR code as Unicode block characters in terminal."""
    try:
        qr = qrcode.QRCode(version=version, error_correction=qrcode.constants.ERROR_CORRECT_L)
        qr.add_data(content)
        qr.make(fit=True)
        matrix = qr.get_matrix()
        lines = []
        header = f"  ╭{'──' * len(matrix[0])}╮"
        lines.append(header)
        for row in matrix:
            blocks = "".join("██" if cell else "  " for cell in row)
            lines.append(f"  │{blocks}│")
        footer = f"  ╰{'──' * len(matrix[0])}╯"
        lines.append(footer)
        return "\n".join(lines)
    except Exception:
        return f"  [QR: {content[:40]}...]"


# ── Scannability score ────────────────────────────────────────
def _compute_scannability_score(content: str, img: Image.Image = None) -> dict:
    """Evaluate how robust/scannable a QR code is. Returns dict with score (0-100) and details."""
    score = 100
    details = []

    # Content-based checks
    content_len = len(content)
    if content_len > 1000:
        penalty = min(20, (content_len - 1000) // 100)
        score -= penalty
        details.append(f"Long content ({content_len} chars): -{penalty}")
    if content_len < 5:
        score -= 10
        details.append(f"Very short content: -10")

    # Character variety
    special = sum(not c.isalnum() and not c.isspace() for c in content)
    if special > content_len * 0.3:
        score -= 5
        details.append(f"High special-char ratio: -5")

    # Image-based checks (if image provided)
    if img is not None:
        w, h = img.size
        # Very small QR region
        min_dim = min(w, h)
        if min_dim < 50:
            score -= 15
            details.append(f"Small region ({min_dim}px): -15")
        elif min_dim < 100:
            score -= 5
            details.append(f"Moderate size ({min_dim}px): -5")

        # Check image mode / color
        if img.mode != "L" and img.mode != "1":
            score -= 3
            details.append(f"Non-grayscale image: -3")

    score = max(0, min(100, score))
    grade = "A" if score >= 90 else "B" if score >= 75 else "C" if score >= 50 else "D" if score >= 25 else "F"
    return {"score": score, "grade": grade, "details": details}


# ── Shell completion generator ────────────────────────────────
def _generate_completion(shell: str):
    """Print shell completion script for bash/zsh/fish."""
    scripts = {
        "bash": """_qr_multi_imgs_completion() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="--path -p --action -a --recursive -r --formats -f --output -o --export-format --qr-format --move --confirm --parallel --progress --log --nomenu --naming --timeout -t --padding --deep-scan --deep-timeout --verbose -v --force-deep --rename-prefix --rename-suffix --filter-pattern --filter-case-sensitive --filter-exclude --json --dedup --symbols -s --completion --score"
    actions="list export delete organize recreate extract decode filter batch-rename verify webcam watch"
    case "${prev}" in
        --action|-a) COMPREPLY=( $(compgen -W "${actions}" -- ${cur}) ) ;;
        --export-format) COMPREPLY=( $(compgen -W "txt json csv" -- ${cur}) ) ;;
        --qr-format) COMPREPLY=( $(compgen -W "png svg pdf" -- ${cur}) ) ;;
        --naming) COMPREPLY=( $(compgen -W "original content sequential" -- ${cur}) ) ;;
        --completion) COMPREPLY=( $(compgen -W "bash zsh fish" -- ${cur}) ) ;;
        --symbols|-s) COMPREPLY=( $(compgen -W "QRCODE EAN13 EAN8 CODE128 CODE39 CODE93 I25 DATABAR PDF417 AZTEC" -- ${cur}) ) ;;
        *) COMPREPLY=($(compgen -W "${opts}" -- ${cur})) ;;
    esac
    return 0
}
complete -F _qr_multi_imgs_completion qr_multi_imgs.py
""",
        "zsh": """#compdef qr_multi_imgs.py
_qr_multi_imgs_completion() {
    local -a opts
    opts=(
        '--path[Folder path to scan]:folder:_files -/'
        '--action[Action to perform]:action:(list export delete organize recreate extract decode filter batch-rename verify webcam watch)'
        '--recursive[Scan subfolders]'
        '--formats[Image formats (comma-separated)]'
        '--output[Output folder path]:folder:_files -/'
        '--export-format[Export format]:(txt json csv)'
        '--qr-format[QR image format]:(png svg pdf)'
        '--move[Move files instead of copy]'
        '--confirm[Skip confirmation prompt]'
        '--parallel[Process images in parallel]'
        '--progress[Show progress during scan]'
        '--log[Save log to file]'
        '--nomenu[Skip interactive menu]'
        '--naming[Filename style]:(original content sequential)'
        '--timeout[Timeout per image]'
        '--deep-scan[Enable enhanced detection]'
        '--deep-timeout[Deep scan timeout]'
        '--verbose[Show detailed progress and errors]'
        '--force-deep[Use maximum detection methods]'
        '--json[Output JSON to stdout]'
        '--dedup[Deduplicate contents]'
        '--symbols[Symbologies to detect]'
        '--completion[Shell completion]:(bash zsh fish)'
        '--score[Show scannability score]'
    )
    _arguments $opts
}
_qr_multi_imgs_completion
""",
        "fish": """function _qr_multi_imgs_completion
    set -l cmds __fish_make_completion_cmd
    complete -c qr_multi_imgs.py -l path -s p -d 'Folder path to scan' -r
    complete -c qr_multi_imgs.py -l action -s a -d 'Action to perform' -xa 'list export delete organize recreate extract decode filter batch-rename verify webcam watch'
    complete -c qr_multi_imgs.py -l recursive -s r -d 'Scan subfolders'
    complete -c qr_multi_imgs.py -l output -s o -d 'Output folder path' -r
    complete -c qr_multi_imgs.py -l export-format -d 'Export format' -xa 'txt json csv'
    complete -c qr_multi_imgs.py -l qr-format -d 'QR image format' -xa 'png svg pdf'
    complete -c qr_multi_imgs.py -l json -d 'Output JSON to stdout'
    complete -c qr_multi_imgs.py -l dedup -d 'Deduplicate contents'
    complete -c qr_multi_imgs.py -l symbols -s s -d 'Symbologies to detect' -r
    complete -c qr_multi_imgs.py -l completion -d 'Shell completion' -xa 'bash zsh fish'
    complete -c qr_multi_imgs.py -l score -d 'Show scannability score'
end
""",
    }
    print(scripts.get(shell, ""))


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
        sys.exit(2)

    if getattr(args, "no_color", False):
        _set_color(False)

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
    # Pass symbols to scanner for barcode support
    raw_symbols = getattr(args, "symbols", None)
    if raw_symbols:
        scanner._symbols = {s.upper() for s in raw_symbols.split(",")}
    else:
        scanner._symbols = None

    dedup = getattr(args, "dedup", False)
    compute_score = getattr(args, "score", False)
    show_qr = getattr(args, "show_qr", False)
    scanner._compute_score = compute_score
    scanner._show_qr = show_qr

    print(f"{_clr('◇', 'cyan')} Scanning: {_clr(args.path, 'bold')}")
    if verbose:
        mode = "Full (force-deep)" if force_deep else "Enhanced (deep-scan)"
        print(f"  {_clr('mode:', 'dim')} {mode}")
        if scanner._symbols and scanner._symbols != DEFAULT_SYMBOLS:
            print(f"  {_clr('symbols:', 'dim')} {', '.join(sorted(scanner._symbols))}")
        if compute_score:
            print(f"  {_clr('score:', 'dim')} scannability scoring enabled")
    print(_clr("─" * 50, "dim"))

    results = scanner.scan(progress=args.progress)

    # Deduplicate if requested
    if dedup:
        for r in results:
            orig_count = len(r.qr_contents)
            r.qr_contents = QRMultiIMGS.deduplicate_contents(r.qr_contents)
            if len(r.qr_contents) < orig_count:
                r.has_qr = len(r.qr_contents) > 0
                if r.has_qr and verbose:
                    print(f"  {_clr('⊘', 'yellow')} Dedup: {orig_count - len(r.qr_contents)} duplicate(s) in {r.file_path}")

    # Compute exit code
    with_qr = [r for r in results if r.has_qr]
    failed = [r for r in results if r.error]

    if args.action == "decode":
        # For decode, exit code reflects whether QR content was found
        scanner.action_decode(output_format=args.export_format, json_output=args.json_output)
        _exit_code = EC_OK if any(r.has_qr for r in results) else EC_NO_QR if not failed else EC_ERROR
        sys.exit(_exit_code)
    elif args.action == "list":
        scanner.action_list(json_output=args.json_output)
        _exit_code = EC_OK if with_qr else EC_NO_QR if not failed else EC_ERROR
        sys.exit(_exit_code)
    elif args.action == "export":
        scanner.export_list(format=args.export_format, output_path=args.output)
        sys.exit(EC_OK)
    elif args.action == "delete":
        scanner.action_delete(output_folder=args.output, confirm=args.confirm)
        sys.exit(EC_OK)
    elif args.action == "organize":
        scanner.action_organize(
            output_folder=args.output, move=args.move, confirm=args.confirm
        )
        sys.exit(EC_OK)
    elif args.action == "recreate":
        scanner.action_recreate(output_folder=args.output, naming=args.naming)
        sys.exit(EC_OK)
    elif args.action == "extract":
        scanner.action_extract(
            output_folder=args.output, naming=args.naming, padding=args.padding
        )
        sys.exit(EC_OK)
    elif args.action == "filter":
        if not args.filter_pattern:
            print("Error: --filter-pattern is required for filter action")
            sys.exit(2)
        scanner.action_filter(
            pattern=args.filter_pattern,
            case_sensitive=args.filter_case_sensitive,
            exclude=args.filter_exclude,
            json_output=args.json_output,
        )
        sys.exit(EC_OK)
    elif args.action == "batch-rename":
        result = scanner.action_batch_rename(
            prefix=args.rename_prefix or "",
            suffix=args.rename_suffix or "",
            dry_run=not args.confirm,
        )
        if not args.confirm:
            print("\nNo files were renamed (dry run). Use --confirm to apply changes.")
        sys.exit(EC_OK)
    elif args.action == "verify":
        scanner.action_verify(originals_folder=args.path, recreated_folder=args.output)
        sys.exit(EC_OK)
    elif args.action == "webcam":
        _run_webcam(
            symbols=scanner._symbols,
            dedup=dedup,
            json_output=args.json_output,
        )
    elif args.action == "watch":
        _watch_folder(
            folder_path=args.path,
            recursive=args.recursive,
            formats=formats,
            symbols=scanner._symbols,
            dedup=dedup,
            json_output=args.json_output,
            verbose=verbose,
        )


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


def _make_args(parser, **overrides):
    """Build argparse Namespace from parser defaults + overrides.

    Gets all argument defaults from *parser*, then applies **overrides.
    Eliminates the manual Namespace duplication in _run_interactive_menu.
    """
    ns = parser.parse_known_args([])[0]
    for k, v in overrides.items():
        setattr(ns, k, v)
    return ns


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

    print("\nEnhanced Detection (recommended for difficult QR codes)? (Y/n): ", end="")
    deep_scan_input = input().strip().lower()
    deep_scan = deep_scan_input not in ("n", "no")

    if deep_scan:
        print("Show detailed progress (method per image)? (Y/n): ", end="")
        verbose_input = input().strip().lower()
        verbose = verbose_input in ("y", "yes")

        print("Use MAXIMUM detection (slower but best results)? (Y/n): ", end="")
        force_deep_input = input().strip().lower()
        force_deep = force_deep_input in ("y", "yes")
    else:
        verbose = False
        force_deep = False

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

    new_args = _make_args(
        parser,
        path=folder,
        action=action,
        recursive=recursive,
        progress=True,
        nomenu=True,
        deep_scan=deep_scan,
        verbose=verbose,
        force_deep=force_deep,
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
            "webcam",
            "watch",
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
        "--no-color", action="store_true", help="Disable colored output (overrides NO_COLOR)"
    )
    parser.add_argument(
        "--force-deep", action="store_true", help="Use maximum detection methods"
    )
    parser.add_argument(
        "--json", dest="json_output", action="store_true",
        help="Output results as JSON to stdout (scriptable)"
    )
    parser.add_argument(
        "--dedup", action="store_true",
        help="Deduplicate identical QR/barcode contents"
    )
    parser.add_argument(
        "--symbols", "-s",
        help="Comma-separated symbologies to detect (default: QRCODE). "
             f"All: {', '.join(sorted(ALL_SYMBOLS))}",
    )
    parser.add_argument(
        "--completion",
        choices=["bash", "zsh", "fish"],
        help="Generate shell completion script",
    )
    parser.add_argument(
        "--score", action="store_true",
        help="Show scannability score for each detected QR code"
    )
    parser.add_argument(
        "--show-qr", action="store_true",
        help="Render decoded QR codes as Unicode art in terminal"
    )

    args = parser.parse_args()

    # Handle --no-color early so all output (--completion, --score banner, etc.) respects it
    if args.no_color:
        _set_color(False)

    # Handle --completion
    if args.completion:
        _generate_completion(args.completion)
        return

    # Handle --score: force verbose and deep for detailed analysis
    if args.score:
        setattr(args, "verbose", True)
        setattr(args, "force_deep", True)

    if args.path:
        run_cli(args)
        return

    _run_interactive_menu(args, parser)


if __name__ == "__main__":
    main()
