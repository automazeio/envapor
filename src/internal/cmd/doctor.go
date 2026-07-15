package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/automazeio/envapor/internal/config"
	"github.com/automazeio/envapor/internal/gitutil"
	"github.com/spf13/cobra"
)

type status int

const (
	statusOK status = iota
	statusWarn
	statusFail
)

type checkResult struct {
	name   string
	status status
	detail string
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose the envapor setup in this repository",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		results := runChecks()
		failed := false
		for _, r := range results {
			switch r.status {
			case statusOK:
				fmt.Printf("  [ok]   %s", r.name)
			case statusWarn:
				fmt.Printf("  [warn] %s", r.name)
			case statusFail:
				fmt.Printf("  [fail] %s", r.name)
				failed = true
			}
			if r.detail != "" {
				fmt.Printf(": %s", r.detail)
			}
			fmt.Println()
		}
		if failed {
			return errors.New("doctor found problems")
		}
		return nil
	},
}

func runChecks() []checkResult {
	var out []checkResult

	if !gitutil.InsideRepo() {
		return []checkResult{{"git repository", statusFail, "not inside a git work tree"}}
	}
	out = append(out, checkResult{"git repository", statusOK, ""})

	if gitutil.FiltersConfigured() {
		out = append(out, checkResult{"filters installed", statusOK, ""})
	} else {
		out = append(out, checkResult{"filters installed", statusFail, "run 'envapor init'"})
	}

	if gitutil.HookInstalled() {
		out = append(out, checkResult{"pre-commit hook", statusOK, ""})
	} else {
		out = append(out, checkResult{"pre-commit hook", statusFail, "run 'envapor init'"})
	}

	name, err := resolveKeyName()
	if err != nil {
		out = append(out, checkResult{"repository mapping", statusFail, err.Error()})
		return out
	}
	out = append(out, checkResult{"repository mapping", statusOK, "key " + name})

	key, err := config.LoadKey(name)
	if err != nil {
		out = append(out, checkResult{"key available", statusFail, err.Error()})
		return out
	}
	out = append(out, checkResult{"key available", statusOK, ""})

	out = append(out, checkAttr("managed .env", ".env", "envapor"))
	for _, ex := range gitutil.DefaultExclusions {
		out = append(out, checkExclusion(ex))
	}

	files, _ := gitutil.ManagedFiles(gitutil.DefaultExclusions)
	if len(files) == 0 {
		out = append(out, checkResult{"managed files", statusOK, "none present yet"})
	} else {
		out = append(out, checkResult{"managed files", statusOK, strings.Join(files, ", ")})
	}
	if ignored, _ := gitutil.IgnoredManagedFiles(gitutil.DefaultExclusions); len(ignored) > 0 {
		out = append(out, checkResult{"ignored files", statusWarn, strings.Join(ignored, ", ") + " (remove from .gitignore)"})
	}

	token, err := key.Encrypt([]byte("envapor-doctor"))
	if err != nil {
		out = append(out, checkResult{"encryption", statusFail, err.Error()})
		return out
	}
	out = append(out, checkResult{"encryption", statusOK, ""})
	plain, err := key.Decrypt(token)
	if err != nil || string(plain) != "envapor-doctor" {
		out = append(out, checkResult{"decryption", statusFail, "round trip failed"})
	} else {
		out = append(out, checkResult{"decryption", statusOK, ""})
	}
	return out
}

func checkAttr(label, path, want string) checkResult {
	got, err := gitutil.CheckAttr("filter", path)
	if err != nil {
		return checkResult{label, statusFail, err.Error()}
	}
	if got != want {
		return checkResult{label, statusFail, fmt.Sprintf("filter=%s, want %s", got, want)}
	}
	return checkResult{label, statusOK, ""}
}

// checkExclusion verifies an example file is not routed through the filter.
func checkExclusion(path string) checkResult {
	got, err := gitutil.CheckAttr("filter", path)
	if err != nil {
		return checkResult{"exclude " + path, statusFail, err.Error()}
	}
	if got == "envapor" {
		return checkResult{"exclude " + path, statusFail, "would be encrypted"}
	}
	return checkResult{"exclude " + path, statusOK, ""}
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
