package cmd

import (
	"io"
	"os"

	"github.com/automazeio/envapor/internal/envfile"
	"github.com/spf13/cobra"
)

// cleanCmd is the Git clean filter: it reads a plaintext .env from stdin and
// writes the encrypted form to stdout on the way into the object store.
var cleanCmd = &cobra.Command{
	Use:    "clean [file]",
	Short:  "Git clean filter (encrypt stdin to stdout)",
	Hidden: true,
	Args:   cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := loadRepoKey()
		if err != nil {
			return err
		}
		in, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		out, err := envfile.Encrypt(in, key)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(out)
		return err
	},
}

// smudgeCmd is the Git smudge filter: it reads an encrypted .env from stdin and
// writes the decrypted form to the working tree on checkout.
var smudgeCmd = &cobra.Command{
	Use:    "smudge [file]",
	Short:  "Git smudge filter (decrypt stdin to stdout)",
	Hidden: true,
	Args:   cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := loadRepoKey()
		if err != nil {
			return err
		}
		in, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		out, err := envfile.Decrypt(in, key)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(out)
		return err
	},
}

// textconvCmd backs `diff=envapor`: Git passes a path to the (encrypted) blob
// and expects decrypted text on stdout so diffs read as plaintext.
var textconvCmd = &cobra.Command{
	Use:    "textconv FILE",
	Short:  "Git diff textconv (decrypt a file to stdout)",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := loadRepoKey()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		out, err := envfile.Decrypt(data, key)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(out)
		return err
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd, smudgeCmd, textconvCmd)
}
