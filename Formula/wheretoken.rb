class Wheretoken < Formula
  desc "Local coding-agent token usage as a character table"
  homepage "https://github.com/rainhuang0220/whereToken"
  license "MIT"
  head "https://github.com/rainhuang0220/whereToken.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X main.version=head"
    system "go", "build", *std_go_args(ldflags: ldflags), "./cmd/wheretoken"
    man1.install "docs/wheretoken.1"
  end

  test do
    assert_match "USAGE", shell_output("#{bin}/wheretoken --help")
  end
end
