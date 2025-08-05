class Lawrence < Formula
  desc "CLI tool to analyze codebases and instrument them with OpenTelemetry"
  homepage "https://github.com/getlawrence/cli"
  url "https://github.com/getlawrence/cli/archive/v0.1.0.tar.gz"
  sha256 "YOUR_SHA256_HERE"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w", output: bin/"lawrence")
  end

  test do
    assert_match "Lawrence CLI", shell_output("#{bin}/lawrence --help")
  end
end
