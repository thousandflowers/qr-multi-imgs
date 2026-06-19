class QrMultiImgs < Formula
  desc "Scan a folder of images for QR codes — decode, organize, export, recreate"
  homepage "https://github.com/thousandflowers/qr-multi-imgs"
  url "https://github.com/thousandflowers/qr-multi-imgs/archive/refs/tags/v1.1.0.tar.gz"
  sha256 "6bc6d0c5a9ca5532b0f0a3a07a66ba9ddccae02f9943c15629e2703c6c77b58f"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", "-o", bin/"qr-multi-imgs", "."
  end

  test do
    assert_match "qr-multi-imgs #{version}", shell_output("#{bin}/qr-multi-imgs --version")
  end
end
