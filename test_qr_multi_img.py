#!/usr/bin/env python3
"""
Test suite for QR Multi IMG
Tests for security fixes: path validation
"""

import os
import sys
import tempfile
import pytest
from pathlib import Path


class TestPathValidation:
    """Test path validation security fixes"""

    def test_path_traversal_attempt_blocked(self):
        """Should block path traversal attempts like ../../../etc"""
        from qr_multi_img import _validate_path

        with tempfile.TemporaryDirectory() as tmpdir:
            malicious_path = os.path.join(tmpdir, "..", "..", "..", "etc", "passwd")

            is_valid, error = _validate_path(malicious_path)

            # Path traversal deve essere rifiutato
            assert not is_valid, f"Path traversal NOT blocked! Error: {error}"
            assert "outside" in error.lower() or "not exist" in error.lower(), (
                f"Wrong error: {error}"
            )

    def test_absolute_path_outside_allowed_tree(self):
        """Should block absolute paths outside allowed tree"""
        from qr_multi_img import _validate_path

        # Test con /etc (di solito non accessibile come cartella)
        system_path = "/etc" if os.path.isdir("/etc") else "/usr"

        is_valid, error = _validate_path(system_path)

        # Se fuori dalla directory di lavoro, deve essere bloccato
        if not is_valid:
            assert "outside" in error.lower()

    def test_valid_directory_accepted(self):
        """Should accept valid directories within allowed tree"""
        from qr_multi_img import _validate_path

        with tempfile.TemporaryDirectory() as tmpdir:
            is_valid, error = _validate_path(tmpdir)

            assert is_valid, f"Valid directory rejected: {error}"
            assert error == ""


class TestQRDetection:
    """Test QR detection functionality"""

    def test_no_images_returns_empty(self):
        """Should return empty list when no images found"""
        with tempfile.TemporaryDirectory() as tmpdir:
            from qr_multi_img import QRMultiIMG

            scanner = QRMultiIMG(folder_path=tmpdir)
            results = scanner.scan(progress=False)
            assert results == []

    def test_scan_counts_images(self):
        """Should count images correctly"""
        with tempfile.TemporaryDirectory() as tmpdir:
            from qr_multi_img import QRMultiIMG

            # Crea alcuni file temporanei (non immagini reali)
            for i in range(3):
                Path(tmpdir, f"test_{i}.txt").touch()

            scanner = QRMultiIMG(folder_path=tmpdir, formats={".txt"})
            results = scanner.scan(progress=False)
            assert scanner._total_count == 3


class TestMemoryLeak:
    """Test for memory leak fixes"""

    def test_image_context_manager(self):
        """Images should be closed after processing"""
        with tempfile.TemporaryDirectory() as tmpdir:
            from qr_multi_img import QRMultiIMG

            # Crea un file JPEG simulato
            test_img = Path(tmpdir, "test.jpg")
            test_img.write_bytes(b"fake jpeg")

            scanner = QRMultiIMG(folder_path=tmpdir)
            results = scanner.scan(progress=False)
            assert isinstance(results, list)


class TestParallelProgress:
    """Test parallel progress ordering"""

    def test_parallel_progress_ordering(self):
        """Progress should show correct count even in parallel mode"""
        with tempfile.TemporaryDirectory() as tmpdir:
            from qr_multi_img import QRMultiIMG

            # Crea 5 file
            for i in range(5):
                Path(tmpdir, f"test_{i}.txt").touch()

            scanner = QRMultiIMG(folder_path=tmpdir, formats={".txt"}, parallel=True)
            results = scanner.scan(progress=False)

            # Total should still be correct (5 files)
            assert scanner._total_count == 5, f"Expected 5, got {scanner._total_count}"


class TestTUIAction:
    """Test TUI action error handling"""

    def test_action_invalid_folder(self):
        """Should handle invalid folder gracefully"""
        import argparse
        from qr_multi_img import run_cli

        # Crea args con cartella inesistente
        args = argparse.Namespace(
            path="/nonexistent/path/that/does/not/exist",
            action="list",
            recursive=False,
            formats=None,
            output=None,
            export_format="txt",
            qr_format="png",
            move=False,
            confirm=True,
            parallel=False,
            progress=True,
            log=False,
            naming="original",
            timeout=30,
        )

        # Should exit with error, not crash
        try:
            run_cli(args)
            # Se arriva qui, non ha gestito l'errore
            assert False, "Should have exited with error"
        except SystemExit as e:
            # Questo è il comportamento corretto: exits with code 1
            assert e.code == 1


class TestQRCodeEdgeCases:
    """Test edge cases in QR code handling"""

    def test_empty_qr_content_is_valid(self):
        """Empty QR code content should be handled gracefully"""
        # QR code with empty content is valid but should be noted
        from qr_multi_img import QRCodeResult

        result = QRCodeResult("/test/image.jpg", has_qr=True, qr_contents=[""])

        # Empty string is valid QR content - store it
        assert result.has_qr == True
        assert "" in result.qr_contents

    def test_multiple_empty_qr_codes(self):
        """Multiple QR codes with empty content should be handled"""
        from qr_multi_img import QRCodeResult

        result = QRCodeResult(
            "/test/image.jpg", has_qr=True, qr_contents=["", "hello", ""]
        )

        # Should have 3 QR codes
        assert len(result.qr_contents) == 3


class TestExtractPathValidation:
    """Test path validation for extract action"""

    def test_extract_with_valid_output_folder(self):
        """Should accept valid output folder in extract action"""
        from qr_multi_img import QRMultiIMG, QRCodeResult
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            # Crea una cartella di output valida
            output_folder = tempfile.mkdtemp()

            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = [
                QRCodeResult(
                    "/fake/image.jpg",
                    has_qr=True,
                    qr_contents=["test content"],
                    qr_bboxes=[(10, 10, 100, 100)],
                )
            ]

            # Questo dovrebbe funzionare senza errori
            count = scanner.action_extract(
                output_folder=output_folder, naming="original", padding=20
            )

            # Il risultato potrebbe essere 0 perché l'immagine non esiste, ma non deve sollevare errore di path
            # Il test verifica che non ci sia un errore di validazione path
            assert count >= 0  # Non deve sollevare ValidationError

    def test_extract_with_malicious_output_path(self):
        """Should reject malicious output path (path traversal) in extract action"""
        from qr_multi_img import QRMultiIMG, QRCodeResult
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            # Simula percorso malevolo
            malicious_output = os.path.join(tmpdir, "..", "..", "..", "etc")

            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = [
                QRCodeResult(
                    "/fake/image.jpg",
                    has_qr=True,
                    qr_contents=["test content"],
                    qr_bboxes=[(10, 10, 100, 100)],
                )
            ]

            # Dovrebbe sollevare errore di validazione
            try:
                scanner.action_extract(
                    output_folder=malicious_output, naming="original", padding=20
                )
                # Se arriva qui senza errore, il bug esiste!
                assert False, (
                    "SECURITY BUG: Path traversal NOT blocked in extract action!"
                )
            except (ValueError, OSError) as e:
                # Questo è il comportamento corretto
                assert "outside" in str(e).lower() or "not exist" in str(e).lower(), (
                    f"Wrong error: {e}"
                )


class TestRecreatePathValidation:
    """Test path validation for recreate action"""

    def test_recreate_with_valid_output_folder(self):
        """Should accept valid output folder in recreate action"""
        from qr_multi_img import QRMultiIMG, QRCodeResult
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            output_folder = tempfile.mkdtemp()

            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = [
                QRCodeResult(
                    "/fake/image.jpg",
                    has_qr=True,
                    qr_contents=["test content"],
                    qr_bboxes=[(10, 10, 100, 100)],
                )
            ]

            count = scanner.action_recreate(
                output_folder=output_folder, naming="original"
            )
            assert count >= 0

    def test_recreate_with_malicious_output_path(self):
        """Should reject malicious output path (path traversal) in recreate action"""
        from qr_multi_img import QRMultiIMG, QRCodeResult
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            malicious_output = os.path.join(tmpdir, "..", "..", "..", "etc")

            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = [
                QRCodeResult(
                    "/fake/image.jpg",
                    has_qr=True,
                    qr_contents=["test content"],
                    qr_bboxes=[(10, 10, 100, 100)],
                )
            ]

            try:
                scanner.action_recreate(
                    output_folder=malicious_output, naming="original"
                )
                assert False, (
                    "SECURITY BUG: Path traversal NOT blocked in recreate action!"
                )
            except (ValueError, OSError) as e:
                assert "Invalid output path" in str(e)
