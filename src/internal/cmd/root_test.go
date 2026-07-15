package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestGitInvocation(t *testing.T) {
	// Note: filepath.Base is OS-aware, so a backslash Windows path can only be
	// exercised on Windows; here we use forward-slash paths plus a .exe name.
	cases := map[string]bool{
		"/usr/local/bin/git-envapor": true,
		"git-envapor":                true,
		"git-envapor.exe":            true,
		"/usr/local/bin/envapor":     false,
		"envapor":                    false,
		"/opt/bin/git-envapor-extra": false,
	}
	for argv0, want := range cases {
		if _, ok := gitInvocation(argv0); ok != want {
			t.Errorf("gitInvocation(%q) = %v, want %v", argv0, ok, want)
		}
	}
	if display, _ := gitInvocation("/x/git-envapor"); display != "git envapor" {
		t.Errorf("display name = %q, want %q", display, "git envapor")
	}
}

// TestGitDisplayNameInCommandPath verifies the display-name annotation makes
// subcommand help read "git envapor <cmd>" while command matching (Name) is
// unchanged.
func TestGitDisplayNameInCommandPath(t *testing.T) {
	root := &cobra.Command{Use: "envapor"}
	sub := &cobra.Command{Use: "status"}
	root.AddCommand(sub)
	root.Annotations = map[string]string{cobra.CommandDisplayNameAnnotation: "git envapor"}

	if got := sub.CommandPath(); !strings.HasPrefix(got, "git envapor status") {
		t.Fatalf("CommandPath = %q, want it to start with %q", got, "git envapor status")
	}
	if sub.Name() != "status" {
		t.Fatalf("sub.Name() = %q, want status", sub.Name())
	}
}
