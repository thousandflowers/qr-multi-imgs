#!/usr/bin/env python3
"""
QR Multi IMG - QR Code Scanner for Images
Version: v0.1.0 (beta)
Author: QR Multi IMG Team
License: MIT
"""

import os
import sys
import json
import csv
import logging
import argparse
import signal
from pathlib import Path
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor, as_completed

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
from PIL import Image, ImageEnhance
import pyzbar.pyzbar as pyzbar

SUPPORTED_FORMATS = {".jpg", ".jpeg", ".png", ".bmp", ".gif", ".webp", ".tiff", ".tif"}
DEFAULT_QR_FORMAT = "png"
VERSION = "v0.1.0"


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
        }


class QRMultiIMG:
    """
    QR Code Scanner for Images.

    Scans a folder of images and detects QR codes using pyzbar.
    Supports multiple detection methods for difficult images.

    Attributes:
        folder_path: Path to folder to scan
        recursive: Whether to scan subfolders
        formats: Set of allowed image extensions
        parallel: Use thread pool for faster scanning
        log_file: Enable logging to file
        qr_format: Output format for recreated QR codes
        timeout: Seconds per image before timeout (0=disabled)
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
        timeout: int = 30,
    ):
        self.folder_path = Path(folder_path)
        self.recursive = recursive
        self.formats = formats or SUPPORTED_FORMATS
        self.parallel = parallel
        self.log_file = log_file
        self.qr_format = qr_format.lower()
        self.timeout = timeout
        self.results: list[QRCodeResult] = []
        self.logger = None
        self._scan_count = 0
        self._total_count = 0

        if log_file:
            self._setup_logger()

    def _setup_logger(self):
        logging.basicConfig(
            filename="qr_scanner.log",
            level=logging.INFO,
            format="%(asctime)s - %(levelname)s - %(message)s",
            filemode="a",  # Append mode
        )
        self.logger = logging.getLogger(__name__)

    def _log(self, message: str):
        if self.logger:
            self.logger.info(message)
        print(message)

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
        image = enhancer.enhance(1.5)
        enhancer = ImageEnhance.Sharpness(image)
        image = enhancer.enhance(1.5)
        if image.mode not in ("RGB", "L"):
            image = image.convert("RGB")
        return image

    def _detect_qr_method1(self, image: Image.Image) -> tuple[list, list]:
        decoded = pyzbar.decode(image)
        contents = [d.data.decode("utf-8", errors="ignore") for d in decoded]
        bboxes = []
        for d in decoded:
            bbox = d.rect
            bboxes.append((bbox.left, bbox.top, bbox.width, bbox.height))
        return contents, bboxes

    def _detect_qr_method2(self, image: Image.Image) -> tuple[list, list]:
        gray = image.convert("L")
        decoded = pyzbar.decode(gray)
        contents = [d.data.decode("utf-8", errors="ignore") for d in decoded]
        bboxes = []
        for d in decoded:
            bbox = d.rect
            bboxes.append((bbox.left, bbox.top, bbox.width, bbox.height))
        return contents, bboxes

    def _detect_qr_method3(
        self, img: Image.Image, attempt: int = 0
    ) -> tuple[list, list, str]:
        methods = []

        # Reuse the already opened image instead of reopening
        methods.append(lambda i: i)  # Original as-is
        methods.append(lambda i: i.convert("RGB"))  # Convert to RGB
        methods.append(lambda i: self._preprocess_image(i))  # Preprocess

        try:
            # For resize, we need a new image
            methods.append(
                lambda i: i.resize((i.width * 2, i.height * 2), Image.LANCZOS)
            )
        except Exception:
            pass

        if attempt >= len(methods):
            return [], [], "all_methods_exhausted"

        try:
            processed_img = methods[attempt](img)
            decoded = pyzbar.decode(processed_img)
            contents = [d.data.decode("utf-8", errors="ignore") for d in decoded]
            bboxes = []
            for d in decoded:
                bbox = d.rect
                bboxes.append((bbox.left, bbox.top, bbox.width, bbox.height))
            return contents, bboxes, None
        except Exception as e:
            return [], [], str(e)

    def detect_qr(self, image_path: Path) -> QRCodeResult:
        import signal
        import platform
        import functools

        contents = []
        bboxes = []

        # Check if signal is available (not on Windows)
        use_signal = self.timeout > 0 and platform.system() != "Windows"

        # Setup timeout handler
        def timeout_handler(signum, frame):
            raise TimeoutError(f"Timeout processing {image_path}")

        try:
            # Set timeout alarm on Unix systems only
            if use_signal:
                signal.signal(signal.SIGALRM, timeout_handler)
                signal.alarm(self.timeout)

            img = Image.open(image_path)
            contents, bboxes = self._detect_qr_method1(img)

            if not contents:
                contents, bboxes = self._detect_qr_method2(img)

            if not contents:
                for attempt in range(4):
                    contents, bboxes, error = self._detect_qr_method3(img, attempt)
                    if contents:
                        break
                    if error == "all_methods_exhausted":
                        break

            # Cancel alarm if still pending (Unix only)
            if use_signal:
                signal.alarm(0)

            # Close the image when done to prevent memory leaks
            img.close()

            return QRCodeResult(
                str(image_path),
                has_qr=len(contents) > 0,
                qr_contents=contents,
                qr_bboxes=bboxes,
            )

        except TimeoutError as e:
            if self.log_file:
                self._log(f"Timeout processing {image_path}: {self.timeout}s exceeded")
            return QRCodeResult(str(image_path), has_qr=False, error="Timeout error")
        except Exception as e:
            if self.log_file:
                self._log(f"Error processing {image_path}: {e}")
            return QRCodeResult(str(image_path), has_qr=False, error=str(e))

    def scan(self, progress: bool = True) -> list[QRCodeResult]:
        images = self._get_images()
        self._total_count = len(images)

        if not images:
            return []

        if self.parallel:
            # Use ordered completion tracking instead of index-based
            with ThreadPoolExecutor() as executor:
                futures = {executor.submit(self.detect_qr, img): img for img in images}
                completed_count = 0
                for future in as_completed(futures):
                    completed_count += 1
                    self._scan_count = completed_count
                    img_path = futures[future]
                    if progress:
                        # Show completed/total instead of index (order is non-deterministic in parallel)
                        print(
                            f"Scanning {completed_count}/{len(images)}: {img_path.name}"
                        )
                    self.results.append(future.result())
        else:
            for i, img in enumerate(images):
                self._scan_count = i + 1
                if progress:
                    print(f"Scanning {self._scan_count}/{len(images)}: {img.name}")
                self.results.append(self.detect_qr(img))

        return self.results

    def export_list(self, format: str = "txt", output_path: str = None) -> str:
        if not output_path:
            output_path = f"qr_results.{format}"

        with_qr = [r for r in self.results if r.has_qr]
        without_qr = [r for r in self.results if not r.has_qr]

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
                    },
                    f,
                    indent=2,
                )

        elif format == "csv":
            with open(output_path, "w", newline="") as f:
                writer = csv.writer(f)
                writer.writerow(
                    ["file_path", "has_qr", "qr_contents", "file_size", "timestamp"]
                )
                for r in self.results:
                    writer.writerow(
                        [
                            r.file_path,
                            r.has_qr,
                            "|".join(r.qr_contents),
                            r.file_size,
                            r.timestamp,
                        ]
                    )

        else:
            with open(output_path, "w") as f:
                f.write(f"QR Multi IMG Scan Results\n")
                f.write(f"{'=' * 50}\n\n")
                f.write(f"Total images scanned: {len(self.results)}\n")
                f.write(f"With QR codes: {len(with_qr)}\n")
                f.write(f"Without QR codes: {len(without_qr)}\n\n")

                if with_qr:
                    f.write(f"{'-' * 50}\n")
                    f.write(f"IMAGES WITH QR CODES:\n")
                    f.write(f"{'-' * 50}\n")
                    for r in with_qr:
                        f.write(f"\nFile: {r.file_path}\n")
                        f.write(f"Size: {r.file_size} bytes\n")
                        f.write(f"QR Contents: {', '.join(r.qr_contents)}\n")

                if without_qr:
                    f.write(f"\n{'-' * 50}\n")
                    f.write(f"IMAGES WITHOUT QR CODES:\n")
                    f.write(f"{'-' * 50}\n")
                    for r in without_qr:
                        f.write(f"File: {r.file_path}\n")

        return output_path

    def action_delete(self, output_folder: str = None, confirm: bool = False) -> int:
        without_qr = [r for r in self.results if not r.has_qr and not r.error]

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
        base_path = Path(output_folder) if output_folder else self.folder_path

        with_qr_folder = base_path / "with_qr"
        without_qr_folder = base_path / "without_qr"

        with_qr_folder.mkdir(parents=True, exist_ok=True)
        without_qr_folder.mkdir(parents=True, exist_ok=True)

        with_qr = [r for r in self.results if r.has_qr]
        without_qr = [r for r in self.results if not r.has_qr and not r.error]

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
        output_path.mkdir(parents=True, exist_ok=True)

        with_qr = [r for r in self.results if r.has_qr]

        if not with_qr:
            print("No QR codes found to recreate.")
            return 0

        print(
            f"Recreating {sum(len(r.qr_contents) for r in with_qr)} QR code images..."
        )

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

                if naming == "sequential":
                    filename = f"qr_{qr_count:04d}.{self.qr_format}"
                elif naming == "content":
                    # Handle empty content gracefully
                    safe_content = (
                        content[:50].replace("/", "_").replace(":", "_")
                        if content
                        else "empty_qr"
                    )
                    filename = f"{safe_content}.{self.qr_format}"
                else:
                    suffix = (
                        f"_qr{i + 1}.{self.qr_format}"
                        if len(r.qr_contents) > 1
                        else f"_qr.{self.qr_format}"
                    )
                    filename = f"{base_name}{suffix}"

                img.save(str(output_path / filename))
                qr_count += 1

        print(f"Created {qr_count} QR code images in {output_path}")
        return qr_count

    def action_extract(
        self, output_folder: str = None, naming: str = "original", padding: int = 20
    ) -> int:
        output_path = (
            Path(output_folder) if output_folder else self.folder_path / "extracted_qr"
        )
        output_path.mkdir(parents=True, exist_ok=True)

        with_qr = [r for r in self.results if r.has_qr]

        if not with_qr:
            print("No QR codes found to extract.")
            return 0

        print(
            f"Extracting {sum(len(r.qr_contents) for r in with_qr)} QR code regions..."
        )

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

                if naming == "sequential":
                    filename = f"qr_{qr_count:04d}{src_ext}"
                elif naming == "content":
                    safe_content = (
                        content[:50].replace("/", "_").replace(":", "_")
                        if content
                        else "empty_qr"
                    )
                    filename = f"{safe_content}{src_ext}"
                else:
                    suffix = (
                        f"_qr{i + 1}{src_ext}"
                        if len(r.qr_contents) > 1
                        else f"_qr{src_ext}"
                    )
                    filename = f"{base_name}{suffix}"

                cropped.save(str(output_path / filename))
                qr_count += 1

            src_img.close()

        print(f"Extracted {qr_count} QR code regions in {output_path}")
        return qr_count

    def action_list(self) -> None:
        with_qr = [r for r in self.results if r.has_qr]
        without_qr = [r for r in self.results if not r.has_qr]

        print(f"\n{'=' * 50}")
        print(f"QR Multi IMG - Scan Results")
        print(f"{'=' * 50}")
        print(f"Total images: {len(self.results)}")
        print(f"With QR: {len(with_qr)}")
        print(f"Without QR: {len(without_qr)}")

        if with_qr:
            print(f"\n--- Images WITH QR codes ---")
            for r in with_qr:
                qr_preview = (
                    r.qr_contents[0][:50] + "..."
                    if len(r.qr_contents[0]) > 50
                    else r.qr_contents[0]
                )
                print(f"  {r.file_path}")
                print(f"    QR: {qr_preview}")
                if len(r.qr_contents) > 1:
                    print(f"    +{len(r.qr_contents) - 1} more QR codes")

        if without_qr:
            print(f"\n--- Images WITHOUT QR codes ---")
            for r in without_qr:
                print(f"  {r.file_path}")


def _validate_path(path: str, base_dir: str = None) -> tuple[bool, str]:
    """
    Validate and sanitize path for security.

    Args:
        path: Path string to validate
        base_dir: Optional base directory to restrict access within

    Returns:
        tuple: (is_valid, error_message)
            - is_valid: True if path is safe to use
            - error_message: Empty if valid, error description if invalid

    Note:
        Resolves symlinks and blocks path traversal attempts (e.g., ../../../etc)
    """
    try:
        input_path = Path(path)

        # Resolve to absolute path (handles .., symlinks, etc.)
        resolved = input_path.resolve()

        # Check if path exists
        if not resolved.exists():
            return False, f"Path does not exist: {path}"

        # Check if it's a directory
        if not resolved.is_dir():
            return False, f"Path is not a directory: {path}"

        # Optional: check if within allowed base tree (for extra security)
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
    # Validate path first
    is_valid, error = _validate_path(args.path)
    if not is_valid:
        print(f"Error: {error}")
        sys.exit(1)

    formats = None
    if args.formats:
        formats = {f".{f.strip('.')}" for f in args.formats.split(",")}

    scanner = QRMultiIMG(
        folder_path=args.path,
        recursive=args.recursive,
        formats=formats,
        parallel=args.parallel,
        log_file=args.log,
        qr_format=args.qr_format or "png",
        timeout=args.timeout or 30,
    )

    print(f"Scanning folder: {args.path}")
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


if TEXTUAL_AVAILABLE:

    class MainMenu(Screen):
        BINDINGS = [
            Binding("q", "quit", "Quit"),
            Binding("escape", "quit", "Quit"),
            Binding("r", "refresh", "Refresh"),
        ]

        def __init__(self, app_ref):
            super().__init__()
            self.app_ref = app_ref  # Reference to main app for communication

        def compose(self) -> ComposeResult:
            yield Container(
                Static("QR Multi IMG", id="title"),
                Static("QR Code Scanner for Images", id="subtitle"),
                Static("", id="spacing"),
                Button("List all images", id="btn-list", variant="primary"),
                Button("Export results", id="btn-export"),
                Button("Delete without QR", id="btn-delete"),
                Button("Organize into folders", id="btn-organize"),
                Button("Recreate QR codes", id="btn-recreate"),
                Button("Extract QR regions", id="btn-extract"),
                Static("", id="footer"),
                Static("Press q or escape to quit", id="footer-text"),
            )

        def get_results(self):
            """Get scan results from parent app"""
            return getattr(self.app_ref, "last_results", None)

        def on_mount(self) -> None:
            try:
                self.query_one("#title").styles.text_align = "center"
                self.query_one("#subtitle").styles.text_align = "center"
            except Exception:
                pass

        def on_button_pressed(self, event: Button.Pressed) -> None:
            action = event.button.id.replace("btn-", "")
            self.app.push_screen(ActionScreen(action, self.app_ref))

    class ActionScreen(Screen):
        BINDINGS = [
            Binding("escape", "back", "Back"),
        ]

        def __init__(self, action: str, app_ref):
            super().__init__()
            self.action = action
            self.app_ref = app_ref
            self.folder_path = ""
            self.naming = "original"

        def compose(self) -> ComposeResult:
            yield Container(
                Static(f"Action: {self.action.upper()}", id="action-title"),
                Input(placeholder="Enter folder path to scan...", id="folder-input"),
                Input(
                    placeholder="Naming (original/content/sequential)",
                    id="naming-input",
                ),
                Button("Run", id="btn-run", variant="success"),
                Button("Back", id="btn-back"),
                id="action-container",
            )

        def on_mount(self) -> None:
            try:
                self.query_one("#action-title").styles.text_align = "center"
            except Exception:
                pass

        def on_button_pressed(self, event: Button.Pressed) -> None:
            if event.button.id == "btn-back":
                self.app.pop_screen()
            elif event.button.id == "btn-run":
                self.run_action()

        def run_action(self):
            folder_input = self.query_one("#folder-input", Input)
            naming_input = self.query_one("#naming-input", Input)

            self.folder_path = folder_input.value or ""
            self.naming = naming_input.value or "original"

            if not self.folder_path:
                self._show_error("Please enter a folder path")
                return

            if not os.path.isdir(self.folder_path):
                self._show_error(f"Folder not found: {self.folder_path}")
                return

            args = argparse.Namespace(
                path=self.folder_path,
                action=self.action,
                recursive=False,
                formats=None,
                output=None,
                export_format="txt",
                qr_format="png",
                move=False,
                confirm=False,
                parallel=False,
                progress=True,
                log=False,
                naming=self.naming,
                timeout=30,
            )

            try:
                run_cli(args)
                self._show_success("Done! Press Back to return to menu.")
            except Exception as e:
                self._show_error(f"Error: {str(e)}")

        def _show_error(self, message: str):
            """Show error message in TUI"""
            try:
                error_label = Static(f"[red]{message}[/red]", id="error-msg")
                container = self.query_one("#action-container")
                container.mount(error_label)
            except Exception:
                print(f"ERROR: {message}")

        def _show_success(self, message: str):
            """Show success message in TUI"""
            try:
                success_label = Static(f"[green]{message}[/green]", id="success-msg")
                container = self.query_one("#action-container")
                container.mount(success_label)
            except Exception:
                print(f"SUCCESS: {message}")


def main():
    parser = argparse.ArgumentParser(
        description="QR Multi IMG - QR Code Scanner for Images"
    )
    parser.add_argument("--path", "-p", help="Folder path to scan")
    parser.add_argument(
        "--action",
        "-a",
        choices=["list", "export", "delete", "organize", "recreate", "extract"],
        default="list",
        help="Action to perform",
    )
    parser.add_argument(
        "--recursive", "-r", action="store_true", help="Scan subfolders recursively"
    )
    parser.add_argument("--formats", "-f", help="Image formats (comma-separated)")
    parser.add_argument("--output", "-o", help="Output folder path")
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
    parser.add_argument(
        "--nomenu", action="store_true", help="Skip interactive menu, use CLI only"
    )
    parser.add_argument(
        "--naming",
        choices=["original", "content", "sequential"],
        default="original",
        help="File naming for recreate action",
    )
    parser.add_argument(
        "--timeout", "-t", type=int, default=30, help="Timeout per image in seconds"
    )
    parser.add_argument(
        "--padding",
        type=int,
        default=20,
        help="Padding around QR region for extract action",
    )

    args = parser.parse_args()

    if args.nomenu or not sys.stdin.isatty():
        if not args.path:
            parser.print_help()
            sys.exit(1)
        run_cli(args)
        return

    if TEXTUAL_AVAILABLE:

        class QRMultiIMGApp(App):
            BINDINGS = [Binding("q", "quit", "Quit")]
            CSS_PATH = None  # Auto-detect theme from system

            def compose(self) -> ComposeResult:
                yield Header()
                yield MainMenu(self)
                yield Footer()

            def on_mount(self) -> None:
                self.title = "QR Multi IMG"
                self.sub_title = VERSION
                # Auto-detect theme based on system preference
                # Textual automatically uses system theme

            def action_quit(self) -> None:
                self.exit()

        try:
            app = QRMultiIMGApp()
            app.run()
        except Exception as e:
            print(f"TUI Error: {e}")
            print("Falling back to CLI mode...")
            if not args.path:
                parser.print_help()
                sys.exit(1)
            run_cli(args)
    else:
        if not args.path:
            parser.print_help()
            sys.exit(1)
        run_cli(args)


if __name__ == "__main__":
    main()
