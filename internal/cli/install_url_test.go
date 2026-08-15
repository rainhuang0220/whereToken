package cli

import "testing"

func TestReleaseAssetNamesMatchGoreleaserAndNpm(t *testing.T) {
	// Keep in lockstep with .goreleaser.yaml name_template and npm/lib/platform.js.
	want := []string{
		"wheretoken_darwin_arm64.tar.gz",
		"wheretoken_darwin_amd64.tar.gz",
		"wheretoken_linux_amd64.tar.gz",
		"wheretoken_linux_arm64.tar.gz",
		"wheretoken_windows_amd64.zip",
		"wheretoken_windows_arm64.zip",
	}
	for _, name := range want {
		osName, arch, ext := parseAsset(name)
		if osName == "" || arch == "" || ext == "" {
			t.Fatalf("bad fixture %s", name)
		}
		got := "wheretoken_" + osName + "_" + arch + "." + ext
		if got != name {
			t.Fatalf("%s != %s", got, name)
		}
	}
}

func parseAsset(name string) (osName, arch, ext string) {
	switch {
	case len(name) > 4 && name[len(name)-4:] == ".zip":
		ext = "zip"
		name = name[:len(name)-4]
	case len(name) > 7 && name[len(name)-7:] == ".tar.gz":
		ext = "tar.gz"
		name = name[:len(name)-7]
	}
	const p = "wheretoken_"
	if len(name) < len(p) || name[:len(p)] != p {
		return "", "", ""
	}
	rest := name[len(p):]
	i := 0
	for i < len(rest) && rest[i] != '_' {
		i++
	}
	if i == 0 || i == len(rest) {
		return "", "", ""
	}
	return rest[:i], rest[i+1:], ext
}
