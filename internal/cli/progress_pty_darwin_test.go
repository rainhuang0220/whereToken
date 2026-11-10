//go:build darwin

package cli

import (
	"bytes"
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

func openPTY(t *testing.T) (master, slave *os.File) {
	t.Helper()
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	fd := int(m.Fd())
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCPTYGRANT, 0); err != nil {
		m.Close()
		t.Fatal(err)
	}
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCPTYUNLK, 0); err != nil {
		m.Close()
		t.Fatal(err)
	}
	buf := make([]byte, 128)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(unix.TIOCPTYGNAME), uintptr(unsafe.Pointer(&buf[0])))
	if errno != 0 {
		m.Close()
		t.Fatal(errno)
	}
	name := string(buf[:bytes.IndexByte(buf, 0)])
	s, err := os.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		m.Close()
		t.Fatal(err)
	}
	return m, s
}
