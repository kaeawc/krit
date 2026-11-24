cask "krit" do
  version "0.2.0"

  on_macos do
    on_arm do
      sha256 "00e2170d07aa0aa53fce0dd8a4cc9a8a4f93b0f7858b38fb2bd2d8158c1b8151"
      url "https://github.com/kaeawc/krit/releases/download/v0.2.0/krit_#{version}_darwin_arm64.tar.gz"
    end
    on_intel do
      sha256 "8c80fdbdd8267fc800e082bd58ef16362e375c861496a1579e48d22b430fb096"
      url "https://github.com/kaeawc/krit/releases/download/v0.2.0/krit_#{version}_darwin_amd64.tar.gz"
    end
  end
  on_linux do
    on_arm do
      sha256 "3006918a2da8f4e1c6cfe032d2688f138748cc6d0a7497686dfa9a3ad58a1f12"
      url "https://github.com/kaeawc/krit/releases/download/v0.2.0/krit_#{version}_linux_arm64.tar.gz"
    end
    on_intel do
      sha256 "ccb39239e547e4141db7f73ec434f09cc3a9b99ed23a3779f586c05d5680408d"
      url "https://github.com/kaeawc/krit/releases/download/v0.2.0/krit_#{version}_linux_amd64.tar.gz"
    end
  end

  name "krit"
  desc "Fast Kotlin static analysis powered by tree-sitter"
  homepage "https://github.com/kaeawc/krit"

  binary "krit"
  binary "krit-lsp"
  binary "krit-mcp"
end
