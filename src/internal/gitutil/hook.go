package gitutil

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

const hookMarker = "# envapor pre-commit guard"

// HooksDir returns the absolute path to the repository's hooks directory,
// honouring a custom core.hooksPath.
func HooksDir() (string, error) {
	p, err := run("rev-parse", "--git-path", "hooks")
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(p) {
		return p, nil
	}
	return filepath.Abs(p)
}

func hookScript(exe string) string {
	return "#!/bin/sh\n" + hookMarker + "\n" + shellQuote(exe) + " hook pre-commit\n"
}

// InstallHook writes the pre-commit guard. If a foreign pre-commit hook already
// exists it is preserved and an error is returned unless force is set.
func InstallHook(exe string, force bool) error {
	dir, err := HooksDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "pre-commit")
	backup := path + ".envapor-backup"

	if existing, err := os.ReadFile(path); err == nil {
		if containsMarker(existing) {
			if err := os.WriteFile(path, []byte(hookScript(exe)), 0o755); err != nil {
				return err
			}
			return os.Chmod(path, 0o755)
		}
		if !force {
			return fmt.Errorf("a pre-commit hook already exists at %s; re-run with --force to replace it", path)
		}
		if _, err := os.Stat(backup); err == nil {
			return fmt.Errorf("cannot preserve existing hook: backup already exists at %s", backup)
		}
		if err := os.Rename(path, backup); err != nil {
			return fmt.Errorf("envapor: backing up existing hook: %w", err)
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("envapor: creating hooks dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(hookScript(exe)), 0o755); err != nil {
		if _, backupErr := os.Stat(backup); backupErr == nil {
			_ = os.Rename(backup, path)
		}
		return err
	}
	return os.Chmod(path, 0o755)
}

// HookInstalled reports whether the Envapor pre-commit guard is installed.
func HookInstalled() bool {
	dir, err := HooksDir()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(dir, "pre-commit"))
	return err == nil && containsMarker(data)
}

func containsMarker(data []byte) bool {
	return bytes.Contains(data, []byte(hookMarker))
}
