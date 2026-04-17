#!/usr/bin/env python3
"""
Test for methods 4,5,6 activation - verifies methods are called in detection flow
"""

import inspect
import pytest


class TestMethods456Activation:
    """Test that methods 4,5,6 are called in detection phases."""

    def test_method4_sharpen_in_phase2(self):
        """Method 4 (Sharpening) should be called in phase2"""
        from qr_multi_imgs import QRMultiIMGS
        import qr_multi_imgs

        source = inspect.getsource(qr_multi_imgs.QRMultiIMGS._detect_phase2)

        assert "_detect_qr_method4_sharpen" in source or "method4" in source.lower(), (
            "Method 4 (Sharpening) is NOT called in _detect_phase2! "
            "This method exists but is never used."
        )

    def test_method5_deblur_in_phase2(self):
        """Method 5 (Deblur) should be called in phase2"""
        from qr_multi_imgs import QRMultiIMGS
        import qr_multi_imgs

        source = inspect.getsource(qr_multi_imgs.QRMultiIMGS._detect_phase2)

        assert "_detect_qr_method5_deblur" in source or "method5" in source.lower(), (
            "Method 5 (Deblur) is NOT called in _detect_phase2! "
            "This method exists but is never used."
        )

    def test_method6_rotation_in_phase3(self):
        """Method 6 (Rotation) should be called in phase3"""
        from qr_multi_imgs import QRMultiIMGS
        import qr_multi_imgs

        source = inspect.getsource(qr_multi_imgs.QRMultiIMGS._detect_phase3)

        assert "_detect_qr_method6_rotation" in source or "method6" in source.lower(), (
            "Method 6 (Rotation) is NOT called in _detect_phase3! "
            "This method exists but is never used."
        )


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
