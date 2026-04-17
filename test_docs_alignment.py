#!/usr/bin/env python3
"""
Test for documentation alignment - verifies README matches code
"""

import tempfile
import pytest
from pathlib import Path
import inspect
import re


class TestDocumentationAlignment:
    """Test that documentation matches code."""

    def test_readme_detection_methods_count(self):
        """README should document all active detection methods"""
        from qr_multi_imgs import QRMultiIMGS
        import qr_multi_imgs

        all_methods = [
            m
            for m in dir(qr_multi_imgs.QRMultiIMGS)
            if m.startswith("_detect_qr_method") and "multiscale" not in m
        ]

        method_numbers = set()
        for m in all_methods:
            match = re.search(r"method(\d+)", m)
            if match:
                method_numbers.add(int(match.group(1)))

        active_in_code = len(method_numbers)

        readme_content = Path("README.md").read_text()
        readme_methods = readme_content.count("Method")

        assert active_in_code <= readme_methods, (
            f"Code has {active_in_code} methods but README mentions only {readme_methods}. "
            f"Documentation is out of sync!"
        )

    def test_phase3_calls_all_heavy_methods(self):
        """Phase3 should call multiple methods, not just QReader"""
        from qr_multi_imgs import QRMultiIMGS
        import qr_multi_imgs

        source = inspect.getsource(qr_multi_imgs.QRMultiIMGS._detect_phase3)

        method_calls = [
            "_detect_qr_method7_multiscale",
            "_detect_qr_method8_qreader",
            "_detect_qr_method9_adaptive",
            "_detect_qr_method10_morphology",
        ]

        called_count = sum(1 for m in method_calls if m in source)

        assert called_count >= 2, (
            f"Phase3 should call multiple methods, "
            f"but only {called_count} found in source!"
        )

    def test_readme_detection_flow_correct(self):
        """README detection flow should match actual implementation"""
        readme = Path("README.md").read_text()

        assert "Phase 1" in readme, "README should document Phase 1"
        assert "Phase 2" in readme, "README should document Phase 2"
        assert "Phase 3" in readme, "README should document Phase 3"

        flow_section = readme[readme.find("### Detection Flow") :]
        flow_section = flow_section[:500]

        assert "Multi-scale" in flow_section or "multiscale" in flow_section.lower(), (
            "README should mention multiscale in detection flow"
        )


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
