package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/automazeio/envapor/internal/config"
	"github.com/automazeio/envapor/internal/crypto"
	"github.com/automazeio/envapor/internal/envfile"
	"github.com/automazeio/envapor/internal/gitutil"
	"github.com/spf13/cobra"
)

var (
	initPem   string
	initForce bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up envapor in the current repository",
	Args:  cobra.NoArgs,
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	if !gitutil.InsideRepo() {
		return fmt.Errorf("not a git repository (run 'git init' first)")
	}
	root, err := gitutil.Root()
	if err != nil {
		return err
	}

	pemData, err := os.ReadFile(initPem)
	if err != nil {
		return fmt.Errorf("reading key file: %w", err)
	}
	if _, err := crypto.LoadPEM(pemData); err != nil {
		return err
	}

	name := keyNameFromPath(initPem)
	if err := importKey(name, initPem, pemData); err != nil {
		return err
	}

	if err := gitutil.ConfigSet("envapor.key", name); err != nil {
		return err
	}
	if remote, _ := gitutil.RemoteURL(); remote != "" {
		if cfg, err := config.Load(); err == nil {
			_ = cfg.SetRepoKey(remote, name)
		}
	}

	exe, err := selfPath()
	if err != nil {
		return err
	}
	if err := gitutil.ConfigureFilters(exe); err != nil {
		return err
	}
	if err := gitutil.EnsureAttributes(root, gitutil.DefaultExclusions); err != nil {
		return err
	}
	if err := gitutil.InstallHook(exe, initForce); err != nil {
		return err
	}

	files, err := gitutil.ManagedFiles(gitutil.DefaultExclusions)
	if err != nil {
		return err
	}
	if err := gitutil.Stage(files...); err != nil {
		return err
	}

	key, err := loadRepoKey()
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}
	defer key.Destroy()
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("refreshing %s: %w", path, err)
		}
		plain, err := envfile.Decrypt(content, key)
		if err != nil {
			return fmt.Errorf("refreshing %s through smudge filter: %w", path, err)
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("refreshing %s: %w", path, err)
		}
		if err := replaceFile(path, plain, info.Mode().Perm()); err != nil {
			return fmt.Errorf("refreshing %s: %w", path, err)
		}
	}

	fmt.Println("Envapor initialized.")
	fmt.Printf("  key:      %s\n", name)
	if len(files) == 0 {
		fmt.Println("  managed:  (no .env files found yet)")
	} else {
		fmt.Printf("  managed:  %s\n", strings.Join(files, ", "))
	}
	if ignored, _ := gitutil.IgnoredManagedFiles(gitutil.DefaultExclusions); len(ignored) > 0 {
		fmt.Printf("  warning:  %s is git-ignored and will not be committed; remove it from .gitignore\n", strings.Join(ignored, ", "))
	}
	fmt.Println("Run 'envapor doctor' to verify the full setup.")
	return nil
}

// importKey copies a PEM file into the central keys directory unless it already
// lives there under the same name.
func importKey(name, srcPath string, data []byte) error {
	dest, err := config.KeyPath(name)
	if err != nil {
		return err
	}
	if sameFile(srcPath, dest) {
		return config.SecureKey(name)
	}
	_, err = config.WriteKey(name, data)
	return err
}

func sameFile(a, b string) bool {
	ap, err1 := filepath.Abs(a)
	bp, err2 := filepath.Abs(b)
	return err1 == nil && err2 == nil && ap == bp
}

func init() {
	initCmd.Flags().StringVar(&initPem, "pem", "", "path to the PEM key file (required)")
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite an existing foreign pre-commit hook")
	_ = initCmd.MarkFlagRequired("pem")
	rootCmd.AddCommand(initCmd)
}
