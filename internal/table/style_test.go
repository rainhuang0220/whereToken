package table

import "testing"

func TestUseASCIIWindowsLegacy(t *testing.T) {
	env := func(k string) string { return "" }
	if !UseASCII(false, "windows", env) {
		t.Fatal("windows cmd should ascii")
	}
	if UseASCII(false, "darwin", env) {
		t.Fatal("mac should unicode")
	}
	if !UseASCII(true, "darwin", env) {
		t.Fatal("--ascii")
	}
	wt := func(k string) string {
		if k == "WT_SESSION" {
			return "abc"
		}
		return ""
	}
	if UseASCII(false, "windows", wt) {
		t.Fatal("windows terminal should unicode")
	}
}

func TestUseColorNO_COLOR(t *testing.T) {
	env := func(k string) string {
		if k == "NO_COLOR" {
			return "1"
		}
		return ""
	}
	if UseColor(false, true, env) {
		t.Fatal("NO_COLOR must win")
	}
	if UseColor(true, true, func(string) string { return "" }) {
		t.Fatal("--no-color")
	}
	if !UseColor(false, true, func(string) string { return "" }) {
		t.Fatal("tty should color")
	}
	if UseColor(false, false, func(string) string { return "" }) {
		t.Fatal("pipe should not color")
	}
}

func TestUseColorFORCE_COLOR(t *testing.T) {
	force := func(k string) string {
		if k == "FORCE_COLOR" {
			return "1"
		}
		return ""
	}
	if !UseColor(false, false, force) {
		t.Fatal("FORCE_COLOR should color a pipe")
	}
	both := func(k string) string {
		if k == "FORCE_COLOR" || k == "NO_COLOR" {
			return "1"
		}
		return ""
	}
	if UseColor(false, true, both) {
		t.Fatal("NO_COLOR must beat FORCE_COLOR")
	}
}

func TestDimEmptyWhenNoColor(t *testing.T) {
	if Dim("x", false) != "x" {
		t.Fatal(Dim("x", false))
	}
	if Dim("x", true) == "x" || !containsESC(Dim("x", true)) {
		t.Fatal(Dim("x", true))
	}
}

func containsESC(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			return true
		}
	}
	return false
}
