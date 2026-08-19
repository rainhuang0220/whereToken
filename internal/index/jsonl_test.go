package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func scanLines(t *testing.T, raw string) (lines []string, at []int64, consumed int64) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "a.jsonl")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	consumed, err = ScanJSONL(f, func(line []byte, off int64) error {
		lines = append(lines, string(line))
		at = append(at, off)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return lines, at, consumed
}

func TestScanJSONLEmptyFile(t *testing.T) {
	lines, _, consumed := scanLines(t, "")
	if len(lines) != 0 || consumed != 0 {
		t.Fatalf("lines=%v consumed=%d", lines, consumed)
	}
}

func TestScanJSONLSingleCompleteLine(t *testing.T) {
	lines, at, consumed := scanLines(t, "a\n")
	if len(lines) != 1 || lines[0] != "a" || at[0] != 0 || consumed != 2 {
		t.Fatalf("lines=%v at=%v consumed=%d", lines, at, consumed)
	}
}

func TestScanJSONLTwoCompleteLines(t *testing.T) {
	lines, at, consumed := scanLines(t, "a\nb\n")
	if len(lines) != 2 || lines[0] != "a" || lines[1] != "b" || at[0] != 0 || at[1] != 2 || consumed != 4 {
		t.Fatalf("lines=%v at=%v consumed=%d", lines, at, consumed)
	}
}

func TestScanJSONLIncompleteLastLine(t *testing.T) {
	lines, _, consumed := scanLines(t, "a\nb")
	if len(lines) != 1 || lines[0] != "a" || consumed != 2 {
		t.Fatalf("incomplete tail must not be consumed: lines=%v consumed=%d", lines, consumed)
	}
}

func TestScanJSONLIncompleteAfterTwoLines(t *testing.T) {
	lines, _, consumed := scanLines(t, "a\nb\nc")
	if len(lines) != 2 || lines[0] != "a" || lines[1] != "b" || consumed != 4 {
		t.Fatalf("lines=%v consumed=%d", lines, consumed)
	}
}

func TestScanJSONLCRLF(t *testing.T) {
	lines, _, consumed := scanLines(t, "a\r\nb\r\n")
	if len(lines) != 2 || lines[0] != "a" || lines[1] != "b" || consumed != 6 {
		t.Fatalf("crlf lines=%v consumed=%d", lines, consumed)
	}
}

func TestScanJSONLOnlyIncomplete(t *testing.T) {
	lines, _, consumed := scanLines(t, "no-newline")
	if len(lines) != 0 || consumed != 0 {
		t.Fatalf("lines=%v consumed=%d", lines, consumed)
	}
}

func TestScanJSONLMalformedCompleteLineIsConsumed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.jsonl")
	raw := "good\nnot-json\nalso-good\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var kept []string
	consumed, err := ScanJSONL(f, func(line []byte, _ int64) error {
		if line[0] == 'n' {
			return nil // adapter skips malformed
		}
		kept = append(kept, string(line))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 2 || kept[0] != "good" || kept[1] != "also-good" {
		t.Fatalf("kept=%v", kept)
	}
	if consumed != int64(len(raw)) {
		t.Fatalf("malformed complete line must still advance offset: %d want %d", consumed, len(raw))
	}
}

func TestScanJSONLFromNonZeroOffset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.jsonl")
	raw := "aaa\nbbb\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Seek(4, 0); err != nil {
		t.Fatal(err)
	}
	var lines []string
	var at []int64
	consumed, err := ScanJSONL(f, func(line []byte, off int64) error {
		lines = append(lines, string(line))
		at = append(at, off)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0] != "bbb" || at[0] != 4 || consumed != 8 {
		t.Fatalf("lines=%v at=%v consumed=%d", lines, at, consumed)
	}
}

func TestScanJSONLResumeAfterIncompleteThenComplete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.jsonl")
	if err := os.WriteFile(path, []byte("a\nb"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	consumed, err := ScanJSONL(f, func(line []byte, _ int64) error { return nil })
	f.Close()
	if err != nil || consumed != 2 {
		t.Fatalf("first consumed=%d err=%v", consumed, err)
	}
	af, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := af.WriteString("\n"); err != nil {
		t.Fatal(err)
	}
	af.Close()
	f, err = os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(consumed, 0); err != nil {
		t.Fatal(err)
	}
	var lines []string
	consumed2, err := ScanJSONL(f, func(line []byte, _ int64) error {
		lines = append(lines, string(line))
		return nil
	})
	f.Close()
	if err != nil || len(lines) != 1 || lines[0] != "b" || consumed2 != 4 {
		t.Fatalf("resume lines=%v consumed=%d err=%v", lines, consumed2, err)
	}
}

func TestScanJSONLOversizeCompleteLineIsSkipped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.jsonl")
	huge := strings.Repeat("x", maxJSONLLine+8)
	raw := "keep-a\n" + huge + "\nkeep-b\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var kept []string
	consumed, err := ScanJSONL(f, func(line []byte, _ int64) error {
		kept = append(kept, string(line))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 2 || kept[0] != "keep-a" || kept[1] != "keep-b" {
		t.Fatalf("oversize complete line must be skipped: %v", kept)
	}
	if consumed != int64(len(raw)) {
		t.Fatalf("consumed=%d want %d", consumed, len(raw))
	}
}

func TestScanJSONLOversizeIncompleteLineErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.jsonl")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", maxJSONLLine+1)), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, err = ScanJSONL(f, func([]byte, int64) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("err=%v", err)
	}
}
