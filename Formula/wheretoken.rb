class Wheretoken < Formula
  desc "Local coding-agent token usage as a character table"
  homepage "https://github.com/rainhuang0220/whereToken"
  url "https://github.com/rainhuang0220/whereToken/archive/refs/tags/v0.6.2.tar.gz"
  sha256 "3d6052f71cdaf1d2928214f7d93bc77210513f14c24574afa58436455872f9f3"
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
