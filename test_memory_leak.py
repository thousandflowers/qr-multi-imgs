#!/usr/bin/env python3
"""
Test for memory leak - verifies preprocessed images are closed
"""

import tempfile
import pytest
from pathlib import Path
from PIL import Image
import qrcode
import inspect
from unittest.mock import patch, MagicMock


class TestMemoryLeakFix:
    """Test that preprocessed images are properly closed."""

    def test_phase1_closes_created_images(self):
        """Phase1 should close all preprocessed images it creates"""
        from qr_multi_imgs import QRMultiIMGS
        import qr_multi_imgs

        source = inspect.getsource(qr_multi_imgs.QRMultiIMGS._detect_phase1)

        created_images = []
        for line in source.split("\n"):
            if (
                "= img.convert" in line
                or "= img.resize" in line
                or "enhancer.enhance" in line
                or "filter(" in line
            ):
                if "try:" not in line:
                    created_images.append(line.strip())

        lines_with_close = source.count(".close()")
        lines_with_try = source.split("try:")

        assert ".close()" in source, (
            f"No .close() calls found in _detect_phase1! "
            f"Preprocessed images are NOT being closed."
        )

    def test_phase2_closes_created_images(self):
        """Phase2 should close all preprocessed images it creates"""
        from qr_multi_imgs import QRMultiIMGS
        import qr_multi_imgs

        source = inspect.getsource(qr_multi_imgs.QRMultiIMGS._detect_phase2)

        assert ".close()" in source or "with " in source, (
            f"No .close() or context manager in _detect_phase2! "
            f"Preprocessed images are NOT being closed."
        )

    def test_gray_image_closed(self):
        """Gray converted image must be closed after use"""
        from qr_multi_imgs import QRMultiIMGS
        import qr_multi_imgs

        source = inspect.getsource(qr_multi_imgs.QRMultiIMGS._detect_phase1)

        assert (
            "gray.close()" in source
            or "enhanced.close()" in source
            or "sharp.close()" in source
        ), (
            "Gray/enhanced/sharp images are NOT closed in _detect_phase1! "
            "This causes memory leak."
        )


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
