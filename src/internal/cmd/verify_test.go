package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/automazeio/envapor/internal/crypto"
	"github.com/automazeio/envapor/internal/envfile"
)

func TestVerifyPassesOnEncryptedIndex(t *testing.T) {
	root := initTestRepo(t)
	key, err := crypto.Generate()
	if err != nil {
		t.Fatal(err)
	}
	enc, err := envfile.Encrypt([]byte("SECRET=value\nPUB=safe # PUBLIC\n"), key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), enc, 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, "add", ".env")

	if err := verifyCmd.RunE(verifyCmd, nil); err != nil {
		t.Fatalf("verify failed on an encrypted index: %v", err)
	}
}

func TestVerifyFailsOnPlaintextIndex(t *testing.T) {
	root := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=value\nPUB=safe # PUBLIC\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, "add", ".env")

	if err := verifyCmd.RunE(verifyCmd, nil); err == nil {
		t.Fatal("verify passed despite a plaintext SECRET in the index")
	}
}
