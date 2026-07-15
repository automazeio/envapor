package envfile

import (
	"strings"
	"testing"

	"github.com/automazeio/envapor/internal/crypto"
)

func mustKey(t *testing.T) *crypto.Key {
	t.Helper()
	k, err := crypto.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return k
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	k := mustKey(t)
	inputs := []string{
		"DATABASE_URL=postgres://user:pass@host/db\nSTRIPE_KEY=sk_live_x\n",
		"# a comment\n\nexport TOKEN=abc\nQUOTED=\"has # hash and spaces\"\n",
		"NO_TRAILING_NEWLINE=value",
		"WIN=value\r\nOTHER=thing\r\n",
	}
	for _, in := range inputs {
		enc, err := Encrypt([]byte(in), k)
		if err != nil {
			t.Fatalf("Encrypt: %v", err)
		}
		dec, err := Decrypt(enc, k)
		if err != nil {
			t.Fatalf("Decrypt: %v", err)
		}
		if string(dec) != in {
			t.Fatalf("round trip mismatch:\n got %q\nwant %q", dec, in)
		}
	}
}

func TestPublicValuesStayPlaintext(t *testing.T) {
	k := mustKey(t)
	in := "APP_ENV=production # PUBLIC\n" +
		"API_URL=https://api.example.com # PUBLIC: browser endpoint\n" +
		"LOG_LEVEL=debug # PUBLIC - safe\n" +
		"SECRET=hunter2\n"
	enc, err := Encrypt([]byte(in), k)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got := string(enc)
	for _, want := range []string{
		"APP_ENV=production # PUBLIC",
		"API_URL=https://api.example.com # PUBLIC: browser endpoint",
		"LOG_LEVEL=debug # PUBLIC - safe",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("public line not preserved: %q\nfull:\n%s", want, got)
		}
	}
	if strings.Contains(got, "hunter2") {
		t.Errorf("secret value leaked as plaintext:\n%s", got)
	}
}

func TestFailsClosedOnAmbiguousMarker(t *testing.T) {
	k := mustKey(t)
	// "PUBLICLY" is not the PUBLIC marker, so the value must be encrypted.
	in := "TOKEN=secret # PUBLICLY known fact\n"
	enc, _ := Encrypt([]byte(in), k)
	if strings.Contains(string(enc), "secret") {
		t.Errorf("value should have been encrypted (fail closed):\n%s", enc)
	}
}

func TestKeysRemainReadable(t *testing.T) {
	k := mustKey(t)
	in := "DATABASE_URL=postgres://x\nSTRIPE_KEY=sk_live_x\n"
	enc, _ := Encrypt([]byte(in), k)
	got := string(enc)
	if !strings.Contains(got, "DATABASE_URL=ENC[") || !strings.Contains(got, "STRIPE_KEY=ENC[") {
		t.Errorf("expected readable keys with ENC values, got:\n%s", got)
	}
}

func TestEncryptIsIdempotent(t *testing.T) {
	k := mustKey(t)
	in := "SECRET=value\n"
	once, _ := Encrypt([]byte(in), k)
	twice, _ := Encrypt(once, k)
	if string(once) != string(twice) {
		t.Errorf("encrypt not idempotent:\n once: %s\ntwice: %s", once, twice)
	}
}

func TestVerify(t *testing.T) {
	k := mustKey(t)
	plain := []byte("A=secret\nB=public # PUBLIC\nC=other\n")
	v := Verify(plain)
	if len(v) != 2 {
		t.Fatalf("expected 2 violations, got %d: %+v", len(v), v)
	}
	if v[0].Key != "A" || v[1].Key != "C" {
		t.Errorf("unexpected violations: %+v", v)
	}
	enc, _ := Encrypt(plain, k)
	if got := Verify(enc); len(got) != 0 {
		t.Errorf("encrypted content should have no violations, got %+v", got)
	}
}

func TestBOMPrefixedAssignmentIsProtected(t *testing.T) {
	k := mustKey(t)
	in := []byte("\xef\xbb\xbfSECRET=hunter2\r\n")

	if got := Verify(in); len(got) != 1 || got[0].Key != "SECRET" {
		t.Fatalf("Verify() = %+v, want SECRET violation", got)
	}
	enc, err := Encrypt(in, k)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if strings.Contains(string(enc), "hunter2") {
		t.Fatalf("BOM-prefixed secret leaked: %q", enc)
	}
	dec, err := Decrypt(enc, k)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(dec) != string(in) {
		t.Fatalf("round trip mismatch: got %q want %q", dec, in)
	}
}

func TestMultilineQuotedValueIsProtected(t *testing.T) {
	k := mustKey(t)
	in := []byte("MULTI=\"line1\nline2 secret\"\nNEXT=value\n")

	if got := Verify(in); len(got) != 2 || got[0].Key != "MULTI" || got[1].Key != "NEXT" {
		t.Fatalf("Verify() = %+v, want MULTI and NEXT violations", got)
	}
	enc, err := Encrypt(in, k)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if strings.Contains(string(enc), "line2 secret") {
		t.Fatalf("multiline secret leaked: %q", enc)
	}
	dec, err := Decrypt(enc, k)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(dec) != string(in) {
		t.Fatalf("round trip mismatch: got %q want %q", dec, in)
	}
}

func TestMalformedTokenIsNotAcceptedAsEncrypted(t *testing.T) {
	in := []byte("SECRET=ENC[v1:not-base64]\n")
	if got := Verify(in); len(got) != 1 || got[0].Key != "SECRET" {
		t.Fatalf("Verify() = %+v, want SECRET violation", got)
	}
}

func TestUnparseableContentFailsClosed(t *testing.T) {
	k := mustKey(t)
	in := []byte("SECRET without equals\n")
	if _, err := Encrypt(in, k); err == nil {
		t.Fatal("Encrypt succeeded for unparseable content")
	}
	got := Verify(in)
	if len(got) != 1 || got[0].Key != "<unparseable>" {
		t.Fatalf("Verify() = %+v, want unparseable violation", got)
	}
}
