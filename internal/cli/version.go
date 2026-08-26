package cli

import (
	"runtime/debug"
	"strings"
)

func ResolveVersion(ldflag string) string {
	if ldflag != "" && ldflag != "dev" {
		return tidyVersion(ldflag)
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return tidyVersion(v)
		}
	}
	return "dev"
}

func tidyVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimSuffix(v, "+dirty")
	if strings.HasPrefix(v, "v0.0.0-") {
		i := strings.LastIndex(v, "-")
		if i >= 0 && i+1 < len(v) {
			rev := v[i+1:]
			if len(rev) > 7 {
				rev = rev[:7]
			}
			return "dev-" + rev
		}
	}
	return v
}
