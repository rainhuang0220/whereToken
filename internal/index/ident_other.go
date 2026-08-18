//go:build !unix

package index

import "os"

func inodeOf(st os.FileInfo) int64 { return 0 }
