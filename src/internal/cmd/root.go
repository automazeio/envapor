package cmd

import (
	"fmt"
	"os"

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
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "envapor: "+err.Error())
		os.Exit(1)
	}
}
