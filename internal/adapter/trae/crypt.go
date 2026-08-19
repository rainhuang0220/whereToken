package trae

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

// ByteDance iCube storage.json blobs (iCubeAuthInfo://…). Header tc\x05\x10\x00\x00
// then a 32-byte wrap key and AES-128-CBC of SHA-512(plain)||plain.
// Ported from Trae CN's out/vs/base/common/byteCrypto.js. Never log plaintext.

const (
	wrapKeyLen = 32
	headerLen  = 6
	hashLen    = 64
	aesKeyLen  = 16
	ivLen      = 16
)

var (
	hdrAES        = []byte{116, 99, 5, 16, 0, 0} // "tc" + AES-128
	hdrAESPrivate = []byte{18, 57, 32, 32, 2, 3}
	saltAES       = xor64(
		[]byte{82, 9, 106, 213, 48, 54, 165, 56, 191, 64, 163, 158, 129, 243, 215, 251, 124, 227, 57, 130, 155, 47, 255, 135, 52, 142, 67, 68, 196, 222, 233, 203, 84, 123, 148, 50, 166, 194, 35, 61, 238, 76, 149, 11, 66, 250, 195, 78, 8, 46, 161, 102, 40, 217, 36, 178, 118, 91, 162, 73, 109, 139, 209, 37},
		[]byte{31, 221, 168, 51, 136, 7, 199, 49, 177, 18, 16, 89, 39, 128, 236, 95, 96, 81, 127, 169, 25, 181, 74, 13, 45, 229, 122, 159, 147, 201, 156, 239, 160, 224, 59, 77, 174, 42, 245, 176, 200, 235, 187, 60, 131, 83, 153, 97, 23, 43, 4, 126, 186, 119, 214, 38, 225, 105, 20, 99, 85, 33, 12, 125},
	)
	saltPrivate = xor64(
		[]byte{191, 192, 216, 250, 122, 246, 220, 97, 31, 254, 98, 27, 8, 72, 71, 176, 135, 99, 96, 18, 127, 101, 203, 104, 211, 102, 191, 125, 37, 72, 150, 156, 51, 229, 121, 35, 17, 153, 141, 177, 110, 131, 150, 128, 172, 255, 254, 6, 18, 140, 55, 62, 236, 249, 135, 64, 135, 12, 117, 4, 89, 149, 168, 209},
		[]byte{246, 204, 26, 232, 232, 70, 129, 109, 223, 146, 169, 242, 23, 241, 105, 145, 50, 196, 165, 42, 254, 120, 3, 54, 244, 207, 209, 85, 53, 6, 138, 106, 175, 148, 31, 204, 186, 186, 165, 182, 87, 142, 49, 10, 39, 110, 26, 154, 86, 56, 173, 125, 18, 64, 198, 225, 99, 99, 83, 82, 191, 134, 76, 170},
	)
)

var errCrypto = errors.New("encrypted blob")

type traeUserInfo struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
	Host         string `json:"host"`
	UserRegion   struct {
		Region string `json:"region"`
	} `json:"userRegion"`
	Account struct {
		StoreRegion string `json:"storeRegion"`
		UserTag     string `json:"userTag"`
		Scope       string `json:"scope"`
	} `json:"account"`
}

func xor64(a, b []byte) []byte {
	out := make([]byte, 64)
	for i := 0; i < 64; i++ {
		out[i] = a[i] ^ b[i]
	}
	return out
}

func sha512Sum(p []byte) []byte {
	h := sha512.Sum512(p)
	return h[:]
}

func deriveAES(wrapKey, salt []byte) (key, iv []byte) {
	n := make([]byte, 0, hashLen+len(salt))
	n = append(n, sha512Sum(wrapKey)...)
	n = append(n, salt...)
	d := sha512Sum(n)
	return d[:aesKeyLen], d[aesKeyLen : aesKeyLen+ivLen]
}

func pkcs7Unpad(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, errCrypto
	}
	n := int(b[len(b)-1])
	if n == 0 || n > aes.BlockSize || n > len(b) {
		return nil, errCrypto
	}
	for i := 0; i < n; i++ {
		if b[len(b)-1-i] != byte(n) {
			return nil, errCrypto
		}
	}
	return b[:len(b)-n], nil
}

func pkcs7Pad(b []byte) []byte {
	n := aes.BlockSize - len(b)%aes.BlockSize
	out := make([]byte, len(b)+n)
	copy(out, b)
	for i := len(b); i < len(out); i++ {
		out[i] = byte(n)
	}
	return out
}

func decryptBlob(raw []byte) ([]byte, error) {
	if len(raw) < headerLen+wrapKeyLen+aes.BlockSize {
		return nil, errCrypto
	}
	var salt []byte
	switch {
	case bytesEqual(raw[:headerLen], hdrAES):
		salt = saltAES
	case bytesEqual(raw[:headerLen], hdrAESPrivate):
		salt = saltPrivate
	default:
		return nil, errCrypto
	}
	wrap := raw[headerLen : headerLen+wrapKeyLen]
	ct := raw[headerLen+wrapKeyLen:]
	if len(ct)%aes.BlockSize != 0 {
		return nil, errCrypto
	}
	key, iv := deriveAES(wrap, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errCrypto
	}
	plain := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, ct)
	plain, err = pkcs7Unpad(plain)
	if err != nil {
		return nil, err
	}
	if len(plain) < hashLen {
		return nil, errCrypto
	}
	body := plain[hashLen:]
	if !bytesEqual(plain[:hashLen], sha512Sum(body)) {
		return nil, errCrypto
	}
	return body, nil
}

func encryptBlob(plain []byte) ([]byte, error) {
	wrap := make([]byte, wrapKeyLen)
	if _, err := rand.Read(wrap); err != nil {
		return nil, err
	}
	key, iv := deriveAES(wrap, saltAES)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	inner := append(sha512Sum(plain), plain...)
	padded := pkcs7Pad(inner)
	ct := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, padded)
	out := make([]byte, 0, headerLen+wrapKeyLen+len(ct))
	out = append(out, hdrAES...)
	out = append(out, wrap...)
	out = append(out, ct...)
	return out, nil
}

func decryptUserInfo(b64 string) (traeUserInfo, bool) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil || len(raw) == 0 {
		return traeUserInfo{}, false
	}
	body, err := decryptBlob(raw)
	if err != nil {
		return traeUserInfo{}, false
	}
	var u traeUserInfo
	if json.Unmarshal(body, &u) != nil {
		return traeUserInfo{}, false
	}
	u.Token = strings.TrimSpace(u.Token)
	if u.Token == "" {
		return traeUserInfo{}, false
	}
	return u, true
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
