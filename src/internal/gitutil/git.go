package gitutil

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// run executes git with the given arguments and returns trimmed stdout.
func run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// runRaw executes git and returns raw (untrimmed) stdout bytes.
func runRaw(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

// InsideRepo reports whether the current directory is within a Git work tree.
func InsideRepo() bool {
	out, err := run("rev-parse", "--is-inside-work-tree")
	return err == nil && out == "true"
}

// Root returns the absolute path of the repository's top-level directory.
func Root() (string, error) {
	return run("rev-parse", "--show-toplevel")
}

// GitPath resolves a path within the .git directory (e.g. "hooks").
func GitPath(name string) (string, error) {
	return run("rev-parse", "--git-path", name)
}

// RemoteURL returns the origin remote URL, falling back to the first configured
// remote. It returns ("", nil) when the repository has no remotes yet.
func RemoteURL() (string, error) {
	if url, err := run("config", "--get", "remote.origin.url"); err == nil && url != "" {
		return url, nil
	}
	remotes, err := run("remote")
	if err != nil || remotes == "" {
		return "", nil
	}
	first := strings.SplitN(remotes, "\n", 2)[0]
	url, err := run("config", "--get", fmt.Sprintf("remote.%s.url", first))
	if err != nil {
		return "", nil
	}
	return url, nil
}

// ConfigGet returns a local git config value, or "" if unset.
func ConfigGet(key string) string {
	out, err := run("config", "--local", "--get", key)
	if err != nil {
		return ""
	}
	return out
}

// ConfigSet writes a local git config value.
func ConfigSet(key, value string) error {
	_, err := run("config", "--local", key, value)
	return err
}

// ConfigUnset removes a local git config value, tolerating its absence.
func ConfigUnset(key string) error {
	if ConfigGet(key) == "" {
		return nil
	}
	_, err := run("config", "--local", "--unset", key)
	return err
}

// TrackedFiles returns the repository-relative paths tracked by Git.
func TrackedFiles() ([]string, error) {
	out, err := run("ls-files")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// StagedFiles returns the repository-relative paths staged for commit.
func StagedFiles() ([]string, error) {
	out, err := run("diff", "--cached", "--diff-filter=ACMR", "--name-only")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// ShowStaged returns the staged (index) content of a path.
func ShowStaged(path string) ([]byte, error) {
	return runRaw("show", ":"+path)
}

// IndexEntry captures a path's exact index object so transactional commands
// can restore pre-existing staged content without re-cleaning the work tree.
type IndexEntry struct {
	Path    string
	Mode    string
	Object  string
	Present bool
}

// SnapshotIndex records the current stage-zero index entries for paths.
func SnapshotIndex(paths ...string) ([]IndexEntry, error) {
	entries := make([]IndexEntry, 0, len(paths))
	for _, path := range paths {
		out, err := run("ls-files", "--stage", "--", path)
		if err != nil {
			return nil, err
		}
		entry := IndexEntry{Path: path}
		if out != "" {
			lines := strings.Split(out, "\n")
			if len(lines) != 1 {
				return nil, fmt.Errorf("envapor: cannot migrate unmerged index path %q", path)
			}
			meta, _, found := strings.Cut(lines[0], "\t")
			fields := strings.Fields(meta)
			if !found || len(fields) != 3 || fields[2] != "0" {
				return nil, fmt.Errorf("envapor: unexpected index entry for %q", path)
			}
			entry.Mode = fields[0]
			entry.Object = fields[1]
			entry.Present = true
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// RestoreIndex restores entries captured by SnapshotIndex.
func RestoreIndex(entries []IndexEntry) error {
	for _, entry := range entries {
		if !entry.Present {
			if _, err := run("update-index", "--force-remove", "--", entry.Path); err != nil {
				return err
			}
			continue
		}
		if _, err := run("update-index", "--add", "--cacheinfo", entry.Mode, entry.Object, entry.Path); err != nil {
			return err
		}
	}
	return nil
}

// CheckAttr returns the resolved value of a gitattributes attribute for a path
// (for example "envapor", "unset", or "unspecified").
func CheckAttr(attr, path string) (string, error) {
	out, err := run("check-attr", attr, "--", path)
	if err != nil {
		return "", err
	}
	// Output form: "<path>: <attr>: <value>"
	idx := strings.LastIndex(out, ": ")
	if idx < 0 {
		return "", fmt.Errorf("unexpected check-attr output: %q", out)
	}
	return out[idx+2:], nil
}

// Stage adds paths to the index, applying the clean filter so managed files are
// stored encrypted. It works for both tracked and untracked files.
func Stage(paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"add", "--"}, paths...)
	_, err := run(args...)
	return err
}
