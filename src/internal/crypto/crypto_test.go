package crypto

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestEncryptRoundTrip(t *testing.T) {
	k, err := Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	cases := [][]byte{
		[]byte("sk_live_abc123"),
		[]byte(""),
		[]byte("postgres://user:pass@host:5432/db?sslmode=require"),
		[]byte("value with spaces and # hash"),
		bytes.Repeat([]byte("x"), 4096),
	}
	for _, pt := range cases {
		token, err := k.Encrypt(pt)
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", pt, err)
		}
		if !IsEncrypted(token) {
			t.Fatalf("IsEncrypted(%q) = false", token)
		}
		got, err := k.Decrypt(token)
		if err != nil {
			t.Fatalf("Decrypt(%q): %v", token, err)
		}
		if !bytes.Equal(got, pt) {
			t.Fatalf("round trip mismatch: got %q want %q", got, pt)
		}
	}
}

func TestEncryptDeterministic(t *testing.T) {
	k, _ := Generate()
	a, _ := k.Encrypt([]byte("same-value"))
	b, _ := k.Encrypt([]byte("same-value"))
	if a != b {
		t.Fatalf("expected deterministic ciphertext, got %q and %q", a, b)
	}
	c, _ := k.Encrypt([]byte("other-value"))
	if a == c {
		t.Fatal("distinct plaintext produced identical ciphertext")
	}
}

func TestContextBinding(t *testing.T) {
	k, _ := Generate()
	token, err := k.EncryptContext([]byte("same-value"), "DB_PASSWORD")
	if err != nil {
		t.Fatal(err)
	}
	if !IsEncrypted(token) {
		t.Fatalf("context token is not recognized: %q", token)
	}
	if _, err := k.DecryptContext(token, "OTHER_KEY"); err == nil {
		t.Fatal("expected authentication failure in a different context")
	}
	got, err := k.DecryptContext(token, "DB_PASSWORD")
	if err != nil || string(got) != "same-value" {
		t.Fatalf("DecryptContext() = %q, %v", got, err)
	}
	other, _ := k.EncryptContext([]byte("same-value"), "OTHER_KEY")
	if token == other {
		t.Fatal("different contexts produced identical tokens")
	}
}

func TestPEMRoundTrip(t *testing.T) {
	k, _ := Generate()
	loaded, err := LoadPEM(k.MarshalPEM())
	if err != nil {
		t.Fatalf("LoadPEM: %v", err)
	}
	token, _ := k.Encrypt([]byte("secret"))
	got, err := loaded.Decrypt(token)
	if err != nil {
		t.Fatalf("Decrypt with reloaded key: %v", err)
	}
	if string(got) != "secret" {
		t.Fatalf("got %q want %q", got, "secret")
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	k1, _ := Generate()
	k2, _ := Generate()
	token, _ := k1.Encrypt([]byte("secret"))
	if _, err := k2.Decrypt(token); err == nil {
		t.Fatal("expected authentication failure with wrong key")
	}
}

func TestIsEncrypted(t *testing.T) {
	validKey, _ := Generate()
	valid, _ := validKey.Encrypt([]byte("secret"))
	if !IsEncrypted(valid) {
		t.Fatalf("IsEncrypted(%q) = false, want true", valid)
	}

	tooShort := base64.StdEncoding.EncodeToString(make([]byte, nonceSize))
	for _, s := range []string{
		"plaintext",
		"ENC[]",
		"ENC[v2:abc]",
		"ENC[abc]",
		"ENC[v1:not-base64]",
		"ENC[v1:" + tooShort + "]",
	} {
		if IsEncrypted(s) {
			t.Errorf("IsEncrypted(%q) = true, want false", s)
		}
	}
}
