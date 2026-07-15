package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestKeyPathRejectsUnsafeNames(t *testing.T) {
	t.Setenv("ENVAPOR_HOME", t.TempDir())
	for _, name := range []string{"", ".", "..", "../outside", "nested/key", `nested\key`, "/absolute"} {
		if _, err := KeyPath(name); err == nil {
			t.Errorf("KeyPath(%q) succeeded, want error", name)
		}
	}
	if _, err := KeyPath("team-prod"); err != nil {
		t.Fatalf("KeyPath(valid): %v", err)
	}
}

func TestWriteKeyCorrectsPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not enforced on Windows")
	}
	t.Setenv("ENVAPOR_HOME", t.TempDir())
	path, err := KeyPath("team")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteKey("team", []byte("new")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("key mode = %o, want 600", got)
	}
}

func TestSaveCorrectsPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not enforced on Windows")
	}
	home := t.TempDir()
	t.Setenv("ENVAPOR_HOME", home)
	path := filepath.Join(home, "config.yaml")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("repos: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&Config{Repos: map[string]RepoConfig{}}).Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %o, want 600", got)
	}
}
