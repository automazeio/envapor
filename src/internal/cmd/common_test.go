package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/automazeio/envapor/internal/config"
	"github.com/automazeio/envapor/internal/crypto"
)

func TestResolveKeyArg(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("ENVAPOR_HOME", filepath.Join(root, "config"))
	key, err := crypto.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.WriteKey("team", key.MarshalPEM()); err != nil {
		t.Fatal(err)
	}

	stored, err := config.KeyPath("team")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := resolveKeyArg("team"); err != nil || got != stored {
		t.Fatalf("resolveKeyArg(name) = %q, %v; want %q", got, err, stored)
	}

	pem := filepath.Join(root, "other.pem")
	if err := os.WriteFile(pem, key.MarshalPEM(), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := resolveKeyArg(pem); err != nil || got != pem {
		t.Fatalf("resolveKeyArg(path) = %q, %v; want %q", got, err, pem)
	}

	// A local file with the same name as a stored key: the stored key wins,
	// and the ./ prefix forces the file interpretation.
	local := filepath.Join(root, "team")
	if err := os.WriteFile(local, key.MarshalPEM(), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := resolveKeyArg("team"); got != stored {
		t.Fatalf("resolveKeyArg(ambiguous) = %q, want stored key %q", got, stored)
	}
	if got, err := resolveKeyArg("./team"); err != nil || got != "./team" {
		t.Fatalf("resolveKeyArg(./team) = %q, %v; want ./team", got, err)
	}

	if err := os.Mkdir(filepath.Join(root, "demo"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveKeyArg("demo"); err == nil {
		t.Fatal("resolveKeyArg accepted a directory")
	}

	if _, err := resolveKeyArg("no-such-key"); err == nil {
		t.Fatal("resolveKeyArg accepted a nonexistent argument")
	}
}
