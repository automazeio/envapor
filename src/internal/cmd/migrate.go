package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/automazeio/envapor/internal/config"
	"github.com/automazeio/envapor/internal/crypto"
	"github.com/automazeio/envapor/internal/envfile"
	"github.com/automazeio/envapor/internal/gitutil"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate OLDPEM NEWPEM",
	Short: "Re-encrypt managed values from an old key to a new one",
	Long: "Re-encrypts the working tree and future commits from OLDPEM to NEWPEM.\n" +
		"It does not rewrite Git history: past commits stay encrypted under the old\n" +
		"key, so migration rotates the key, not the secrets themselves. After a\n" +
		"compromise, also rotate the affected secrets at their source.",
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMigrate(args[0], args[1])
	},
}

type migrationFile struct {
	path     string
	original []byte
	plain    []byte
	mode     os.FileMode
}

func runMigrate(oldPEM, newPEM string) (err error) {
	if !gitutil.InsideRepo() {
		return fmt.Errorf("not a git repository")
	}
	oldKey, err := loadPEMFile(oldPEM)
	if err != nil {
		return fmt.Errorf("old key: %w", err)
	}
	defer oldKey.Destroy()
	newData, err := os.ReadFile(newPEM)
	if err != nil {
		return fmt.Errorf("new key: %w", err)
	}
	if _, err := crypto.LoadPEM(newData); err != nil {
		return fmt.Errorf("new key: %w", err)
	}
	newName := keyNameFromPath(newPEM)
	if _, err := config.KeyPath(newName); err != nil {
		return err
	}

	files, err := gitutil.ManagedFiles(gitutil.DefaultExclusions)
	if err != nil {
		return err
	}
	prepared := make([]migrationFile, 0, len(files))
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		plain, err := envfile.Decrypt(content, oldKey)
		if err != nil {
			return fmt.Errorf("%s: decrypt with old key: %w", path, err)
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		prepared = append(prepared, migrationFile{
			path: path, original: content, plain: plain, mode: info.Mode().Perm(),
		})
	}

	oldName := gitutil.ConfigGet("envapor.key")
	indexEntries, err := gitutil.SnapshotIndex(files...)
	if err != nil {
		return err
	}
	changed := true
	defer func() {
		if err == nil || !changed {
			return
		}
		var rollbackErr error
		for _, file := range prepared {
			rollbackErr = errors.Join(rollbackErr, replaceFile(file.path, file.original, file.mode))
		}
		if oldName == "" {
			rollbackErr = errors.Join(rollbackErr, gitutil.ConfigUnset("envapor.key"))
		} else {
			rollbackErr = errors.Join(rollbackErr, gitutil.ConfigSet("envapor.key", oldName))
		}
		rollbackErr = errors.Join(rollbackErr, gitutil.RestoreIndex(indexEntries))
		if rollbackErr != nil {
			err = fmt.Errorf("%w (rollback failed: %v)", err, rollbackErr)
		}
	}()

	for _, file := range prepared {
		if err = replaceFile(file.path, file.plain, file.mode); err != nil {
			return err
		}
	}
	if err = importKey(newName, newPEM, newData); err != nil {
		return err
	}
	if err = gitutil.ConfigSet("envapor.key", newName); err != nil {
		return err
	}
	if err = gitutil.Stage(files...); err != nil {
		return err
	}
	if remote, remoteErr := gitutil.RemoteURL(); remoteErr != nil {
		return remoteErr
	} else if remote != "" {
		cfg, loadErr := config.Load()
		if loadErr != nil {
			return loadErr
		}
		if err = cfg.SetRepoKey(remote, newName); err != nil {
			return err
		}
	}

	fmt.Printf("Migrated %d file(s) to key %q.\n", len(files), newName)
	fmt.Println("Note: Git history still holds values encrypted under the old key.")
	fmt.Println("Rotate the underlying secrets at their source if this was a compromise.")
	return nil
}

func replaceFile(path string, data []byte, mode os.FileMode) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), ".envapor-migrate-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		_ = f.Close()
		if err != nil {
			_ = os.Remove(tmp)
		}
	}()
	if err = f.Chmod(mode); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, path); err != nil {
		return err
	}
	return nil
}

func loadPEMFile(path string) (*crypto.Key, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return crypto.LoadPEM(data)
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
