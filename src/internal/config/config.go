package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/automazeio/envapor/internal/crypto"
	"gopkg.in/yaml.v3"
)

// Dir returns the Envapor configuration directory, honouring ENVAPOR_HOME and
// XDG_CONFIG_HOME, and falling back to ~/.config/envapor (or %APPDATA%\envapor
// on Windows).
func Dir() (string, error) {
	if v := os.Getenv("ENVAPOR_HOME"); v != "" {
		return v, nil
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "envapor"), nil
	}
	if runtime.GOOS == "windows" {
		if v := os.Getenv("APPDATA"); v != "" {
			return filepath.Join(v, "envapor"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("envapor: locating home directory: %w", err)
	}
	return filepath.Join(home, ".config", "envapor"), nil
}

// KeysDir returns the directory that holds PEM key files.
func KeysDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "keys"), nil
}

// KeyPath returns the on-disk path for a named key.
func KeyPath(name string) (string, error) {
	if err := validateKeyName(name); err != nil {
		return "", err
	}
	dir, err := KeysDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

func validateKeyName(name string) error {
	if name == "" || name == "." || name == ".." ||
		filepath.IsAbs(name) || filepath.Base(name) != name ||
		strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("envapor: invalid key name %q", name)
	}
	return nil
}

func configPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// RepoConfig is the per-repository mapping entry stored in config.yaml.
type RepoConfig struct {
	Key string `yaml:"key"`
}

// Config is the local, never-committed mapping of repositories to key names.
type Config struct {
	Repos map[string]RepoConfig `yaml:"repos"`
}

// Load reads config.yaml, returning an empty config if it does not yet exist.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{Repos: map[string]RepoConfig{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("envapor: reading config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("envapor: parsing config: %w", err)
	}
	if c.Repos == nil {
		c.Repos = map[string]RepoConfig{}
	}
	return &c, nil
}

// Save writes the config back to disk with owner-only permissions.
func (c *Config) Save() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("envapor: creating config dir: %w", err)
	}
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("envapor: encoding config: %w", err)
	}
	if err := atomicWrite(path, data, 0o600); err != nil {
		return fmt.Errorf("envapor: writing config: %w", err)
	}
	return nil
}

// SetRepoKey records that a repository (identified by its remote URL) uses the
// given key name, and persists the change.
func (c *Config) SetRepoKey(remote, keyName string) error {
	if err := validateKeyName(keyName); err != nil {
		return err
	}
	if c.Repos == nil {
		c.Repos = map[string]RepoConfig{}
	}
	c.Repos[remote] = RepoConfig{Key: keyName}
	return c.Save()
}

// LookupRepoKey returns the key name mapped to a remote URL, if any.
func (c *Config) LookupRepoKey(remote string) (string, bool) {
	rc, ok := c.Repos[remote]
	return rc.Key, ok && rc.Key != ""
}

// WriteKey persists PEM-encoded key bytes under the given name and returns the
// path it was written to.
func WriteKey(name string, pemBytes []byte) (string, error) {
	dir, err := KeysDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("envapor: creating keys dir: %w", err)
	}
	path, err := KeyPath(name)
	if err != nil {
		return "", err
	}
	if err := atomicWrite(path, pemBytes, 0o600); err != nil {
		return "", fmt.Errorf("envapor: writing key: %w", err)
	}
	return path, nil
}

// LoadKey reads and parses a named key from the keys directory.
func LoadKey(name string) (*crypto.Key, error) {
	path, err := KeyPath(name)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("envapor: reading key %q: %w", name, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("envapor: refusing symlink key %q", name)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("envapor: key %q has unsafe permissions %o (want 600)", name, info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("envapor: reading key %q: %w", name, err)
	}
	return crypto.LoadPEM(data)
}

// SecureKey enforces owner-only permissions on an existing named key.
func SecureKey(name string) error {
	path, err := KeyPath(name)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("envapor: securing key %q: %w", name, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("envapor: refusing symlink key %q", name)
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("envapor: securing key %q: %w", name, err)
	}
	return nil
}

// KeyExists reports whether a named key file is present.
func KeyExists(name string) bool {
	path, err := KeyPath(name)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) (err error) {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".envapor-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if f != nil {
			_ = f.Close()
		}
		if err != nil {
			_ = os.Remove(tmp)
		}
	}()
	if err = f.Chmod(mode); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	f = nil
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}
