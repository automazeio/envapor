package cmd

import (
	"fmt"

	"github.com/automazeio/envapor/internal/config"
	"github.com/automazeio/envapor/internal/gitutil"
	"github.com/spf13/cobra"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "List stored encryption keys",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		names, err := config.ListKeys()
		if err != nil {
			return err
		}
		if len(names) == 0 {
			dir, err := config.KeysDir()
			if err != nil {
				return err
			}
			fmt.Printf("No keys found in %s (create one with 'envapor keygen NAME').\n", dir)
			return nil
		}
		active := ""
		if gitutil.InsideRepo() {
			if name, err := resolveKeyName(); err == nil {
				active = name
			}
		}
		for _, name := range names {
			if name == active {
				fmt.Printf("%s  (this repository)\n", name)
			} else {
				fmt.Println(name)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(keysCmd)
}
