package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/automazeio/envapor/internal/envfile"
	"github.com/automazeio/envapor/internal/gitutil"
	"github.com/spf13/cobra"
)

var hookCmd = &cobra.Command{
	Use:    "hook",
	Short:  "Internal Git hook entry points",
	Hidden: true,
}

var (
	listStagedFiles = gitutil.StagedFiles
	readStagedFile  = gitutil.ShowStaged
)

// hookPreCommitCmd is the safety net invoked by the pre-commit guard: it
// inspects the staged (post-clean) content of managed files and aborts the
// commit if any non-PUBLIC value reached the index as plaintext, which happens
// when the clean filter did not run.
var hookPreCommitCmd = &cobra.Command{
	Use:  "pre-commit",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		staged, err := listStagedFiles()
		if err != nil {
			return err
		}
		var problems []string
		var readErrors []string
		for _, f := range staged {
			if !gitutil.IsManagedName(f, gitutil.DefaultExclusions) {
				continue
			}
			content, err := readStagedFile(f)
			if err != nil {
				readErrors = append(readErrors, fmt.Sprintf("  %s: %v", f, err))
				continue
			}
			for _, v := range envfile.Verify(content) {
				problems = append(problems, fmt.Sprintf("  %s: %s", f, v.Key))
			}
		}
		if len(readErrors) > 0 {
			fmt.Fprintln(os.Stderr, "envapor: refusing to commit because staged secrets could not be verified:")
			for _, problem := range readErrors {
				fmt.Fprintln(os.Stderr, problem)
			}
			return errors.New("pre-commit guard could not verify staged secrets")
		}
		if len(problems) > 0 {
			fmt.Fprintln(os.Stderr, "envapor: refusing to commit plaintext secrets:")
			for _, p := range problems {
				fmt.Fprintln(os.Stderr, p)
			}
			fmt.Fprintln(os.Stderr, "\nThe clean filter may not be installed. Run 'envapor doctor' to diagnose.")
			return errors.New("pre-commit guard blocked plaintext secrets")
		}
		return nil
	},
}

func init() {
	hookCmd.AddCommand(hookPreCommitCmd)
	rootCmd.AddCommand(hookCmd)
}
