package syncagg

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const HashKeySize = 32

func SourceKeyHash(userKey []byte, tool string, scope SourceScope, stableLocalID string) (string, error) {
	if len(userKey) != HashKeySize {
		return "", fmt.Errorf("source hmac key must be %d bytes", HashKeySize)
	}
	if tool == "" || stableLocalID == "" {
		return "", fmt.Errorf("source key preimage is incomplete")
	}
	if _, err := ParseScope(string(scope)); err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, userKey)
	mac.Write([]byte("wheretoken/v1/"))
	mac.Write([]byte(tool))
	mac.Write([]byte("/"))
	mac.Write([]byte(scope))
	mac.Write([]byte("/"))
	mac.Write([]byte(stableLocalID))
	return hex.EncodeToString(mac.Sum(nil)), nil
}
