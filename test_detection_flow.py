#!/usr/bin/env python3
"""
Test for detection flow - verifies method7_multiscale is called
"""

import tempfile
import pytest
from pathlib import Path
from PIL import Image
import qrcode
import os
import inspect


class TestDetectionMethod7Enabled:
    """Test that method7_multiscale is actually called in detection flow."""

    def test_method7_exists_and_is_callable(self):
        """Method7 should exist and be callable"""
        from qr_multi_imgs import QRMultiIMGS

        scanner = QRMultiIMGS(folder_path="/tmp")
        assert hasattr(scanner, "_detect_qr_method7_multiscale"), (
            "Method7 _detect_qr_method7_multiscale does not exist!"
        )

    def test_detect_full_method_not_duplicated(self):
        """_detect_full should be defined only once (not duplicated)"""
        from qr_multi_imgs import QRMultiIMGS
        import qr_multi_imgs

        methods = [
            m for m in dir(qr_multi_imgs.QRMultiIMGS) if m.startswith("_detect_full")
        ]
        assert len(methods) == 1, (
            f"_detect_full is duplicated! Found {len(methods)} definitions: {methods}"
        )

    def test_method7_called_in_detection_flow(self):
        """Method7 should be called when force_deep is enabled - in phase3"""
        from qr_multi_imgs import QRMultiIMGS
        import qr_multi_imgs

        source = inspect.getsource(qr_multi_imgs.QRMultiIMGS._detect_phase3)

        assert (
            "_detect_qr_method7_multiscale" in source or "method7_multiscale" in source
        ), "Method7 (multiscale) is NOT called in _detect_phase3 flow!"

    def test_multiscale_method_works_on_small_qr(self):
        """Multiscale detection should help with small QR codes"""
        from qr_multi_imgs import QRMultiIMGS, QRCodeResult
        import tempfile

        with tempfile.TemporaryDirectory() as tmpdir:
            scanner = QRMultiIMGS(folder_path=tmpdir, force_deep=True)

            qr = qrcode.QRCode(
                version=1, error_correction=qrcode.constants.ERROR_CORRECT_L
            )
            qr.add_data("TEST")
            qr.make()

            img = qr.make_image(fill_color="black", back_color="white")
            img = img.resize((50, 50))

            test_path = Path(tmpdir) / "small_qr.png"
            img.save(str(test_path))

            result = scanner.detect_qr(test_path)

            assert result.has_qr, (
                f"Small QR should be detected with multiscale! "
                f"Method7 may not be working properly."
            )


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
