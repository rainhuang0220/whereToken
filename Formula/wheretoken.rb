class Wheretoken < Formula
  desc "Local coding-agent token usage as a character table"
  homepage "https://github.com/rainhuang0220/whereToken"
  url "https://github.com/rainhuang0220/whereToken/archive/refs/tags/v0.2.0.tar.gz"
  sha256 "ca10e3e1bc64f54162eb9610e63b4dfd4a55091013229806d4188f088e5c6bb5"
  license "MIT"
  head "https://github.com/rainhuang0220/whereToken.git", branch: "main"

  depends_on "go" => :build

  def install
    ver = "head"
    ver = version.to_s unless build.head?
    ldflags = "-s -w -X main.version=#{ver}"
    system "go", "build", *std_go_args(ldflags: ldflags), "./cmd/wheretoken"
    man1.install "docs/wheretoken.1"
    bash_completion.install "completions/wheretoken.bash" => "wheretoken"
    zsh_completion.install "completions/_wheretoken"
    fish_completion.install "completions/wheretoken.fish"
  end

  test do
    assert_match "USAGE", shell_output("#{bin}/wheretoken --help")
  end
end
