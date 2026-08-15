package table

import (
	"os"
	"strings"
)

func UseASCII(flagASCII bool, goos string, getenv func(string) string) bool {
	if getenv == nil {
		getenv = os.Getenv
	}
	if flagASCII {
		return true
	}
	if getenv("WHERETOKEN_ASCII") == "1" || getenv("WHERETOKEN_ASCII") == "true" {
		return true
	}
	if getenv("NO_UTF8") != "" {
		return true
	}
	if goos != "windows" {
		return false
	}
	if getenv("WT_SESSION") != "" {
		return false
	}
	term := getenv("TERM")
	if strings.Contains(term, "xterm") || strings.Contains(term, "vt100") || term == "alacritty" {
		return false
	}
	return true
}

func UseColor(flagNoColor bool, tty bool, getenv func(string) string) bool {
	if getenv == nil {
		getenv = os.Getenv
	}
	if flagNoColor {
		return false
	}
	if getenv("NO_COLOR") != "" {
		return false
	}
	if getenv("TERM") == "dumb" {
		return false
	}
	if fc := getenv("FORCE_COLOR"); fc != "" && fc != "0" {
		return true
	}
	return tty
}

func Dim(s string, color bool) string {
	if !color || s == "" {
		return s
	}
	return "\x1b[2m" + s + "\x1b[0m"
}
