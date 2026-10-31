package cursor

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/adapter"
)

const authAccessTokenKey = "cursorAuth/accessToken"
const authRefreshTokenKey = "cursorAuth/refreshToken"

const sealedStoragePrefix = "cursor-safe-storage:"

var errNoLocalAuth = errors.New("未找到本机登录态")

// errAccountUsageUnavailable is a sealed or otherwise unreadable session.
// publicprofile.tokenReason maps any "auth" substring, and the missing-login
// sentence, to a logged-out user. This text must stay free of both.
var errAccountUsageUnavailable = errors.New("账号用量不可用：Cursor 把会话封存在系统密钥中，本程序不读取该密钥")

// normalizeStoredSession unwraps one JSON string. ItemTable sometimes stores
// the session that way, quotes included. A raw JWT is unchanged.
func normalizeStoredSession(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' {
		var decoded string
		if json.Unmarshal([]byte(value), &decoded) == nil {
			value = strings.TrimSpace(decoded)
		}
	}
	return value
}

// usableSessionToken reports whether value can be sent as a bearer.
// Accepted shapes are a three-part JWT whose header is the standard
// base64url of "{", and that JWT after a WorkOS user id (anything::jwt).
// The safeStorage envelope and any other ciphertext are not: reading the
// OS secret is not the default install.
func usableSessionToken(value string) bool {
	value = normalizeStoredSession(value)
	if value == "" || strings.HasPrefix(value, sealedStoragePrefix) {
		return false
	}
	if i := strings.LastIndex(value, "::"); i >= 0 {
		value = strings.TrimSpace(value[i+2:])
	}
	parts := strings.Split(value, ".")
	if len(parts) != 3 || !strings.HasPrefix(parts[0], "eyJ") {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
	}
	return true
}

func readItem(db *sql.DB, key string) (string, error) {
	var v sql.NullString
	err := db.QueryRow(`SELECT value FROM ItemTable WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		if !adapter.HasTable(db, "ItemTable") {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(v.String), nil
}

func readStorageJSONToken(vscdbPath string) string {
	p := filepath.Join(filepath.Dir(vscdbPath), "storage.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil {
		return ""
	}
	v, _ := obj[authAccessTokenKey].(string)
	return strings.TrimSpace(v)
}
