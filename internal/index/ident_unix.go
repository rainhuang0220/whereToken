//go:build unix

package index

import (
	"os"
	"syscall"
)

func inodeOf(st os.FileInfo) int64 {
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		return 0
	}
	return int64(sys.Ino)
}
