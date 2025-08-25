class Lawrence < Formula
  desc "CLI tool to analyze codebases and instrument them with OpenTelemetry"
  homepage "https://github.com/getlawrence/cli"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/getlawrence/cli/releases/latest/download/lawrence_darwin_arm64.tar.gz"
      sha256 "placeholder_sha256_for_arm64" # This will be updated by the release workflow
    else
      url "https://github.com/getlawrence/cli/releases/latest/download/lawrence_darwin_amd64.tar.gz"
      sha256 "placeholder_sha256_for_amd64" # This will be updated by the release workflow
    end
  end

  on_linux do
    url "https://github.com/getlawrence/cli/releases/latest/download/lawrence_linux_amd64.tar.gz"
    sha256 "placeholder_sha256_for_linux" # This will be updated by the release workflow
  end

  def install
    bin.install "lawrence"
  end

  test do
    output = shell_output("#{bin}/lawrence --version")
    assert_match "Lawrence CLI", output
  end
end
