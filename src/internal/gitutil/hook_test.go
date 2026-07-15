package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInstallHookForcePreservesExistingHook(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if out, err := exec.Command("git", "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	dir, err := HooksDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "pre-commit")
	foreign := []byte("#!/bin/sh\necho foreign\n")
	if err := os.WriteFile(path, foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := InstallHook("/tmp/envapor", true); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".envapor-backup")
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != string(foreign) {
		t.Fatalf("backup = %q, want %q", backup, foreign)
	}
}

func TestSnapshotAndRestoreIndex(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	runGit := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "-q")
	runGit("config", "user.name", "Envapor Test")
	runGit("config", "user.email", "envapor@example.invalid")
	path := filepath.Join(root, ".env")
	if err := os.WriteFile(path, []byte("A=one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".env")
	runGit("commit", "-qm", "base")
	if err := os.WriteFile(path, []byte("A=two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".env")
	snapshot, err := SnapshotIndex(".env")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("A=three\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".env")
	if err := RestoreIndex(snapshot); err != nil {
		t.Fatal(err)
	}
	got, err := ShowStaged(".env")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "A=two\n" {
		t.Fatalf("restored index = %q, want staged partial content", got)
	}
}
