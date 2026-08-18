package index

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

const maxJSONLLine = 10 * 1024 * 1024

// ScanJSONL calls fn for each newline-terminated line and returns the absolute
// file offset of the first unconsumed byte.
//
// Invariant: a complete line (ends with \n, including a malformed JSON line)
// is consumed even if fn ignores it. Only a trailing fragment with no newline
// is left unconsumed. The next incremental scan seeks there and rereads it
// after the writer finishes the record.
//
// This is one pass to the EOF this reader observes. Bytes appended after that
// EOF are the next scan's job.
func ScanJSONL(f *os.File, fn func(line []byte, at int64) error) (consumed int64, err error) {
	start, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	r := bufio.NewReaderSize(f, 64*1024)
	var n int64
	for {
		raw, complete, err := readCompleteLine(r)
		if complete {
			rec := raw[:len(raw)-1]
			if len(rec) > 0 && rec[len(rec)-1] == '\r' {
				rec = rec[:len(rec)-1]
			}
			if err := fn(rec, start+n); err != nil {
				return start + n, err
			}
			n += int64(len(raw))
			if err == io.EOF {
				return start + n, nil
			}
			continue
		}
		if err == io.EOF || err == nil {
			return start + n, nil
		}
		return start + n, err
	}
}

func readCompleteLine(r *bufio.Reader) (line []byte, complete bool, err error) {
	var buf []byte
	for {
		part, e := r.ReadSlice('\n')
		buf = append(buf, part...)
		if len(buf) > maxJSONLLine {
			return nil, false, fmt.Errorf("jsonl line exceeds %d bytes", maxJSONLLine)
		}
		if len(part) > 0 && part[len(part)-1] == '\n' {
			return buf, true, e
		}
		if e == bufio.ErrBufferFull {
			continue
		}
		return buf, false, e
	}
}
