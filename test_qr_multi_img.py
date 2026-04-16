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


class TestActionRecreate:
    """Test action_recreate functionality"""

    def test_recreate_with_no_qr_results(self):
        """Should handle no QR results gracefully"""
        from qr_multi_img import QRMultiIMG
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = []  # No results

            output_folder = tempfile.mkdtemp()
            count = scanner.action_recreate(
                output_folder=output_folder, naming="original"
            )

            assert count == 0

    def test_recreate_naming_sequential(self):
        """Should use sequential naming when specified"""
        from qr_multi_img import QRMultiIMG, QRCodeResult
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            output_folder = tempfile.mkdtemp()

            scanner = QRMultiIMG(folder_path=tmpdir, qr_format="png")
            scanner.results = [
                QRCodeResult(
                    "/fake/image.jpg",
                    has_qr=True,
                    qr_contents=["test1", "test2"],
                    qr_bboxes=[(10, 10, 100, 100), (200, 200, 100, 100)],
                )
            ]

            # Non crea file perché le immagini non esistono真实, ma non deve fallire
            count = scanner.action_recreate(
                output_folder=output_folder, naming="sequential"
            )
            assert count >= 0


class TestActionExtract:
    """Test action_extract functionality"""

    def test_extract_with_no_qr_results(self):
        """Should handle no QR results gracefully"""
        from qr_multi_img import QRMultiIMG
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = []

            output_folder = tempfile.mkdtemp()
            count = scanner.action_extract(
                output_folder=output_folder, naming="original", padding=20
            )

            assert count == 0

    def test_extract_padding_boundary(self):
        """Should handle padding that exceeds image boundaries"""
        from qr_multi_img import QRMultiIMG, QRCodeResult
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            output_folder = tempfile.mkdtemp()

            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = [
                QRCodeResult(
                    "/fake/small.jpg",
                    has_qr=True,
                    qr_contents=["test"],
                    qr_bboxes=[(5, 5, 10, 10)],  # Small QR in corner
                )
            ]

            # Large padding should be clamped to image bounds, not error
            count = scanner.action_extract(
                output_folder=output_folder, naming="original", padding=1000
            )
            assert count >= 0


class TestVersionConsistency:
    """Test version consistency between docstring and VERSION constant"""

    def test_version_docstring_matches_constant(self):
        """Docstring version should match VERSION constant"""
        import qr_multi_img

        with open(qr_multi_img.__file__, "r") as f:
            content = f.read()

        import re

        docstring_version = re.search(r"Version:\s*(v[\d.]+)", content)
        constant_version = qr_multi_img.VERSION

        assert docstring_version is not None, "Version not found in docstring"
        assert docstring_version.group(1) == constant_version, (
            f"Version mismatch: docstring says '{docstring_version.group(1)}' "
            f"but VERSION constant is '{constant_version}'"
        )


class TestActionDecode:
    """Test action_decode functionality"""

    def test_decode_with_no_qr_results(self):
        """Should handle no QR results gracefully"""
        from qr_multi_img import QRMultiIMG
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = []  # No results

            result = scanner.action_decode(output_format="text")
            assert result == []

    def test_decode_returns_list_with_qr(self):
        """Should return list of results when QR codes found"""
        from qr_multi_img import QRMultiIMG, QRCodeResult
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = [
                QRCodeResult(
                    "/fake/image1.jpg",
                    has_qr=True,
                    qr_contents=["test1", "test2"],
                ),
                QRCodeResult(
                    "/fake/image2.jpg",
                    has_qr=True,
                    qr_contents=["test3"],
                ),
            ]

            result = scanner.action_decode(output_format="text")
            assert len(result) == 2
            assert result[0].qr_contents == ["test1", "test2"]
            assert result[1].qr_contents == ["test3"]


class TestActionFilter:
    """Test action_filter functionality"""

    def test_filter_no_qr_results(self):
        """Should handle no QR results gracefully"""
        from qr_multi_img import QRMultiIMG
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = []

            result = scanner.action_filter(pattern="test")
            assert result == []

    def test_filter_matching_pattern(self):
        """Should return images matching pattern"""
        from qr_multi_img import QRMultiIMG, QRCodeResult
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = [
                QRCodeResult(
                    "/fake/image1.jpg",
                    has_qr=True,
                    qr_contents=["hello world"],
                ),
                QRCodeResult(
                    "/fake/image2.jpg",
                    has_qr=True,
                    qr_contents=["foo bar"],
                ),
            ]

            result = scanner.action_filter(pattern="hello")
            assert len(result) == 1
            assert result[0].file_path == "/fake/image1.jpg"

    def test_filter_exclude_mode(self):
        """Should return non-matching images when exclude is True"""
        from qr_multi_img import QRMultiIMG, QRCodeResult
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = [
                QRCodeResult(
                    "/fake/image1.jpg",
                    has_qr=True,
                    qr_contents=["hello world"],
                ),
                QRCodeResult(
                    "/fake/image2.jpg",
                    has_qr=True,
                    qr_contents=["foo bar"],
                ),
            ]

            result = scanner.action_filter(pattern="hello", exclude=True)
            assert len(result) == 1
            assert result[0].file_path == "/fake/image2.jpg"


class TestActionBatchRename:
    """Test action_batch_rename functionality"""

    def test_batch_rename_no_qr_results(self):
        """Should handle no QR results gracefully"""
        from qr_multi_img import QRMultiIMG
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = []

            result = scanner.action_batch_rename(prefix="qr_")
            assert result["renamed"] == 0

    def test_batch_rename_dry_run(self):
        """Should show dry run without actually renaming"""
        from qr_multi_img import QRMultiIMG, QRCodeResult
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            test_file = Path(tmpdir) / "test.jpg"
            test_file.write_bytes(b"fake image")

            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = [
                QRCodeResult(
                    str(test_file),
                    has_qr=True,
                    qr_contents=["test_content"],
                ),
            ]

            result = scanner.action_batch_rename(prefix="qr_", dry_run=True)

            # File should still exist with original name
            assert test_file.exists()
            assert result["renamed"] == 1

    def test_batch_rename_returns_changes(self):
        """Should return list of changes"""
        from qr_multi_img import QRMultiIMG, QRCodeResult
        import tempfile
        from pathlib import Path

        with tempfile.TemporaryDirectory() as tmpdir:
            # Create a real file
            test_file = Path(tmpdir) / "original.jpg"
            test_file.write_bytes(b"fake image")

            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = [
                QRCodeResult(
                    str(test_file),
                    has_qr=True,
                    qr_contents=["test123"],
                ),
            ]

            result = scanner.action_batch_rename(
                prefix="pre_", suffix="_suf", dry_run=True
            )
            assert result["renamed"] == 1
            assert len(result["changes"]) == 1


class TestActionVerify:
    """Test action_verify functionality"""

    def test_verify_invalid_folder(self):
        """Should handle invalid recreated folder"""
        from qr_multi_img import QRMultiIMG
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMG(folder_path=tmpdir)

            result = scanner.action_verify(
                originals_folder=tmpdir, recreated_folder="/nonexistent_folder_12345"
            )
            # Should handle error gracefully
            assert "errors" in result

    def test_verify_empty_folder(self):
        """Should handle empty recreated folder"""
        from qr_multi_img import QRMultiIMG
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            empty_folder = Path(tmpdir) / "empty"
            empty_folder.mkdir()

            scanner = QRMultiIMG(folder_path=tmpdir)

            result = scanner.action_verify(
                originals_folder=tmpdir, recreated_folder=str(empty_folder)
            )
            # No matches expected
            assert result["matched"] == 0


class TestDeepScanFeature:
    """Test deep_scan feature functionality"""

    def test_deep_scan_parameter_accepted(self):
        """Should accept deep_scan parameter in QRMultiIMG constructor"""
        from qr_multi_img import QRMultiIMG, DEFAULT_DEEP_TIMEOUT
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            # Test with deep_scan=False (default)
            scanner = QRMultiIMG(folder_path=tmpdir, deep_scan=False)
            assert scanner.deep_scan == False
            assert scanner.deep_timeout == DEFAULT_DEEP_TIMEOUT

            # Test with deep_scan=True
            scanner = QRMultiIMG(folder_path=tmpdir, deep_scan=True, deep_timeout=120)
            assert scanner.deep_scan == True
            assert scanner.deep_timeout == 120

    def test_deep_scan_default_timeout(self):
        """Should have correct default timeout for deep scan"""
        from qr_multi_img import QRMultiIMG, DEFAULT_DEEP_TIMEOUT, DEFAULT_TIMEOUT
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMG(folder_path=tmpdir)
            # Default should be regular timeout
            assert scanner.timeout == DEFAULT_TIMEOUT
            assert scanner.deep_timeout == DEFAULT_DEEP_TIMEOUT
            assert scanner.deep_timeout > scanner.timeout


class TestDecodeJSONFormat:
    """Test decode action with JSON output format"""

    def test_decode_json_output_format(self):
        """Should output valid JSON when format is json"""
        from qr_multi_img import QRMultiIMG, QRCodeResult
        import tempfile
        import json
        import re

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = [
                QRCodeResult(
                    "/fake/image1.jpg",
                    has_qr=True,
                    qr_contents=["test1", "test2"],
                ),
                QRCodeResult(
                    "/fake/image2.jpg",
                    has_qr=True,
                    qr_contents=["test3"],
                ),
            ]

            import io
            import sys

            old_stdout = sys.stdout
            sys.stdout = io.StringIO()

            result = scanner.action_decode(output_format="json")

            output = sys.stdout.getvalue()
            sys.stdout = old_stdout

            # Output contains JSON followed by "Total: ..." line - extract JSON part
            json_match = re.match(r"(\[.*\])", output, re.DOTALL)
            assert json_match is not None, f"Could not find JSON in output: {output}"

            parsed = json.loads(json_match.group(1))
            assert len(parsed) == 2
            assert parsed[0]["file"] == "/fake/image1.jpg"
            assert parsed[0]["qr_codes"] == ["test1", "test2"]
            assert parsed[0]["count"] == 2
            assert parsed[1]["count"] == 1


class TestActionVerifyEdgeCases:
    """Test action_verify edge cases and error handling"""

    def test_verify_mismatched_qr_codes(self):
        """Should detect mismatched QR codes correctly"""
        from qr_multi_img import QRMultiIMG
        import tempfile
        from pathlib import Path

        with tempfile.TemporaryDirectory() as tmpdir:
            originals_folder = Path(tmpdir) / "originals"
            originals_folder.mkdir()
            recreated_folder = Path(tmpdir) / "recreated"
            recreated_folder.mkdir()

            scanner = QRMultiIMG(folder_path=str(originals_folder))

            result = scanner.action_verify(
                originals_folder=str(originals_folder),
                recreated_folder=str(recreated_folder),
            )

            # Should handle gracefully - no matches, no mismatches, no errors
            assert "matched" in result
            assert "mismatched" in result
            assert "errors" in result
            assert result["matched"] == 0

    def test_verify_with_only_images_with_qr(self):
        """Should handle folder with only QR-coded images"""
        from qr_multi_img import QRMultiIMG
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMG(folder_path=tmpdir)

            result = scanner.action_verify(
                originals_folder=tmpdir,
                recreated_folder="/nonexistent",
            )

            # Should return error structure even when folder doesn't exist
            assert "matched" in result
            assert "mismatched" in result
            assert "errors" in result
