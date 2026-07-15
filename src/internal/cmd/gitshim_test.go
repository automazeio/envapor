package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallGitShim(t *testing.T) {
	dir := t.TempDir()
	gitShimDir = dir
	gitShimForce = false
	t.Cleanup(func() { gitShimDir = ""; gitShimForce = false })

	if err := installGitShimCmd.RunE(installGitShimCmd, nil); err != nil {
		t.Fatalf("install-git-shim: %v", err)
	}
	target := filepath.Join(dir, shimName())
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("shim not created: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("shim is not a symlink: mode %v", info.Mode())
	}
	// Resolves to a real, executable file (the running test binary).
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("shim does not resolve: %v", err)
	}

	// Idempotent: re-running replaces the existing symlink without --force.
	if err := installGitShimCmd.RunE(installGitShimCmd, nil); err != nil {
		t.Fatalf("re-install over existing shim: %v", err)
	}
}

func TestInstallGitShimRefusesForeignFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shim is a plain copy on Windows")
	}
	dir := t.TempDir()
	gitShimDir = dir
	gitShimForce = false
	t.Cleanup(func() { gitShimDir = ""; gitShimForce = false })

	target := filepath.Join(dir, shimName())
	if err := os.WriteFile(target, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installGitShimCmd.RunE(installGitShimCmd, nil); err == nil {
		t.Fatal("expected refusal to overwrite a non-symlink without --force")
	}

	gitShimForce = true
	if err := installGitShimCmd.RunE(installGitShimCmd, nil); err != nil {
		t.Fatalf("--force replace failed: %v", err)
	}
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("--force did not replace the file with a symlink")
	}
}
