package cmd

import (
	"fmt"

	"github.com/automazeio/envapor/internal/config"
	"github.com/automazeio/envapor/internal/crypto"
	"github.com/spf13/cobra"
)

var keygenForce bool

var keygenCmd = &cobra.Command{
	Use:   "keygen NAME",
	Short: "Generate a new encryption key with safe parameters",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if config.KeyExists(name) && !keygenForce {
			return fmt.Errorf("key %q already exists (use --force to overwrite)", name)
		}
		key, err := crypto.Generate()
		if err != nil {
			return err
		}
		defer key.Destroy()
		path, err := config.WriteKey(name, key.MarshalPEM())
		if err != nil {
			return err
		}
		fmt.Printf("Created key %q at %s\n", name, path)
		return nil
	},
}

func init() {
	keygenCmd.Flags().BoolVar(&keygenForce, "force", false, "overwrite an existing key")
	rootCmd.AddCommand(keygenCmd)
}
