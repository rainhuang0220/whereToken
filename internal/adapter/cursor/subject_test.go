package cursor

import (
	"encoding/base64"
	"testing"
)

func TestJWTSubjectDoesNotReturnToken(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"cursor-user-99","email":"a@b.c"}`))
	tok := "eyJhbGciOiJub25lIn0." + payload + ".sig"
	if got := JWTSubject(tok); got != "cursor-user-99" {
		t.Fatalf("got %q", got)
	}
	if JWTSubject(tok) == tok {
		t.Fatal("returned raw token")
	}
	if JWTSubject("not-a-jwt") != "" {
		t.Fatal("garbage")
	}
}
