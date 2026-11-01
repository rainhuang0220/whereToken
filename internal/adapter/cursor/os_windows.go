//go:build windows

package cursor

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// unsealOS uses CryptUnprotectData on the Cursor Local State key, then
// AES-GCM. That is the user-account DPAPI secret Electron already stored.
// It does not add an ACL entry. The caller has already required cursor connect.
func unsealOS(token, dbPath string, _ bool) (string, error) {
	blob, ok := sealedBlob(token)
	if !ok {
		return "", errSealedCredential
	}
	key, err := dpapiChromeKey(dbPath)
	if err != nil {
		return "", err
	}
	plain, err := decryptChromeGCM(blob, key)
	if err != nil {
		return "", errSealedCredential
	}
	return plain, nil
}

func dpapiChromeKey(dbPath string) ([]byte, error) {
	path := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(dbPath))), "Local State")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, errSealedCredential
	}
	var obj struct {
		OSCrypt struct {
			EncryptedKey string `json:"encrypted_key"`
		} `json:"os_crypt"`
	}
	if json.Unmarshal(raw, &obj) != nil || obj.OSCrypt.EncryptedKey == "" {
		return nil, errSealedCredential
	}
	enc, err := base64.StdEncoding.DecodeString(obj.OSCrypt.EncryptedKey)
	if err != nil || len(enc) <= 5 || string(enc[:5]) != "DPAPI" {
		return nil, errSealedCredential
	}
	return cryptUnprotect(enc[5:])
}

func cryptUnprotect(in []byte) ([]byte, error) {
	if len(in) == 0 {
		return nil, errSealedCredential
	}
	inBlob := windows.DataBlob{Size: uint32(len(in)), Data: &in[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&inBlob, nil, nil, 0, nil, 0, &out); err != nil {
		return nil, errSealedCredential
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
	if out.Size == 0 || out.Data == nil {
		return nil, errSealedCredential
	}
	buf := unsafe.Slice(out.Data, out.Size)
	return append([]byte(nil), buf...), nil
}
