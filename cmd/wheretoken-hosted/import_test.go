package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestHostedBinaryDoesNotDependOnAdapters(t *testing.T) {
	out, err := exec.Command("go", "list", "-f", "{{join .Deps \"\\n\"}}", ".").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, dep := range strings.Split(string(out), "\n") {
		dep = strings.TrimSpace(dep)
		if strings.Contains(dep, "/internal/adapter/") && !strings.Contains(dep, "/internal/adapter/testhome") {
			t.Fatalf("wheretoken-hosted must not import scanners: %s", dep)
		}
		if dep == "github.com/rainhuang0220/whereToken/internal/scan" {
			t.Fatal("wheretoken-hosted must not import internal/scan")
		}
	}
}
