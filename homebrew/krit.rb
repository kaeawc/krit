class Krit < Formula
  desc "Fast Kotlin static analysis powered by tree-sitter"
  homepage "https://github.com/kaeawc/krit"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/kaeawc/krit/releases/download/v#{version}/krit_#{version}_darwin_arm64.tar.gz"
      sha256 "PLACEHOLDER"
    else
      url "https://github.com/kaeawc/krit/releases/download/v#{version}/krit_#{version}_darwin_amd64.tar.gz"
      sha256 "PLACEHOLDER"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/kaeawc/krit/releases/download/v#{version}/krit_#{version}_linux_arm64.tar.gz"
      sha256 "PLACEHOLDER"
    else
      url "https://github.com/kaeawc/krit/releases/download/v#{version}/krit_#{version}_linux_amd64.tar.gz"
      sha256 "PLACEHOLDER"
    end
  end

  def install
    bin.install "krit"
    bin.install "krit-lsp"
    bin.install "krit-mcp"
  end

  test do
    system "#{bin}/krit", "--version"
  end
end
