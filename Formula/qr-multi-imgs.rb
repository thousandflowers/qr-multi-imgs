require "language/python"

class QrMultiImgs < Formula
  desc "QR Code Scanner for Images - Scan folders of images to detect QR codes"
  homepage "https://github.com/thousandflowers/qr-multi-imgs"
  url "https://github.com/thousandflowers/qr-multi-imgs/archive/refs/tags/v0.3.3.tar.gz"
  version "0.3.3"
  license "MIT"

  depends_on "zbar"
  depends_on "python@3.12"

  def install
    # Create a wrapper script that calls python3
    bin.mkpath
    (bin/"qr-multi-imgs").write <<~EOS
#!/bin/bash
exec python3 "#{prefix}/share/qr_multi_imgs.py" "$@"
EOS
    chmod "+x", bin/"qr-multi-imgs"

    # Install the Python script to share folder
    (share).install "qr_multi_imgs.py"
  end

  def caveats
    <<~EOS
      QR Multi IMGS has been installed successfully!

      Usage:
        qr-multi-imgs          # Start with interactive menu
        qr-multi-imgs --help  # Show CLI options

      Note: Requires zbar library. If you get errors:
        brew reinstall zbar

      For more info: https://github.com/thousandflowers/qr-multi-imgs
    EOS
  end

  test do
    assert_match "QR Multi IMGS", shell_output("#{bin}/qr-multi-imgs --help 2>&1")
  end
end