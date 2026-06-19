class QrMultiImgs < Formula
  desc "Scan a folder of images for QR codes — decode, organize, export, recreate"
  homepage "https://github.com/thousandflowers/qr-multi-imgs"
  url "https://github.com/thousandflowers/qr-multi-imgs/archive/refs/tags/v1.2.0.tar.gz"
  sha256 "73f624db909b1813e0ee4adaf66d0b77f4337cd323fc636e985cbffa95cb0595"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", "-o", bin/"qr-multi-imgs", "."
  end

  test do
    assert_match "qr-multi-imgs #{version}", shell_output("#{bin}/qr-multi-imgs --version")
  end
end
