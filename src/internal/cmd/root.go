package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// version is the Envapor release, overridable at build time via -ldflags.
var version = "dev"

var rootCmd = &cobra.Command{
	Use:           "envapor",
	Short:         "Commit your secrets, securely.",
	Long:          "Envapor transparently encrypts .env values in Git: encrypted on commit, restored on checkout, with nothing extra to manage.",
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command and maps any error to a non-zero exit code.
func Execute() {
	if display, ok := gitInvocation(os.Args[0]); ok {
		// Invoked as the `git envapor` shim: show the Git subcommand form in
		// help and usage without changing how commands are matched.
		if rootCmd.Annotations == nil {
			rootCmd.Annotations = map[string]string{}
		}
		rootCmd.Annotations[cobra.CommandDisplayNameAnnotation] = display
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "envapor: "+err.Error())
		os.Exit(1)
	}
}

// gitInvocation reports whether the binary was run as Git's `git-envapor` shim
// (that is, via `git envapor`), returning the display name to use in help.
func gitInvocation(argv0 string) (string, bool) {
	base := strings.TrimSuffix(filepath.Base(argv0), ".exe")
	if base == "git-envapor" {
		return "git envapor", true
	}
	return "", false
}
