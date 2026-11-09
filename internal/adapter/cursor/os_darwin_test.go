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

func TestInteractiveSafeStorageReadNeverEntersLegacyNoUIQuery(t *testing.T) {
	for _, tc := range []struct {
		name, password string
		err            error
	}{
		{name: "authorized", password: "fixture-password"},
		{name: "user denied", err: errOSPermission},
		{name: "missing", err: errKeychainMiss},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			password, err := cursorSafeStoragePasswordUsing(true, func(service, account string, prompt bool) (string, error) {
				calls++
				if service != "Cursor Safe Storage" || account != "" || !prompt {
					t.Errorf("interactive read entered wrong Keychain query: service=%q account=%q prompt=%v", service, account, prompt)
					return "", errOSPermission
				}
				return tc.password, tc.err
			})
			if calls != 1 || password != tc.password || err != tc.err {
				t.Fatalf("interactive query retried or changed result: calls=%d password matches=%v err=%v want=%v", calls, password == tc.password, err, tc.err)
			}
		})
	}
}

func TestBackgroundSafeStorageReadKeepsNonUIAccountOrder(t *testing.T) {
	for _, tc := range []struct {
		name, found string
		wantCalls   int
		err         error
	}{
		{name: "unqualified", found: "", wantCalls: 1},
		{name: "Cursor", found: "Cursor", wantCalls: 2},
		{name: "Cursor Key", found: "Cursor Key", wantCalls: 3},
		{name: "all missing", found: "missing", wantCalls: 3, err: errSealedCredential},
		{name: "blocked", found: "blocked", wantCalls: 1, err: errOSPermission},
	} {
		t.Run(tc.name, func(t *testing.T) {
			accounts := []string{"", "Cursor", "Cursor Key"}
			calls := 0
			password, err := cursorSafeStoragePasswordUsing(false, func(service, account string, prompt bool) (string, error) {
				if prompt {
					t.Fatal("background read allowed a Keychain dialog")
				}
				if calls >= len(accounts) || account != accounts[calls] || service != "Cursor Safe Storage" {
					t.Fatalf("unexpected nonUI query: account=%q call=%d", account, calls)
				}
				calls++
				if tc.found == "blocked" {
					return "", errOSPermission
				}
				if account == tc.found {
					return "fixture-password", nil
				}
				return "", errKeychainMiss
			})
			if calls != tc.wantCalls || err != tc.err {
				t.Fatalf("calls=%d err=%v want calls=%d err=%v", calls, err, tc.wantCalls, tc.err)
			}
			if tc.err == nil && password != "fixture-password" {
				t.Fatal("lost fixture password")
			}
		})
	}
}
