# typed: false
# frozen_string_literal: true

class GhStarSearch < Formula
  desc "GH CLI extension to search your stars "
  homepage "https://github.com/KyleKing/gh-star-search"
  license "MIT"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "#{homepage}/releases/download/v#{version}/gh-star-search-darwin-arm64"
      sha256 "REPLACE_WITH_SHA256_FOR_DARWIN_ARM64"
    else
      url "#{homepage}/releases/download/v#{version}/gh-star-search-darwin-amd64"
      sha256 "REPLACE_WITH_SHA256_FOR_DARWIN_AMD64"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "#{homepage}/releases/download/v#{version}/gh-star-search-linux-arm64"
      sha256 "REPLACE_WITH_SHA256_FOR_LINUX_ARM64"
    else
      url "#{homepage}/releases/download/v#{version}/gh-star-search-linux-amd64"
      sha256 "REPLACE_WITH_SHA256_FOR_LINUX_AMD64"
    end
  end

  def install
    binary_name = "gh-star-search-#{OS.kernel_name.downcase}-#{Hardware::CPU.arch}"
    bin.install binary_name => "gh-star-search"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/gh-star-search --version")
  end
end
