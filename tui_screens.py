"""
TUI Screens for QR Multi IMGS
Textual-based user interface components

Note: These screens are currently disabled by default due to terminal
compatibility issues. Use --tui flag to enable in the future.
"""

import os
import argparse
import threading
from textual.app import App, ComposeResult
from textual.widgets import Header, Footer, Static, Button, Input, Container
from textual.screen import Screen
from textual.binding import Binding


DEFAULT_TIMEOUT = 30
DEFAULT_DEEP_TIMEOUT = 60
DEFAULT_PADDING = 20


def run_cli(args):
    """Import and call the main CLI function."""
    from qr_multi_imgs import run_cli as _run_cli

    _run_cli(args)


class FolderScreen(Screen):
    BINDINGS = [
        Binding("q", "quit", "Quit"),
        Binding("escape", "quit", "Quit"),
    ]

    def __init__(self, app_ref):
        super().__init__()
        self.app_ref = app_ref

    def compose(self) -> ComposeResult:
        yield Container(
            Static("QR Multi IMGS", id="title"),
            Static("QR Code Scanner for Images", id="subtitle"),
            Static("", id="spacing"),
            Static("Step 1: Select folder to scan", id="step-label"),
            Input(placeholder="Enter folder path...", id="folder-input"),
            Button("Browse...", id="btn-browse", variant="default"),
            Button("Continue", id="btn-continue", variant="primary"),
            Static("", id="footer"),
            Static("Press q or escape to quit", id="footer-text"),
        )

    def on_mount(self) -> None:
        try:
            self.query_one("#title").styles.text_align = "center"
            self.query_one("#subtitle").styles.text_align = "center"
            self.query_one("#step-label").styles.text_align = "center"
            self.query_one("#folder-input").focus()
        except Exception:
            pass

    def on_button_pressed(self, event: Button.Pressed) -> None:
        if event.button.id == "btn-browse":
            self._browse_folder()
        elif event.button.id == "btn-continue":
            self._continue_to_subfolder()

    def _browse_folder(self):
        try:
            import tkinter as tk
            from tkinter import filedialog

            root = tk.Tk()
            root.withdraw()
            folder = filedialog.askdirectory(title="Select folder to scan")
            root.destroy()
            if folder:
                input_field = self.query_one("#folder-input", Input)
                input_field.value = folder
        except Exception as e:
            print(f"Browser error: {e}")
            self._show_error("Could not open file browser")

    def _continue_to_subfolder(self):
        folder_input = self.query_one("#folder-input", Input)
        folder_path = folder_input.value or ""

        if not folder_path:
            self._show_error("Please enter a folder path")
            return

        if not os.path.isdir(folder_path):
            self._show_error(f"Folder not found: {folder_path}")
            return

        self.app_ref.folder_path = folder_path
        self.app.push_screen(SubfolderScreen(self.app_ref))

    def _show_error(self, message: str):
        try:
            container = self.query_one("Container")
            existing = container.query_one("#error-msg")
            if existing:
                existing.remove()
            error_label = Static(f"[red]{message}[/red]", id="error-msg")
            container.mount(error_label)
        except Exception:
            print(f"ERROR: {message}")


class SubfolderScreen(Screen):
    BINDINGS = [
        Binding("q", "quit", "Quit"),
        Binding("escape", "back", "Back"),
    ]

    def __init__(self, app_ref):
        super().__init__()
        self.app_ref = app_ref

    def compose(self) -> ComposeResult:
        yield Container(
            Static("Step 2: Scan options", id="step-label"),
            Static(f"Folder: {self.app_ref.folder_path}", id="folder-display"),
            Static("", id="spacing"),
            Static("Scan subfolders too?", id="question"),
            Button("Yes - Include subfolders", id="btn-yes", variant="primary"),
            Button("No - This folder only", id="btn-no", variant="default"),
            Static("", id="footer"),
        )

    def on_mount(self) -> None:
        try:
            self.query_one("#step-label").styles.text_align = "center"
            self.query_one("#folder-display").styles.text_align = "center"
            self.query_one("#question").styles.text_align = "center"
        except Exception:
            pass

    def on_button_pressed(self, event: Button.Pressed) -> None:
        recursive = event.button.id == "btn-yes"
        self.app_ref.recursive = recursive
        self.app.push_screen(ActionScreen(self.app_ref))


class ActionScreen(Screen):
    BINDINGS = [
        Binding("q", "quit", "Quit"),
        Binding("escape", "back", "Back"),
    ]

    def __init__(self, app_ref):
        super().__init__()
        self.app_ref = app_ref

    def compose(self) -> ComposeResult:
        yield Container(
            Static("Step 3: Select action", id="step-label"),
            Static(f"Folder: {self.app_ref.folder_path}", id="folder-display"),
            Static(
                f"Subfolders: {'Yes' if self.app_ref.recursive else 'No'}",
                id="recursive-display",
            ),
            Static("", id="spacing"),
            Button(
                "Show - Display which images have QR codes",
                id="btn-list",
                variant="primary",
            ),
            Button("Save - Export results to a file", id="btn-export"),
            Button("Delete - Remove images without QR codes", id="btn-delete"),
            Button("Sort - Organize into folders", id="btn-organize"),
            Button("Create - Generate new QR code images", id="btn-recreate"),
            Button("Crop - Extract QR code regions", id="btn-extract"),
            Button("Read - Just decode, don't save", id="btn-decode"),
            Button("Filter - Find images by QR content", id="btn-filter"),
            Button("Rename - Batch rename files by QR", id="btn-batch-rename"),
            Button("Check - Verify recreated QR codes", id="btn-verify"),
            Static("", id="footer"),
        )

    def on_mount(self) -> None:
        try:
            self.query_one("#step-label").styles.text_align = "center"
            self.query_one("#folder-display").styles.text_align = "center"
            self.query_one("#recursive-display").styles.text_align = "center"
        except Exception:
            pass

    def on_button_pressed(self, event: Button.Pressed) -> None:
        action = event.button.id.replace("btn-", "")
        self.app_ref.selected_action = action

        if action in ["recreate", "extract"]:
            self.app.push_screen(OutputScreen(self.app_ref))
        else:
            self.app_ref.output_folder = None
            self.app_ref.naming = "original"
            self.app.push_screen(RunScreen(self.app_ref))


class OutputScreen(Screen):
    BINDINGS = [
        Binding("q", "quit", "Quit"),
        Binding("escape", "back", "Back"),
    ]

    def __init__(self, app_ref):
        super().__init__()
        self.app_ref = app_ref

    def compose(self) -> ComposeResult:
        yield Container(
            Static("Step 4: Output settings", id="step-label"),
            Static(
                f"Action: {self.app_ref.selected_action.upper()}",
                id="action-display",
            ),
            Static("", id="spacing"),
            Static("Where to save output?", id="question1"),
            Input(
                placeholder="Output folder path (leave empty for default)",
                id="output-input",
            ),
            Button("Browse...", id="btn-browse", variant="default"),
            Static("", id="spacing2"),
            Static("What filename style?", id="question2"),
            Button(
                "Original - keep original filename",
                id="btn-original",
                variant="primary",
            ),
            Button("Content - use QR content as filename", id="btn-content"),
            Button("Sequential - numbered (001, 002...)", id="btn-sequential"),
            Button("Continue", id="btn-continue", variant="success"),
            Static("", id="footer"),
        )

    def on_mount(self) -> None:
        try:
            self.query_one("#step-label").styles.text_align = "center"
            self.query_one("#action-display").styles.text_align = "center"
            self.query_one("#question1").styles.text_align = "center"
            self.query_one("#question2").styles.text_align = "center"
        except Exception:
            pass

    def on_button_pressed(self, event: Button.Pressed) -> None:
        if event.button.id == "btn-browse":
            self._browse_output()
        elif event.button.id == "btn-continue":
            self._continue_to_run()
        elif event.button.id == "btn-original":
            self.app_ref.naming = "original"
            self._update_selection("btn-original")
        elif event.button.id == "btn-content":
            self.app_ref.naming = "content"
            self._update_selection("btn-content")
        elif event.button.id == "btn-sequential":
            self.app_ref.naming = "sequential"
            self._update_selection("btn-sequential")

    def _update_selection(self, selected_id: str):
        for btn_id in ["btn-original", "btn-content", "btn-sequential"]:
            try:
                btn = self.query_one(f"#{btn_id}", Button)
                btn.variant = "primary" if btn_id == selected_id else "default"
            except Exception:
                pass

    def _browse_output(self):
        try:
            import tkinter as tk
            from tkinter import filedialog

            root = tk.Tk()
            root.withdraw()
            folder = filedialog.askdirectory(title="Select output folder")
            root.destroy()
            if folder:
                input_field = self.query_one("#output-input", Input)
                input_field.value = folder
        except Exception as e:
            print(f"Browser error: {e}")

    def _continue_to_run(self):
        output_input = self.query_one("#output-input", Input)
        self.app_ref.output_folder = output_input.value or None
        self.app.push_screen(RunScreen(self.app_ref))


class RunScreen(Screen):
    BINDINGS = [
        Binding("q", "quit", "Quit"),
        Binding("escape", "back", "Back"),
    ]

    def __init__(self, app_ref):
        super().__init__()
        self.app_ref = app_ref

    def compose(self) -> ComposeResult:
        yield Container(
            Static("Running...", id="status"),
            Static(f"Scanning: {self.app_ref.folder_path}", id="folder-display"),
            Static(
                f"Action: {self.app_ref.selected_action.upper()}",
                id="action-display",
            ),
            Static(
                f"Subfolders: {'Yes' if self.app_ref.recursive else 'No'}",
                id="recursive-display",
            ),
            Static("Progress: Scanning...", id="progress-label"),
            id="run-container",
        )

    def on_mount(self) -> None:
        try:
            self.query_one("#status").styles.text_align = "center"
            self.query_one("#folder-display").styles.text_align = "center"
            self.query_one("#action-display").styles.text_align = "center"
            self.query_one("#recursive-display").styles.text_align = "center"
        except Exception:
            pass

        import threading

        threading.Thread(target=self._run_action, daemon=True).start()

    def _run_action(self):
        args = argparse.Namespace(
            path=self.app_ref.folder_path,
            action=self.app_ref.selected_action,
            recursive=self.app_ref.recursive,
            formats=None,
            output=self.app_ref.output_folder,
            export_format="txt",
            qr_format="png",
            move=False,
            confirm=False,
            parallel=False,
            progress=True,
            log=False,
            naming=self.app_ref.naming or "original",
            timeout=DEFAULT_TIMEOUT,
            padding=DEFAULT_PADDING,
            deep_scan=False,
            deep_timeout=DEFAULT_DEEP_TIMEOUT,
            filter_pattern=None,
            filter_case_sensitive=False,
            filter_exclude=False,
            rename_prefix=None,
            rename_suffix=None,
            nomenu=True,
        )

        try:
            run_cli(args)
            self._update_status("[green]Done! Press Escape to go back to menu[/green]")
        except Exception as e:
            self._update_status(f"[red]Error: {str(e)}[/red]")

    def _update_status(self, message: str):
        try:
            status = self.query_one("#status", Static)
            status.update(message)
        except Exception:
            pass
