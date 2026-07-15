package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var gitShimForce bool

// shimName is the file Git looks for to resolve `git envapor`: Git runs the
// executable `git-<name>` found on PATH, passing the remaining arguments.
func shimName() string {
	if runtime.GOOS == "windows" {
		return "git-envapor.exe"
	}
	return "git-envapor"
}

var installGitShimCmd = &cobra.Command{
	Use:   "install-git-shim",
	Short: "Install a git-envapor shim so `git envapor` works",
	Long: "Creates a git-envapor executable next to the envapor binary (or in\n" +
		"--dir) so Git resolves 'git envapor <command>' to envapor. On Unix this\n" +
		"is a symlink; on Windows it is a copy. The target directory must be on\n" +
		"your PATH, which the envapor binary's own directory already is.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locating envapor binary: %w", err)
		}
		exe, _ = filepath.Abs(exe)

		dir := gitShimDir
		if dir == "" {
			dir = filepath.Dir(exe)
		}
		target := filepath.Join(dir, shimName())

		if info, err := os.Lstat(target); err == nil {
			// A symlink (our Unix shim) or a copy of the envapor binary (our
			// Windows shim) is our own and safe to replace idempotently. Any
			// other file is foreign and needs --force so we never clobber an
			// unrelated executable that happens to share the name.
			ours := info.Mode()&os.ModeSymlink != 0 || isOwnShim(target, exe)
			if !ours && !gitShimForce {
				return fmt.Errorf("%s already exists and is not an envapor shim; re-run with --force to replace it", target)
			}
			if err := os.Remove(target); err != nil {
				return fmt.Errorf("removing existing shim: %w", err)
			}
		}

		if err := writeShim(exe, dir, target); err != nil {
			return err
		}
		fmt.Printf("Installed git shim at %s\n", target)
		fmt.Println("Try it: git envapor status")
		return nil
	},
}

// writeShim symlinks the shim to the envapor binary on Unix, or copies the
// binary on Windows where symlinks need extra privileges. The symlink is
// relative to the shim's directory so it survives the directory being moved.
func writeShim(exe, dir, target string) error {
	if runtime.GOOS != "windows" {
		rel, err := filepath.Rel(dir, exe)
		if err != nil {
			rel = exe
		}
		if err := os.Symlink(rel, target); err != nil {
			return fmt.Errorf("creating shim symlink: %w", err)
		}
		return nil
	}
	return copyFile(exe, target)
}

// isOwnShim reports whether the file at path is a byte-for-byte copy of the
// envapor binary, i.e. the shim we write on Windows. This lets re-installing
// over our own shim succeed without --force while a foreign file still needs it.
func isOwnShim(path, exe string) bool {
	pi, err := os.Stat(path)
	if err != nil {
		return false
	}
	ei, err := os.Stat(exe)
	if err != nil || pi.Size() != ei.Size() {
		return false
	}
	a, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	b, err := os.ReadFile(exe)
	if err != nil {
		return false
	}
	return bytes.Equal(a, b)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

var gitShimDir string

func init() {
	installGitShimCmd.Flags().StringVar(&gitShimDir, "dir", "", "directory to install the shim into (default: the envapor binary's directory)")
	installGitShimCmd.Flags().BoolVar(&gitShimForce, "force", false, "replace an existing git-envapor file")
	rootCmd.AddCommand(installGitShimCmd)
}
