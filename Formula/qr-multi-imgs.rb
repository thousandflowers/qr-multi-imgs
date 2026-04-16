require "language/python"

class QrMultiImgs < Formula
  desc "QR Code Scanner for Images - Scan folders of images to detect QR codes"
  homepage "https://github.com/thousandflowers/qr-multi-imgs"
  url "https://github.com/thousandflowers/qr-multi-imgs/archive/refs/tags/v0.3.1.tar.gz"
  version "0.3.1"
  license "MIT"

  depends_on "zbar"
  depends_on "python@3.12"

  def install
    python = Formula["python@3.12"]
    python_path = python.opt_libexec/"python3.12/site-packages"

    # Install Python dependencies using pip
    system python.bin/"pip3.12", "install",
      "--target=#{python_path}",
      "textual>=0.80.0",
      "pyzbar>=0.1.9",
      "Pillow>=10.0.0",
      "qrcode>=7.4.2"

    # Create the bin directory if it doesn't exist
    bin.mkpath

    # Install the main script
    bin.install "qr_multi_imgs.py" => "qr-multi-imgs"
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