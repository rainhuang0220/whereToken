package trae

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestByteCryptoRoundTrip(t *testing.T) {
	plain := []byte(`{"token":"eyJtest.payload.sig","userRegion":{"region":"CN"}}`)
	raw, err := encryptBlob(plain)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decryptBlob(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(plain) {
		t.Fatalf("got %s", got)
	}
}

func TestDecryptUserInfoFromStorageBlob(t *testing.T) {
	u := traeUserInfo{Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.aaa.bbb", Host: "https://api.trae.cn"}
	u.UserRegion.Region = "CN"
	body, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := encryptBlob(body)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := decryptUserInfo(base64.StdEncoding.EncodeToString(raw))
	if !ok || got.Token != u.Token || got.UserRegion.Region != "CN" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

func TestDecryptUserInfoRejectsGarbage(t *testing.T) {
	if _, ok := decryptUserInfo("dGMFEAAAfixture-encrypted-blob"); ok {
		t.Fatal("garbage accepted")
	}
	if _, ok := decryptUserInfo("not-base64!!!"); ok {
		t.Fatal("accepted")
	}
}

func TestDecryptUserInfoDoesNotLookLikeSecretInErrors(t *testing.T) {
	_, ok := decryptUserInfo("dGMFEAAA" + strings.Repeat("A", 80))
	if ok {
		t.Fatal("should fail")
	}
}
