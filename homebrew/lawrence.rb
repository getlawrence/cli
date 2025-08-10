class Lawrence < Formula
  desc "CLI tool to analyze codebases and instrument them with OpenTelemetry"
  homepage "https://github.com/getlawrence/cli"
  head "https://github.com/getlawrence/cli.git", branch: "main"
  license "MIT"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X github.com/getlawrence/cli/cmd.Version=#{version}
      -X github.com/getlawrence/cli/cmd.GitCommit=homebrew
      -X github.com/getlawrence/cli/cmd.BuildDate=#{Time.now.utc.iso8601}
    ]
    system "go", "build", *std_go_args(ldflags: ldflags.join(" "), output: bin/"lawrence")
  end

  test do
    output = shell_output("#{bin}/lawrence --version")
    assert_match version.to_s, output
  end
end
