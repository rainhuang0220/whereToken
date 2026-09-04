package syncagg

import (
	"encoding/hex"
	"strings"
	"testing"
)

func testKey(t *testing.T, seed byte) []byte {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = seed + byte(i)
	}
	return k
}

func TestSourceKeyHashStableAcrossDevices(t *testing.T) {
	t.Parallel()
	key := testKey(t, 1)
	a, err := SourceKeyHash(key, "cursor", ScopeAccountGlobal, "cursor-sub-abc")
	if err != nil {
		t.Fatal(err)
	}
	b, err := SourceKeyHash(key, "cursor", ScopeAccountGlobal, "cursor-sub-abc")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("same user key + same Cursor sub must match:\n%s\n%s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("hex length %d", len(a))
	}
	if _, err := hex.DecodeString(a); err != nil {
		t.Fatal(err)
	}
}

func TestSourceKeyHashDiffersByUserKey(t *testing.T) {
	t.Parallel()
	a, err := SourceKeyHash(testKey(t, 1), "cursor", ScopeAccountGlobal, "cursor-sub-abc")
	if err != nil {
		t.Fatal(err)
	}
	b, err := SourceKeyHash(testKey(t, 2), "cursor", ScopeAccountGlobal, "cursor-sub-abc")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("different whereToken users must not share source_key_hash")
	}
}

func TestSourceKeyHashDiffersByStableID(t *testing.T) {
	t.Parallel()
	key := testKey(t, 1)
	a, err := SourceKeyHash(key, "cursor", ScopeAccountGlobal, "acct-1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := SourceKeyHash(key, "cursor", ScopeAccountGlobal, "acct-2")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("different source identities collided")
	}
}

func TestSourceKeyHashRejectsWeakKeyAndEmptyID(t *testing.T) {
	t.Parallel()
	if _, err := SourceKeyHash([]byte("short"), "cursor", ScopeAccountGlobal, "x"); err == nil {
		t.Fatal("accepted short key")
	}
	if _, err := SourceKeyHash(testKey(t, 1), "cursor", ScopeAccountGlobal, ""); err == nil {
		t.Fatal("accepted empty stable id")
	}
}

func TestSourceKeyHashDoesNotEmbedEnumerableIdentity(t *testing.T) {
	t.Parallel()
	email := "rain@example.com"
	sum, err := SourceKeyHash(testKey(t, 9), "cursor", ScopeAccountGlobal, email)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(sum)
	if strings.Contains(lower, "rain") || strings.Contains(sum, email) || strings.Contains(lower, hex.EncodeToString([]byte(email))) {
		t.Fatalf("hash leaked identity preimage: %s", sum)
	}
}
