package cmd

import (
	"testing"

	"github.com/automazeio/envapor/internal/config"
	"github.com/automazeio/envapor/internal/crypto"
	"github.com/automazeio/envapor/internal/gitutil"
)

func TestInitWithKeyName(t *testing.T) {
	initTestRepo(t)
	t.Setenv("ENVAPOR_HOME", t.TempDir())
	key, err := crypto.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.WriteKey("team", key.MarshalPEM()); err != nil {
		t.Fatal(err)
	}

	if err := runInit(initCmd, []string{"team"}); err != nil {
		t.Fatalf("init with key name failed: %v", err)
	}
	if got := gitutil.ConfigGet("envapor.key"); got != "team" {
		t.Fatalf("envapor.key = %q, want %q", got, "team")
	}
}

func TestInitWithUnknownKeyName(t *testing.T) {
	initTestRepo(t)
	t.Setenv("ENVAPOR_HOME", t.TempDir())

	if err := runInit(initCmd, []string{"missing"}); err == nil {
		t.Fatal("init succeeded with a key name that does not exist")
	}
}

func TestResolveInitPemRejectsNameAndPem(t *testing.T) {
	initPem = "some.pem"
	defer func() { initPem = "" }()

	if _, err := resolveInitPem([]string{"team"}); err == nil {
		t.Fatal("resolveInitPem accepted both a key name and --pem")
	}
}

func TestResolveInitPemRequiresKey(t *testing.T) {
	initPem = ""

	if _, err := resolveInitPem(nil); err == nil {
		t.Fatal("resolveInitPem accepted a call with neither key name nor --pem")
	}
}
