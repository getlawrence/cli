class Lawrence < Formula
  desc "CLI tool to analyze codebases and instrument them with OpenTelemetry"
  homepage "https://github.com/getlawrence/cli"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/getlawrence/cli/releases/download/v0.1.0-beta.5/lawrence_v0.1.0-beta.5_darwin_arm64.tar.gz"
      sha256 "424ebdea6b860e353788cd0e287938b9fd02c9df62fbc1e5d63eda7e653cc8f6"
    else
      url "https://github.com/getlawrence/cli/releases/download/v0.1.0-beta.5/lawrence_v0.1.0-beta.5_darwin_amd64.tar.gz"
      sha256 "f2e44b42b3c6e4484c184a4dcfd0ad9c7c8d27d7911e080bab76f116194ea907"
    end
  end

  on_linux do
    url "https://github.com/getlawrence/cli/releases/download/v0.1.0-beta.5/lawrence_v0.1.0-beta.5_linux_amd64.tar.gz"
    sha256 "a0c55761f1c490aaead2269944b741d861a5f71cce5bad3a129c6a6b19c517bd"
  end

  def install
    # The archive contains lawrence-{OS}-{ARCH} binary
    if Hardware::CPU.arm?
      bin.install "lawrence-darwin-arm64" => "lawrence"
    elsif OS.mac?
      bin.install "lawrence-darwin-amd64" => "lawrence"
    else
      bin.install "lawrence-linux-amd64" => "lawrence"
    end
  end

  test do
    output = shell_output("#{bin}/lawrence --version")
    assert_match "Lawrence CLI", output
  end
end
