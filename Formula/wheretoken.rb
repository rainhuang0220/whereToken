class Wheretoken < Formula
  desc "Local coding-agent token usage as a character table"
  homepage "https://github.com/rainhuang0220/whereToken"
  url "https://github.com/rainhuang0220/whereToken/archive/refs/tags/v0.7.9.tar.gz"
  sha256 "9ab77137e3e3b3cdb758fb52fbf573857f71c45ca56a8e35c20181a8527ff817"
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
