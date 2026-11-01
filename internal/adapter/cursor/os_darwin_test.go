//go:build darwin

package cursor

import (
	"testing"
	"unsafe"
)

func TestCPtrReadsTheWordAtAGoAddress(t *testing.T) {
	word := uintptr(0x1122334455667788)
	got := *(*uintptr)(cPtr(uintptr(unsafe.Pointer(&word))))
	if got != word {
		t.Fatalf("cPtr read %#x, want %#x", got, word)
	}
}

func TestBogusKeychainItemDoesNotPrompt(t *testing.T) {
	_, err := readGenericPassword("wheretoken-test-missing-item", "no-such-account", false)
	if err == nil {
		t.Fatal("missing item returned a password")
	}
	if err != errKeychainMiss && err != errOSPermission && err != errSealedCredential {
		t.Fatalf("err=%v", err)
	}
}
