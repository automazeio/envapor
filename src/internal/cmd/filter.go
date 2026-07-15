package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/automazeio/envapor/internal/envfile"
	"github.com/spf13/cobra"
)

// smudgeFallback reports a smudge failure and returns the content unchanged,
// so the surrounding git operation (pull, checkout, clone) succeeds with the
// file left encrypted instead of aborting halfway through. Clean has no such
// fallback: failures there stay fatal so plaintext can never reach the object
// store unencrypted.
func smudgeFallback(path string, cause error, content []byte) []byte {
	if path == "" {
		path = "the file"
	}
	reason := strings.TrimPrefix(cause.Error(), "envapor: ")
	fmt.Fprintf(os.Stderr, "envapor: warning: could not decrypt %s: %s\n", path, reason)
	fmt.Fprintf(os.Stderr, "envapor: the git operation itself succeeded, but %s still holds encrypted values.\n", path)
	fmt.Fprintf(os.Stderr, "envapor: set the right key with 'envapor init <key>', then run 'envapor decrypt'.\n")
	return content
}

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
// writes the decrypted form to the working tree on checkout. When decryption is
// impossible (missing or wrong key), it passes the encrypted content through
// with a warning rather than failing the whole git operation.
var smudgeCmd = &cobra.Command{
	Use:    "smudge [file]",
	Short:  "Git smudge filter (decrypt stdin to stdout)",
	Hidden: true,
	Args:   cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := ".env"
		if len(args) == 1 {
			path = args[0]
		}
		in, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		key, err := loadRepoKey()
		if err != nil {
			_, werr := os.Stdout.Write(smudgeFallback(path, err, in))
			return werr
		}
		defer key.Destroy()
		out, err := envfile.Decrypt(in, key)
		if err != nil {
			out = smudgeFallback(path, err, in)
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
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		key, err := loadRepoKey()
		if err != nil {
			_, werr := os.Stdout.Write(smudgeFallback(args[0], err, data))
			return werr
		}
		defer key.Destroy()
		out, err := envfile.Decrypt(data, key)
		if err != nil {
			out = smudgeFallback(args[0], err, data)
		}
		_, err = os.Stdout.Write(out)
		return err
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd, smudgeCmd, textconvCmd)
}
