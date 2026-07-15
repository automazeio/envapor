package cmd

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/automazeio/envapor/internal/config"
	"github.com/automazeio/envapor/internal/crypto"
	"github.com/automazeio/envapor/internal/gitutil"
)

// errNotInitialized is returned when a command needs a key mapping the current
// repository does not have.
var errNotInitialized = errors.New("repository is not initialized for envapor (run: envapor init --pem <key>)")

// resolveKeyName determines the key name for the current repository, preferring
// the local git config (which works even before a remote exists) and falling
// back to the global remote-URL mapping.
func resolveKeyName() (string, error) {
	if name := gitutil.ConfigGet("envapor.key"); name != "" {
		return name, nil
	}
	remote, err := gitutil.RemoteURL()
	if err == nil && remote != "" {
		cfg, err := config.Load()
		if err != nil {
			return "", err
		}
		if name, ok := cfg.LookupRepoKey(remote); ok {
			return name, nil
		}
	}
	return "", errNotInitialized
}

// loadRepoKey loads the encryption key mapped to the current repository.
func loadRepoKey() (*crypto.Key, error) {
	name, err := resolveKeyName()
	if err != nil {
		return nil, err
	}
	return config.LoadKey(name)
}

// selfPath returns the absolute, symlink-resolved path to the running binary,
// used when writing Git filter and hook commands.
func selfPath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	return p, nil
}

// keyNameFromPath derives a key name from a PEM file path, dropping a trailing
// ".pem" extension.
func keyNameFromPath(path string) string {
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext == ".pem" {
		return base[:len(base)-len(ext)]
	}
	return base
}
