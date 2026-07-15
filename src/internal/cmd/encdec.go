package cmd

import (
	"fmt"
	"os"

	"github.com/automazeio/envapor/internal/crypto"
	"github.com/automazeio/envapor/internal/envfile"
	"github.com/automazeio/envapor/internal/gitutil"
	"github.com/spf13/cobra"
)

var encryptCmd = &cobra.Command{
	Use:   "encrypt [file...]",
	Short: "Encrypt managed .env files in place",
	RunE: func(cmd *cobra.Command, args []string) error {
		return transformFiles(args, func(content []byte, k *crypto.Key) ([]byte, error) {
			return envfile.Encrypt(content, k)
		})
	},
}

var decryptCmd = &cobra.Command{
	Use:   "decrypt [file...]",
	Short: "Decrypt managed .env files in place",
	RunE: func(cmd *cobra.Command, args []string) error {
		return transformFiles(args, func(content []byte, k *crypto.Key) ([]byte, error) {
			return envfile.Decrypt(content, k)
		})
	},
}

// transformFiles applies fn to each target file in place, defaulting to the
// repository's managed .env files when no explicit paths are given.
func transformFiles(args []string, fn func([]byte, *crypto.Key) ([]byte, error)) error {
	key, err := loadRepoKey()
	if err != nil {
		return err
	}
	files := args
	if len(files) == 0 {
		files, err = gitutil.ManagedFiles(gitutil.DefaultExclusions)
		if err != nil {
			return err
		}
	}
	if len(files) == 0 {
		fmt.Println("No managed .env files found.")
		return nil
	}
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		out, err := fn(content, key)
		if err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		if err := os.WriteFile(f, out, info.Mode().Perm()); err != nil {
			return err
		}
		fmt.Printf("Processed %s\n", f)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(encryptCmd, decryptCmd)
}
