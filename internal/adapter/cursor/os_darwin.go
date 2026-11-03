//go:build darwin

package cursor

import (
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
)

// unsealOS reads the Cursor Safe Storage password through Security.framework
// in this process. The keychain dialog, when it appears, names this binary.
// Always Allow then matches this binary's designated requirement.
// An ad-hoc signature changes on every rebuild, so an unsigned upgrade asks
// again. This file does not exec /usr/bin/security and does not edit ACLs:
// granting that helper would let every local process read the item.
func unsealOS(token, _ string, prompt bool) (string, error) {
	blob, ok := sealedBlob(token)
	if !ok {
		return "", errSealedCredential
	}
	password, err := cursorSafeStoragePassword(prompt)
	if err != nil {
		return "", err
	}
	plain, err := decryptChromeCBC(blob, deriveChromeKey(password))
	if err != nil {
		return "", errSealedCredential
	}
	return plain, nil
}

func cursorSafeStoragePassword(prompt bool) (string, error) {
	// Cursor 3.27 builds have used both "Cursor" and "Cursor Key".
	// Try every name without UI first. At most one dialog follows.
	var blocked error
	for _, account := range []string{"", "Cursor", "Cursor Key"} {
		password, err := readGenericPassword("Cursor Safe Storage", account, false)
		if err == nil {
			return password, nil
		}
		if err == errKeychainMiss {
			continue
		}
		blocked = err
		break
	}
	if blocked == nil {
		return "", errSealedCredential
	}
	if !prompt {
		return "", blocked
	}
	password, err := readGenericPassword("Cursor Safe Storage", "", true)
	if err != nil {
		return "", err
	}
	return password, nil
}

var errKeychainMiss = usageKind("KEYCHAIN_MISS")

const (
	errSecSuccess               int32  = 0
	errSecUserCanceled          int32  = -128
	errSecNoAccessForItem       int32  = -25243
	errSecAuthFailed            int32  = -25293
	errSecItemNotFound          int32  = -25300
	errSecInteractionNotAllowed int32  = -25308
	cfStringEncodingUTF8        uint32 = 0x08000100
)

func readGenericPassword(service, account string, prompt bool) (string, error) {
	sec, err := purego.Dlopen("/System/Library/Frameworks/Security.framework/Security", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return "", errSealedCredential
	}
	cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return "", errSealedCredential
	}

	var (
		secItemCopyMatching func(query uintptr, result *uintptr) int32
		cfStringCreate      func(alloc uintptr, cstr *byte, encoding uint32) uintptr
		cfDictionaryCreate  func(alloc uintptr, keys *uintptr, values *uintptr, n int64, keyCB, valueCB uintptr) uintptr
		cfDataGetLength     func(data uintptr) int64
		cfDataGetBytePtr    func(data uintptr) uintptr
		cfRelease           func(uintptr)
	)
	purego.RegisterLibFunc(&secItemCopyMatching, sec, "SecItemCopyMatching")
	purego.RegisterLibFunc(&cfStringCreate, cf, "CFStringCreateWithCString")
	purego.RegisterLibFunc(&cfDictionaryCreate, cf, "CFDictionaryCreate")
	purego.RegisterLibFunc(&cfDataGetLength, cf, "CFDataGetLength")
	purego.RegisterLibFunc(&cfDataGetBytePtr, cf, "CFDataGetBytePtr")
	purego.RegisterLibFunc(&cfRelease, cf, "CFRelease")

	kClass, err := cfSymbol(sec, "kSecClass")
	if err != nil {
		return "", errSealedCredential
	}
	kClassGeneric, err := cfSymbol(sec, "kSecClassGenericPassword")
	if err != nil {
		return "", errSealedCredential
	}
	kService, err := cfSymbol(sec, "kSecAttrService")
	if err != nil {
		return "", errSealedCredential
	}
	kAccount, err := cfSymbol(sec, "kSecAttrAccount")
	if err != nil {
		return "", errSealedCredential
	}
	kReturn, err := cfSymbol(sec, "kSecReturnData")
	if err != nil {
		return "", errSealedCredential
	}
	kLimit, err := cfSymbol(sec, "kSecMatchLimit")
	if err != nil {
		return "", errSealedCredential
	}
	kLimitOne, err := cfSymbol(sec, "kSecMatchLimitOne")
	if err != nil {
		return "", errSealedCredential
	}
	kTrue, err := cfSymbol(cf, "kCFBooleanTrue")
	if err != nil {
		return "", errSealedCredential
	}
	keyCB, err := purego.Dlsym(cf, "kCFTypeDictionaryKeyCallBacks")
	if err != nil {
		return "", errSealedCredential
	}
	valueCB, err := purego.Dlsym(cf, "kCFTypeDictionaryValueCallBacks")
	if err != nil {
		return "", errSealedCredential
	}

	serviceBuf := append([]byte(service), 0)
	serviceRef := cfStringCreate(0, &serviceBuf[0], cfStringEncodingUTF8)
	runtime.KeepAlive(serviceBuf)
	if serviceRef == 0 {
		return "", errSealedCredential
	}
	defer cfRelease(serviceRef)

	var accountRef uintptr
	var accountBuf []byte
	if account != "" {
		accountBuf = append([]byte(account), 0)
		accountRef = cfStringCreate(0, &accountBuf[0], cfStringEncodingUTF8)
		runtime.KeepAlive(accountBuf)
		if accountRef == 0 {
			return "", errSealedCredential
		}
		defer cfRelease(accountRef)
	}

	keys := []uintptr{kClass, kService, kReturn, kLimit}
	vals := []uintptr{kClassGeneric, serviceRef, kTrue, kLimitOne}
	if accountRef != 0 {
		keys = append(keys, kAccount)
		vals = append(vals, accountRef)
	}
	if !prompt {
		kUI, err := cfSymbol(sec, "kSecUseAuthenticationUI")
		if err != nil {
			return "", errSealedCredential
		}
		kFail, err := cfSymbol(sec, "kSecUseAuthenticationUIFail")
		if err != nil {
			return "", errSealedCredential
		}
		keys = append(keys, kUI)
		vals = append(vals, kFail)
	}
	dict := cfDictionaryCreate(0, &keys[0], &vals[0], int64(len(keys)), keyCB, valueCB)
	runtime.KeepAlive(keys)
	runtime.KeepAlive(vals)
	if dict == 0 {
		return "", errSealedCredential
	}
	defer cfRelease(dict)

	var result uintptr
	status := secItemCopyMatching(dict, &result)
	if status != errSecSuccess {
		return "", keychainStatus(status)
	}
	if result == 0 {
		return "", errSealedCredential
	}
	defer cfRelease(result)
	n := cfDataGetLength(result)
	if n <= 0 || n > 1<<20 {
		return "", errSealedCredential
	}
	ptr := cfDataGetBytePtr(result)
	if ptr == 0 {
		return "", errSealedCredential
	}
	// Copy out of the CFData before CFRelease. cPtr reinterprets the C
	// address; the copy is a Go string the collector can see.
	raw := unsafe.Slice((*byte)(cPtr(ptr)), int(n))
	password := string(append([]byte(nil), raw...))
	if password == "" {
		return "", errSealedCredential
	}
	return password, nil
}

func cfSymbol(handle uintptr, name string) (uintptr, error) {
	addr, err := purego.Dlsym(handle, name)
	if err != nil || addr == 0 {
		return 0, errSealedCredential
	}
	// kSec* symbols are pointer variables. Read the pointer they store.
	return *(*uintptr)(cPtr(addr)), nil
}

// cPtr reinterprets a C address as a pointer. Taking the address of the
// uintptr and loading it avoids a uintptr-to-Pointer conversion that
// go vet rejects. The address is a framework symbol or a CF allocation.
func cPtr(addr uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&addr))
}

func keychainStatus(status int32) error {
	switch status {
	case errSecItemNotFound:
		return errKeychainMiss
	case errSecInteractionNotAllowed, errSecUserCanceled, errSecAuthFailed, errSecNoAccessForItem:
		return errOSPermission
	default:
		if status == errSecSuccess {
			return nil
		}
		return errOSPermission
	}
}
