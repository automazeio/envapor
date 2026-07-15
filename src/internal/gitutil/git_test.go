package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setupRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.name", "Envapor Test"},
		{"config", "user.email", "envapor@example.invalid"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return root
}

func TestCheckAttrBatch(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte(AttributesBlock(DefaultExclusions)), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := CheckAttrBatch("filter", []string{".env", ".env.example"})
	if err != nil {
		t.Fatal(err)
	}
	if got[".env"] != "envapor" {
		t.Fatalf(".env filter = %q, want envapor", got[".env"])
	}
	if got[".env.example"] == "envapor" {
		t.Fatalf(".env.example should be excluded, got %q", got[".env.example"])
	}
}

func TestShowStagedBatch(t *testing.T) {
	root := setupRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env.prod"), []byte("B=2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "add", ".env").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	got, err := ShowStagedBatch([]string{".env", ".env.prod", ".env.missing"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got[".env"]) != "A=1\n" {
		t.Fatalf("staged .env = %q, want A=1", got[".env"])
	}
	if _, ok := got[".env.prod"]; ok {
		t.Fatal(".env.prod is not staged and must be absent")
	}
	if _, ok := got[".env.missing"]; ok {
		t.Fatal(".env.missing must be absent")
	}
}
