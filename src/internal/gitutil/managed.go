package gitutil

import (
	"path/filepath"
	"strings"
)

// DefaultExclusions are the example/template files that must never be
// encrypted. They are conventionally committed as readable placeholders.
var DefaultExclusions = []string{".env.example", ".env.sample", ".env.template"}

// IsManagedName reports whether a path's base name is a managed .env file
// (".env" or ".env.*") that is not on the exclusion list.
func IsManagedName(path string, exclusions []string) bool {
	base := filepath.Base(path)
	if base != ".env" && !strings.HasPrefix(base, ".env.") {
		return false
	}
	for _, e := range exclusions {
		if base == e {
			return false
		}
	}
	return true
}

// ManagedFiles returns tracked and untracked (non-ignored) managed .env files.
func ManagedFiles(exclusions []string) ([]string, error) {
	out, err := run("ls-files", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	return filterManaged(out, exclusions), nil
}

// IgnoredManagedFiles returns managed .env files that Git is currently ignoring
// (they would silently never be committed, which doctor should flag).
func IgnoredManagedFiles(exclusions []string) ([]string, error) {
	out, err := run("ls-files", "--others", "--ignored", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	return filterManaged(out, exclusions), nil
}

func filterManaged(out string, exclusions []string) []string {
	if out == "" {
		return nil
	}
	seen := map[string]bool{}
	var files []string
	for _, p := range strings.Split(out, "\n") {
		if p == "" || seen[p] {
			continue
		}
		if IsManagedName(p, exclusions) {
			seen[p] = true
			files = append(files, p)
		}
	}
	return files
}
