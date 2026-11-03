package cursor

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rainhuang0220/whereToken/internal/adapter"
)

// Failure kinds are stable machine-readable reasons. The text has no
// session material. "auth" is intentionally absent: publicprofile maps
// that substring to a missing login.
const (
	KindOSPermission         = "OS_PERMISSION_REQUIRED"
	KindSealedCredential     = "SEALED_CREDENTIAL"
	KindTokenExpired         = "TOKEN_EXPIRED"
	KindTokenRejected        = "TOKEN_REJECTED"
	KindEndpointUnavailable  = "ENDPOINT_UNAVAILABLE"
	KindRateLimited          = "RATE_LIMITED"
	KindSchemaChanged        = "RESPONSE_SCHEMA_CHANGED"
	KindPaginationIncomplete = "PAGINATION_INCOMPLETE"
	KindAccountMismatch      = "ACCOUNT_MISMATCH"
	KindUnboundHistory       = "UNBOUND_HISTORY"
)

// AccountSubjectMeta is the ledger meta key for the Cursor account that
// owns observations on this install. The value is the JWT subject.
const AccountSubjectMeta = "cursor_account_subject"

type usageKind string

func (k usageKind) Error() string {
	return "账号用量不可用：" + string(k)
}

var (
	errOSPermission         = usageKind(KindOSPermission)
	errSealedCredential     = usageKind(KindSealedCredential)
	errTokenRejected        = usageKind(KindTokenRejected)
	errEndpointUnavailable  = usageKind(KindEndpointUnavailable)
	errRateLimited          = usageKind(KindRateLimited)
	errSchemaChanged        = usageKind(KindSchemaChanged)
	errPaginationIncomplete = usageKind(KindPaginationIncomplete)
	errAccountMismatch      = usageKind(KindAccountMismatch)
	errUnboundHistory       = usageKind(KindUnboundHistory)
	errUsagePageLimit       = usageKind("PAGE_LIMIT")
)

// AccountMismatchError is returned when this install's Cursor subject
// changes. The text does not include the subject.
func AccountMismatchError() error { return errAccountMismatch }

// UnboundHistoryError is returned when Cursor observations already exist
// and this install has not recorded an account subject. The first account
// seen is not written onto that history. The text does not include the subject.
func UnboundHistoryError() error { return errUnboundHistory }

// CoverageWindow is one UTC date range whose filtered pages completed.
// From and To are inclusive calendar days.
type CoverageWindow struct {
	From, To string
	Audit    WindowAudit
}

// WindowAudit is the desensitized count check for one filtered window.
// Validation "ok" is a closed window whose raw rows matched the API total.
// Validation "open" is the same check for a window that ends on the current
// UTC day; that day is not a finished calendar day. Validation "missing"
// is not stored. It is what a lookup returns when this process has no audit.
type WindowAudit struct {
	From, To                             string
	Declared, RawRows, Deduped, Days     int
	Miss, CacheRead, CacheCreate, Output int64
	Validation                           string
}

var (
	coverageMu   sync.Mutex
	coverageByDB = map[string][]CoverageWindow{}
	lookbackByDB = map[string]CoverageWindow{}
)

func setCoverage(dbPath string, windows []CoverageWindow) {
	coverageMu.Lock()
	defer coverageMu.Unlock()
	if len(windows) == 0 {
		delete(coverageByDB, dbPath)
		return
	}
	coverageByDB[dbPath] = append([]CoverageWindow(nil), windows...)
}

func setLookback(dbPath string, w CoverageWindow) {
	coverageMu.Lock()
	defer coverageMu.Unlock()
	lookbackByDB[dbPath] = w
}

func clearCoverage(dbPath string) {
	coverageMu.Lock()
	defer coverageMu.Unlock()
	delete(coverageByDB, dbPath)
	delete(lookbackByDB, dbPath)
}

// TakeAccountCoverage returns the windows recorded by the last parse of
// dbPath and clears them. An empty result means this parse did not
// confirm a filtered window.
func TakeAccountCoverage(dbPath string) []CoverageWindow {
	coverageMu.Lock()
	defer coverageMu.Unlock()
	w := coverageByDB[dbPath]
	delete(coverageByDB, dbPath)
	if len(w) == 0 {
		return nil
	}
	return append([]CoverageWindow(nil), w...)
}

// TakeLookback returns the UTC span this parse was willing to request,
// including months left on the ledger, and clears it.
func TakeLookback(dbPath string) (CoverageWindow, bool) {
	coverageMu.Lock()
	defer coverageMu.Unlock()
	w, ok := lookbackByDB[dbPath]
	delete(lookbackByDB, dbPath)
	if !ok || w.From == "" || w.To == "" || w.From > w.To {
		return CoverageWindow{}, false
	}
	return w, true
}

type connectFile struct {
	Version    int  `json:"version"`
	OSKeychain bool `json:"os_keychain"`
}

// ConsentPath is the local file that records an explicit connect.
// It stores the choice only, never a token or a keychain secret.
// WHERETOKEN_CURSOR_CONNECT overrides it outside tests.
func ConsentPath(home adapter.Home) string {
	if flag.Lookup("test.v") == nil {
		if v := strings.TrimSpace(os.Getenv("WHERETOKEN_CURSOR_CONNECT")); v != "" {
			return v
		}
	}
	if home == nil {
		return ""
	}
	return filepath.Join(home.XDGConfig("wheretoken"), "cursor-connect.json")
}

// ConsentOn reports whether path is an explicit cursor connect choice.
func ConsentOn(path string) bool { return consentAllows(path) }

func consentAllows(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var f connectFile
	if json.Unmarshal(raw, &f) != nil {
		return false
	}
	return f.Version == 1 && f.OSKeychain
}

// WriteConsent records that the user ran cursor connect. The file mode
// is owner-read/write. The directory is owner-only.
func WriteConsent(path string) error {
	if path == "" {
		return errOSPermission
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(connectFile{Version: 1, OSKeychain: true})
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o600)
}

// RemoveConsent deletes the connect choice. It does not change an
// access-control list or a platform secret.
func RemoveConsent(path string) error {
	if path == "" {
		return nil
	}
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// openSealed returns a bearer JWT. Without an explicit connect file the
// sealed value is left unread, which is the zero-config behavior.
// Tests never call the operating system unless Unseal is injected.
func (a Adapter) openSealed(value, dbPath string) (string, error) {
	value = normalizeStoredSession(value)
	if value == "" || !strings.HasPrefix(value, sealedStoragePrefix) {
		return value, nil
	}
	if a.Unseal != nil {
		return a.Unseal(value)
	}
	if plain, ok := rememberedOpened(dbPath); ok {
		return plain, nil
	}
	if !consentAllows(a.ConsentPath) {
		return "", errAccountUsageUnavailable
	}
	if flag.Lookup("test.v") != nil {
		return "", errAccountUsageUnavailable
	}
	plain, err := unsealOS(value, dbPath, a.AllowPrompt)
	if err != nil {
		return "", err
	}
	rememberOpened(dbPath, plain)
	return plain, nil
}

// rememberedOpened keeps a sealed session opened earlier in this process.
// cursor connect reads the subject once, then the same command collects
// usage without a second system dialog. The value is not written to the
// ledger, a log, or an error.
var (
	openedMu    sync.Mutex
	openedPlain = map[string]string{}
)

func rememberedOpened(dbPath string) (string, bool) {
	if dbPath == "" {
		return "", false
	}
	openedMu.Lock()
	defer openedMu.Unlock()
	plain, ok := openedPlain[dbPath]
	return plain, ok && plain != ""
}

func rememberOpened(dbPath, plain string) {
	if dbPath == "" || plain == "" {
		return
	}
	openedMu.Lock()
	openedPlain[dbPath] = plain
	openedMu.Unlock()
}

// sealedBlob returns the v10 ciphertext including its three-byte prefix.
// The Chromium OSCrypt layout is the one Electron safeStorage uses.
// CodexBar (MIT, Copyright 2026 Peter Steinberger) and CCBar (MIT,
// Copyright 2026 nanvon) document the same cursor-safe-storage:v1:
// envelope. This file is an independent implementation.
func sealedBlob(value string) ([]byte, bool) {
	value = normalizeStoredSession(value)
	const prefix = "cursor-safe-storage:v1:"
	if !strings.HasPrefix(value, prefix) {
		return nil, false
	}
	enc := value[len(prefix):]
	blob, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		blob, err = base64.RawStdEncoding.DecodeString(enc)
		if err != nil {
			return nil, false
		}
	}
	if len(blob) < 3 || string(blob[:3]) != "v10" {
		return nil, false
	}
	return blob, true
}

func deriveChromeKey(password string) []byte {
	return pbkdf2HMACSHA1([]byte(password), []byte("saltysalt"), 1003, 16)
}

func decryptChromeCBC(blob, key []byte) (string, error) {
	token, err := decryptChromeCBCRaw(blob, key)
	if err != nil || !usableSessionToken(token) {
		return "", errSealedCredential
	}
	return token, nil
}

func decryptChromeCBCRaw(blob, key []byte) (string, error) {
	if len(blob) < 3+aes.BlockSize || string(blob[:3]) != "v10" || len(key) != 16 {
		return "", errSealedCredential
	}
	payload := blob[3:]
	if len(payload)%aes.BlockSize != 0 {
		return "", errSealedCredential
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", errSealedCredential
	}
	iv := bytesRepeatSpace(aes.BlockSize)
	plain := make([]byte, len(payload))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, payload)
	plain, err = pkcs7Unpad(plain)
	if err != nil {
		return "", errSealedCredential
	}
	return strings.TrimSpace(string(plain)), nil
}

func decryptChromeGCM(blob, key []byte) (string, error) {
	const nonceLen = 12
	if len(blob) < 3+nonceLen+16 || string(blob[:3]) != "v10" || len(key) == 0 {
		return "", errSealedCredential
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", errSealedCredential
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", errSealedCredential
	}
	plain, err := gcm.Open(nil, blob[3:3+nonceLen], blob[3+nonceLen:], nil)
	if err != nil {
		return "", errSealedCredential
	}
	token := strings.TrimSpace(string(plain))
	if !usableSessionToken(token) {
		return "", errSealedCredential
	}
	return token, nil
}

func bytesRepeatSpace(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return b
}

func pkcs7Unpad(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, errSealedCredential
	}
	n := int(b[len(b)-1])
	if n == 0 || n > aes.BlockSize || n > len(b) {
		return nil, errSealedCredential
	}
	for _, c := range b[len(b)-n:] {
		if int(c) != n {
			return nil, errSealedCredential
		}
	}
	return b[:len(b)-n], nil
}

func pbkdf2HMACSHA1(password, salt []byte, iter, keyLen int) []byte {
	if iter < 1 || keyLen < 1 {
		return nil
	}
	const hLen = 20
	blocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, blocks*hLen)
	for i := 1; i <= blocks; i++ {
		out = append(out, pbkdf2Block(password, salt, iter, i)...)
	}
	return out[:keyLen]
}

func pbkdf2Block(password, salt []byte, iter, index int) []byte {
	mac := hmac.New(sha1.New, password)
	mac.Write(salt)
	var idx [4]byte
	binary.BigEndian.PutUint32(idx[:], uint32(index))
	mac.Write(idx[:])
	u := mac.Sum(nil)
	t := append([]byte(nil), u...)
	for n := 1; n < iter; n++ {
		mac = hmac.New(sha1.New, password)
		mac.Write(u)
		u = mac.Sum(nil)
		for i := range t {
			t[i] ^= u[i]
		}
	}
	return t
}
