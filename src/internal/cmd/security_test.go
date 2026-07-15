package cmd

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/automazeio/envapor/internal/crypto"
	"github.com/automazeio/envapor/internal/envfile"
)

func git(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)
	git(t, "init", "-q")
	git(t, "config", "user.name", "Envapor Test")
	git(t, "config", "user.email", "envapor@example.invalid")
	return root
}

func TestPreCommitBlocksStagedReadErrors(t *testing.T) {
	root := initTestRepo(t)
	path := filepath.Join(root, ".env")
	if err := os.WriteFile(path, []byte("SECRET=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, "add", ".env")
	original := readStagedFile
	readStagedFile = func(string) ([]byte, error) {
		return nil, errors.New("synthetic staged read failure")
	}
	defer func() { readStagedFile = original }()

	if err := hookPreCommitCmd.RunE(hookPreCommitCmd, nil); err == nil {
		t.Fatal("pre-commit guard succeeded despite unreadable staged content")
	}
}

func TestPreCommitAllowsStagedDeletion(t *testing.T) {
	root := initTestRepo(t)
	path := filepath.Join(root, ".env")
	if err := os.WriteFile(path, []byte("SECRET=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, "add", ".env")
	git(t, "commit", "-qm", "base")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	git(t, "add", "--", ".env")

	if err := hookPreCommitCmd.RunE(hookPreCommitCmd, nil); err != nil {
		t.Fatalf("pre-commit guard blocked staged deletion: %v", err)
	}
}

func TestMigratePreflightLeavesFilesUnchanged(t *testing.T) {
	root := initTestRepo(t)
	t.Setenv("ENVAPOR_HOME", filepath.Join(root, "config"))
	oldKey, _ := crypto.Generate()
	newKey, _ := crypto.Generate()
	oldPEM := filepath.Join(root, "old.pem")
	newPEM := filepath.Join(root, "new.pem")
	if err := os.WriteFile(oldPEM, oldKey.MarshalPEM(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPEM, newKey.MarshalPEM(), 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := envfile.Encrypt([]byte("A=secret\n"), oldKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), first, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	wrongToken, err := newKey.EncryptContext([]byte("wrong-key"), "B")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", ".env.prod"), []byte("B="+wrongToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err = runMigrate(oldPEM, newPEM)
	if err == nil {
		t.Fatal("runMigrate succeeded with invalid later file")
	}
	got, readErr := os.ReadFile(filepath.Join(root, ".env"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, first) {
		t.Fatalf("first file changed despite preflight failure:\n got %q\nwant %q", got, first)
	}
}
