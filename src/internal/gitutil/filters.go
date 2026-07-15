package gitutil

import "strings"

// shellQuote quotes a value for the POSIX shell used by Git.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// ConfigureFilters installs the long-running clean/smudge filter and the diff
// textconv driver, pointing them at the given Envapor executable. Using the
// `process` protocol lets a single process handle every managed file per git
// operation, so the key is resolved once instead of per file. The filter is
// marked required so a missing filter aborts rather than silently committing
// plaintext.
func ConfigureFilters(exe string) error {
	q := shellQuote(exe)
	settings := [][2]string{
		{"filter.envapor.process", q + " filter-process"},
		{"filter.envapor.required", "true"},
		{"diff.envapor.textconv", q + " textconv"},
	}
	for _, s := range settings {
		if err := ConfigSet(s[0], s[1]); err != nil {
			return err
		}
	}
	// Drop any legacy single-shot filter config so only the process filter
	// drives clean/smudge.
	if err := ConfigUnset("filter.envapor.clean"); err != nil {
		return err
	}
	return ConfigUnset("filter.envapor.smudge")
}

// FiltersConfigured reports whether the required filter settings are present.
func FiltersConfigured() bool {
	return ConfigGet("filter.envapor.process") != "" &&
		ConfigGet("filter.envapor.required") == "true"
}
