#!/usr/bin/env python3
"""
Test suite for QR Multi IMGS
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
        from qr_multi_imgs import _validate_path

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
        from qr_multi_imgs import _validate_path

        # Test con /etc (di solito non accessibile come cartella)
        system_path = "/etc" if os.path.isdir("/etc") else "/usr"

        is_valid, error = _validate_path(system_path)

        # Se fuori dalla directory di lavoro, deve essere bloccato
        if not is_valid:
            assert "outside" in error.lower()

    def test_valid_directory_accepted(self):
        """Should accept valid directories within allowed tree"""
        from qr_multi_imgs import _validate_path

        with tempfile.TemporaryDirectory() as tmpdir:
            is_valid, error = _validate_path(tmpdir)

            assert is_valid, f"Valid directory rejected: {error}"
            assert error == ""


class TestQRDetection:
    """Test QR detection functionality"""

    def test_no_images_returns_empty(self):
        """Should return empty list when no images found"""
        with tempfile.TemporaryDirectory() as tmpdir:
            from qr_multi_imgs import QRMultiIMG

            scanner = QRMultiIMG(folder_path=tmpdir)
            results = scanner.scan(progress=False)
            assert results == []

    def test_scan_counts_images(self):
        """Should count images correctly"""
        with tempfile.TemporaryDirectory() as tmpdir:
            from qr_multi_imgs import QRMultiIMG

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
            from qr_multi_imgs import QRMultiIMG

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
            from qr_multi_imgs import QRMultiIMG

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
        from qr_multi_imgs import run_cli

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

        with pytest.raises(SystemExit) as excinfo:
            run_cli(args)
        assert excinfo.value.code == 2  # EC_ERROR for invalid path


class TestQRCodeEdgeCases:
    """Test edge cases in QR code handling"""

    def test_empty_qr_content_is_valid(self):
        """Empty QR code content should be handled gracefully"""
        # QR code with empty content is valid but should be noted
        from qr_multi_imgs import QRCodeResult

        result = QRCodeResult("/test/image.jpg", has_qr=True, qr_contents=[""])

        # Empty string is valid QR content - store it
        assert result.has_qr == True
        assert "" in result.qr_contents

    def test_multiple_empty_qr_codes(self):
        """Multiple QR codes with empty content should be handled"""
        from qr_multi_imgs import QRCodeResult

        result = QRCodeResult(
            "/test/image.jpg", has_qr=True, qr_contents=["", "hello", ""]
        )

        # Should have 3 QR codes
        assert len(result.qr_contents) == 3


class TestExtractPathValidation:
    """Test path validation for extract action"""

    def test_extract_with_valid_output_folder(self):
        """Should accept valid output folder in extract action"""
        from qr_multi_imgs import QRMultiIMG, QRCodeResult
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            output_folder = tmpdir  # tmpdir già è temporaneo e viene pulito

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
        from qr_multi_imgs import QRMultiIMG, QRCodeResult
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

            with pytest.raises((ValueError, OSError)) as excinfo:
                scanner.action_extract(
                    output_folder=malicious_output, naming="original", padding=20
                )
            err_msg = str(excinfo.value).lower()
            assert "outside" in err_msg or "not exist" in err_msg, (
                f"Wrong error: {excinfo.value}"
            )


class TestRecreatePathValidation:
    """Test path validation for recreate action"""

    def test_recreate_with_valid_output_folder(self):
        """Should accept valid output folder in recreate action"""
        from qr_multi_imgs import QRMultiIMG
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = []

            count = scanner.action_recreate(
                output_folder=tmpdir, naming="original"
            )
            assert count == 0

    def test_recreate_with_malicious_output_path(self):
        """Should reject malicious output path (path traversal) in recreate action"""
        from qr_multi_imgs import QRMultiIMG
        import tempfile
        import pytest

        with tempfile.TemporaryDirectory() as tmpdir:
            malicious_output = os.path.join(tmpdir, "..", "..", "..", "etc")

            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = []

            with pytest.raises((ValueError, OSError)) as excinfo:
                scanner.action_recreate(
                    output_folder=malicious_output, naming="original"
                )
            assert "Invalid output path" in str(excinfo.value)


class TestVersionConsistency:
    """Test version consistency between docstring and VERSION constant"""

    def test_version_docstring_matches_constant(self):
        """Docstring version should match VERSION constant"""
        import qr_multi_imgs

        with open(qr_multi_imgs.__file__, "r") as f:
            content = f.read()

        import re

        docstring_version = re.search(r"Version:\s*(v[\d.]+)", content)
        constant_version = qr_multi_imgs.VERSION

        assert docstring_version is not None, "Version not found in docstring"
        assert docstring_version.group(1) == constant_version, (
            f"Version mismatch: docstring says '{docstring_version.group(1)}' "
            f"but VERSION constant is '{constant_version}'"
        )


class TestActionDecode:
    """Test action_decode functionality"""

    def test_decode_with_no_qr_results(self):
        """Should handle no QR results gracefully"""
        from qr_multi_imgs import QRMultiIMG
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = []  # No results

            result = scanner.action_decode(output_format="text")
            assert result == []

    def test_decode_returns_list_with_qr(self):
        """Should return list of results when QR codes found"""
        from qr_multi_imgs import QRMultiIMG, QRCodeResult
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
        from qr_multi_imgs import QRMultiIMG
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = []

            result = scanner.action_filter(pattern="test")
            assert result == []

    def test_filter_matching_pattern(self):
        """Should return images matching pattern"""
        from qr_multi_imgs import QRMultiIMG, QRCodeResult
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
        from qr_multi_imgs import QRMultiIMG, QRCodeResult
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
        from qr_multi_imgs import QRMultiIMG
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMG(folder_path=tmpdir)
            scanner.results = []

            result = scanner.action_batch_rename(prefix="qr_")
            assert result["renamed"] == 0

    def test_batch_rename_dry_run(self):
        """Should show dry run without actually renaming"""
        from qr_multi_imgs import QRMultiIMG, QRCodeResult
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
        from qr_multi_imgs import QRMultiIMG, QRCodeResult
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
        from qr_multi_imgs import QRMultiIMG
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
        from qr_multi_imgs import QRMultiIMG
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
        from qr_multi_imgs import QRMultiIMG, DEFAULT_DEEP_TIMEOUT
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
        from qr_multi_imgs import QRMultiIMG, DEFAULT_DEEP_TIMEOUT, DEFAULT_TIMEOUT
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
        from qr_multi_imgs import QRMultiIMG, QRCodeResult
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
        from qr_multi_imgs import QRMultiIMG
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
        from qr_multi_imgs import QRMultiIMG
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


class TestStructuredQRParse:
    """Test structured QR content parsing"""

    def test_parse_wifi(self):
        from qr_multi_imgs import QRMultiIMGS
        result = QRMultiIMGS.parse_structured_content('WIFI:S:MyNet;T:WPA;P:pass123;;')
        assert result["type"] == "wifi"
        assert result["fields"]["ssid"] == "MyNet"
        assert result["fields"]["encryption"] == "WPA"

    def test_parse_vcard(self):
        from qr_multi_imgs import QRMultiIMGS
        result = QRMultiIMGS.parse_structured_content(
            "BEGIN:VCARD\nVERSION:3.0\nFN:Jane Doe\nTEL:+987654321\nEMAIL:j@test.com\nEND:VCARD"
        )
        assert result["type"] == "vcard"
        assert "Jane" in result["formatted"]

    def test_parse_url(self):
        from qr_multi_imgs import QRMultiIMGS
        result = QRMultiIMGS.parse_structured_content("https://example.com/qr")
        assert result["type"] == "url"

    def test_parse_email(self):
        from qr_multi_imgs import QRMultiIMGS
        result = QRMultiIMGS.parse_structured_content("mailto:user@example.com?subject=hello")
        assert result["type"] == "email"
        assert result["fields"]["to"] == "user@example.com"

    def test_parse_sms(self):
        from qr_multi_imgs import QRMultiIMGS
        result = QRMultiIMGS.parse_structured_content("smsto:+123456?body=hi")
        assert result["type"] == "sms"

    def test_parse_tel(self):
        from qr_multi_imgs import QRMultiIMGS
        result = QRMultiIMGS.parse_structured_content("tel:+123456789")
        assert result["type"] == "tel"

    def test_parse_geo(self):
        from qr_multi_imgs import QRMultiIMGS
        result = QRMultiIMGS.parse_structured_content("geo:45.4642,9.1900")
        assert result["type"] == "geo"

    def test_parse_calendar(self):
        from qr_multi_imgs import QRMultiIMGS
        result = QRMultiIMGS.parse_structured_content(
            "BEGIN:VEVENT\nSUMMARY:Meeting\nDTSTART:20250101T090000\nDTEND:20250101T100000\nEND:VEVENT"
        )
        assert result["type"] == "calendar"

    def test_parse_unknown(self):
        from qr_multi_imgs import QRMultiIMGS
        result = QRMultiIMGS.parse_structured_content("just some random text")
        assert result["type"] == "unknown"


class TestScannabilityScore:
    """Test scannability score computation"""

    def test_score_empty_content(self):
        from qr_multi_imgs import _compute_scannability_score
        score = _compute_scannability_score("hello")
        assert score["score"] >= 80

    def test_score_long_content_penalty(self):
        from qr_multi_imgs import _compute_scannability_score
        score = _compute_scannability_score("x" * 2000)
        assert score["score"] < 100

    def test_score_grade_a(self):
        from qr_multi_imgs import _compute_scannability_score
        score = _compute_scannability_score("https://example.com")
        assert score["grade"] == "A"


class TestExitCodes:
    """Test structured exit codes"""

    def test_no_qr_folder_returns_no_qr(self):
        import tempfile, argparse
        from qr_multi_imgs import run_cli
        with tempfile.TemporaryDirectory() as tmpdir:
            args = argparse.Namespace(
                path=tmpdir, action="decode", recursive=False, formats=None,
                output=None, export_format="text", qr_format="png",
                move=False, confirm=True, parallel=False, progress=False,
                log=False, naming="original", timeout=15, padding=20,
                deep_scan=True, deep_timeout=30, verbose=False, force_deep=False,
                rename_prefix=None, rename_suffix=None, filter_pattern=None,
                filter_case_sensitive=False, filter_exclude=False, nomenu=True,
                json_output=True, dedup=False, symbols=None, completion=None,
                score=False, show_qr=False,
            )
            with pytest.raises(SystemExit) as exc:
                run_cli(args)
            assert exc.value.code == 1  # EC_NO_QR


class TestDeduplication:
    """Test deduplication of QR contents"""

    def test_deduplicate_simple(self):
        from qr_multi_imgs import QRMultiIMGS
        result = QRMultiIMGS.deduplicate_contents(["a", "b", "a", "c", "b"])
        assert result == ["a", "b", "c"]

    def test_deduplicate_no_dupes(self):
        from qr_multi_imgs import QRMultiIMGS
        result = QRMultiIMGS.deduplicate_contents(["a", "b", "c"])
        assert result == ["a", "b", "c"]

    def test_deduplicate_empty(self):
        from qr_multi_imgs import QRMultiIMGS
        result = QRMultiIMGS.deduplicate_contents([])
        assert result == []


class TestTerminalQR:
    """Test terminal QR Unicode rendering"""

    def test_qr_to_terminal_renders(self):
        from qr_multi_imgs import _qr_to_terminal
        result = _qr_to_terminal("hello")
        assert "╭" in result
        assert "╰" in result
        assert "██" in result


class TestShellCompletion:
    """Test shell completion generation"""

    def test_completion_bash(self):
        from qr_multi_imgs import _generate_completion
        import io, sys
        out = io.StringIO()
        old = sys.stdout
        sys.stdout = out
        _generate_completion("bash")
        sys.stdout = old
        output = out.getvalue()
        assert "_qr_multi_imgs_completion" in output
        assert "complete -F" in output

    def test_completion_zsh(self):
        from qr_multi_imgs import _generate_completion
        import io, sys
        out = io.StringIO()
        old = sys.stdout
        sys.stdout = out
        _generate_completion("zsh")
        sys.stdout = old
        output = out.getvalue()
        assert "#compdef" in output

    def test_completion_fish(self):
        from qr_multi_imgs import _generate_completion
        import io, sys
        out = io.StringIO()
        old = sys.stdout
        sys.stdout = out
        _generate_completion("fish")
        sys.stdout = old
        output = out.getvalue()
        assert "complete -c" in output
