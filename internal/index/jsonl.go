package index

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

const maxJSONLLine = 10 * 1024 * 1024

// ScanJSONL calls fn for each newline-terminated line. A trailing fragment
// without a newline is left unconsumed so the next incremental scan can
// reread it after the writer finishes the record.
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
