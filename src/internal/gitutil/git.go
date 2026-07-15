package gitutil

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
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

// runWithStdin executes git with the given arguments and stdin, returning raw
// stdout bytes.
func runWithStdin(stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Stdin = bytes.NewReader(stdin)
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

// ShowStagedBatch returns the staged (index) content of many paths in a single
// `git cat-file --batch` invocation. Paths absent from the index are omitted
// from the result map rather than reported as an error.
func ShowStagedBatch(paths []string) (map[string][]byte, error) {
	result := make(map[string][]byte, len(paths))
	if len(paths) == 0 {
		return result, nil
	}
	var input bytes.Buffer
	for _, p := range paths {
		input.WriteString(":" + p + "\n")
	}
	out, err := runWithStdin(input.Bytes(), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}

	// Results stream back in request order. Each present object is framed as
	// "<oid> <type> <size>\n" followed by <size> content bytes and a trailing
	// newline; a missing object is a single "<rev> missing\n" line.
	pos := 0
	for _, p := range paths {
		nl := bytes.IndexByte(out[pos:], '\n')
		if nl < 0 {
			return nil, fmt.Errorf("unexpected cat-file output for %q", p)
		}
		header := string(out[pos : pos+nl])
		pos += nl + 1
		if strings.HasSuffix(header, " missing") {
			continue
		}
		fields := strings.Fields(header)
		if len(fields) != 3 {
			return nil, fmt.Errorf("unexpected cat-file header %q", header)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("unexpected cat-file size in %q: %w", header, err)
		}
		if pos+size > len(out) {
			return nil, fmt.Errorf("truncated cat-file content for %q", p)
		}
		content := make([]byte, size)
		copy(content, out[pos:pos+size])
		result[p] = content
		pos += size
		if pos < len(out) && out[pos] == '\n' {
			pos++
		}
	}
	return result, nil
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

// CheckAttrBatch resolves one gitattributes attribute for many paths in a
// single git invocation, returning a path-to-value map (values such as
// "envapor", "unset", or "unspecified"). It uses -z framing so paths and
// values are unambiguous regardless of their contents.
func CheckAttrBatch(attr string, paths []string) (map[string]string, error) {
	result := make(map[string]string, len(paths))
	if len(paths) == 0 {
		return result, nil
	}
	args := append([]string{"check-attr", "-z", attr, "--"}, paths...)
	out, err := runRaw(args...)
	if err != nil {
		return nil, err
	}
	// -z output is NUL-separated fields in repeating groups of three:
	// <path> NUL <attr> NUL <value> NUL.
	raw := strings.TrimSuffix(string(out), "\x00")
	if raw == "" {
		return result, nil
	}
	fields := strings.Split(raw, "\x00")
	if len(fields)%3 != 0 {
		return nil, fmt.Errorf("unexpected check-attr output: %d fields", len(fields))
	}
	for i := 0; i < len(fields); i += 3 {
		result[fields[i]] = fields[i+2]
	}
	return result, nil
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
