# typed: false
# frozen_string_literal: true

class GhStartSearch < Formula
  desc "GH CLI extension to search your stars "
  homepage "https://github.com/kyleking/gh-start-search"
  license "MIT"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "#{homepage}/releases/download/v#{version}/gh-start-search-darwin-arm64"
      sha256 "REPLACE_WITH_SHA256_FOR_DARWIN_ARM64"
    else
      url "#{homepage}/releases/download/v#{version}/gh-start-search-darwin-amd64"
      sha256 "REPLACE_WITH_SHA256_FOR_DARWIN_AMD64"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "#{homepage}/releases/download/v#{version}/gh-start-search-linux-arm64"
      sha256 "REPLACE_WITH_SHA256_FOR_LINUX_ARM64"
    else
      url "#{homepage}/releases/download/v#{version}/gh-start-search-linux-amd64"
      sha256 "REPLACE_WITH_SHA256_FOR_LINUX_AMD64"
    end
  end

  def install
    binary_name = "gh-start-search-#{OS.kernel_name.downcase}-#{Hardware::CPU.arch}"
    bin.install binary_name => "gh-start-search"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/gh-start-search --version")
  end
end
