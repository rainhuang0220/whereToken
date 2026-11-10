//go:build !windows

package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/scan"
)

func TestPTYShowsMascotBeforeFinish(t *testing.T) {
	master, slave := openPTY(t)
	defer master.Close()
	defer slave.Close()
	h := &scanHUD{w: slave}
	h.arm(40*time.Millisecond, scan.Progress{Label: reportWaitLabel, Status: scan.ProgressReading})
	deadline := time.Now().Add(2 * time.Second)
	var got []byte
	buf := make([]byte, 4096)
	for time.Now().Before(deadline) && !strings.Contains(string(got), reportWaitLabel) {
		_ = master.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, _ := master.Read(buf)
		got = append(got, buf[:n]...)
	}
	h.finish()
	if !strings.Contains(string(got), reportWaitLabel) || !strings.Contains(string(got), "▄██████▄") {
		t.Fatalf("pty did not show the mascot before finish:\n%q", got)
	}
	rest := make([]byte, 4096)
	_ = master.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	n, _ := master.Read(rest)
	after := string(rest[:n])
	if strings.Contains(after, reportWaitLabel) && !strings.Contains(after, "\x1b") {
		t.Fatalf("mascot continued after finish: %q", after)
	}
}
