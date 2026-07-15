package cmd

import (
	"fmt"

	"github.com/automazeio/envapor/internal/envfile"
	"github.com/automazeio/envapor/internal/gitutil"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the envapor mapping and managed files",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !gitutil.InsideRepo() {
			return fmt.Errorf("not a git repository")
		}
		name, err := resolveKeyName()
		if err != nil {
			return err
		}
		fmt.Printf("key:      %s\n", name)
		if remote, _ := gitutil.RemoteURL(); remote != "" {
			fmt.Printf("remote:   %s\n", remote)
		}
		fmt.Printf("filters:  %s\n", installed(gitutil.FiltersConfigured()))
		fmt.Printf("hook:     %s\n", installed(gitutil.HookInstalled()))

		files, err := gitutil.ManagedFiles(gitutil.DefaultExclusions)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			fmt.Println("managed:  (none)")
			return nil
		}
		staged, err := gitutil.ShowStagedBatch(files)
		if err != nil {
			return err
		}
		fmt.Println("managed:")
		for _, f := range files {
			state := "plaintext (not yet committed)"
			if content, ok := staged[f]; ok {
				if len(envfile.Verify(content)) == 0 {
					state = "encrypted in index"
				} else {
					state = "PLAINTEXT in index"
				}
			}
			fmt.Printf("  %-24s %s\n", f, state)
		}
		return nil
	},
}

func installed(ok bool) string {
	if ok {
		return "installed"
	}
	return "missing"
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
