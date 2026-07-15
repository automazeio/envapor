package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/automazeio/envapor/internal/envfile"
	"github.com/automazeio/envapor/internal/gitutil"
	"github.com/spf13/cobra"
)

// verifyCmd exposes the pre-commit guard's check as a standalone command for CI
// and pre-push use. It inspects the index content of managed .env files (what
// Git actually stores), since the working tree is plaintext by design, and
// exits non-zero if any non-PUBLIC value is still plaintext.
var verifyCmd = &cobra.Command{
	Use:   "verify [file...]",
	Short: "Check that .env files stored in Git contain no plaintext secrets",
	Long: "Verifies that every managed .env file, as stored in Git's index, has all\n" +
		"non-PUBLIC values encrypted. The working tree is plaintext by design, so\n" +
		"verify inspects index content. Exits non-zero when plaintext is found,\n" +
		"which makes it suitable for CI and pre-push checks.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !gitutil.InsideRepo() {
			return errors.New("not a git repository")
		}

		files := args
		if len(files) == 0 {
			var err error
			files, err = gitutil.ManagedFiles(gitutil.DefaultExclusions)
			if err != nil {
				return err
			}
		}
		if len(files) == 0 {
			fmt.Println("No managed .env files found.")
			return nil
		}

		var problems []string
		var skipped []string
		checked := 0
		for _, f := range files {
			content, err := gitutil.ShowStaged(f)
			if err != nil {
				// Not in the index (e.g. untracked): nothing is committed to verify.
				skipped = append(skipped, f)
				continue
			}
			checked++
			for _, v := range envfile.Verify(content) {
				problems = append(problems, fmt.Sprintf("  %s:%d %s", f, v.Line, v.Key))
			}
		}

		if len(problems) > 0 {
			fmt.Fprintln(os.Stderr, "envapor: plaintext secrets found in git-stored .env files:")
			for _, p := range problems {
				fmt.Fprintln(os.Stderr, p)
			}
			fmt.Fprintln(os.Stderr, "\nThe clean filter may not have run when these were committed. Run 'envapor doctor'.")
			return errors.New("verification failed: plaintext secrets present")
		}

		for _, f := range skipped {
			fmt.Fprintf(os.Stderr, "envapor: skipped %s (not tracked in git)\n", f)
		}
		fmt.Printf("verified %d file(s): no plaintext secrets\n", checked)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}
