require "language/python"

class QrMultiImg < Formula
  desc "QR Code Scanner for Images - Scan folders of images to detect QR codes"
  homepage "https://github.com/thousandflowers/qr-multi-img"
  url "https://github.com/thousandflowers/qr-multi-img/archive/refs/tags/v0.2.0.tar.gz"
  version "0.2.0"
  license "MIT"

  depends_on "zbar"
  depends_on python: "3.12"

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
    bin.install "qr_multi_img.py" => "qr-multi-img"
  end

  def caveats
    <<~EOS
      QR Multi IMG has been installed successfully!

      Usage:
        qr-multi-img          # Start with interactive menu
        qr-multi-img --help  # Show CLI options

      Note: Requires zbar library. If you get errors:
        brew reinstall zbar

      For more info: https://github.com/thousandflowers/qr-multi-img
    EOS
  end

  test do
    assert_match "QR Multi IMG", shell_output("#{bin}/qr-multi-img --help 2>&1")
  end
end