package cursor

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const authAccessTokenKey = "cursorAuth/accessToken"
const authRefreshTokenKey = "cursorAuth/refreshToken"

var errNoLocalAuth = errors.New("未找到本机登录态")

func readItem(db *sql.DB, key string) (string, error) {
	var v sql.NullString
	err := db.QueryRow(`SELECT value FROM ItemTable WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such table") {
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
