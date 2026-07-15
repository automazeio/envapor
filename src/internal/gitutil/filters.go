package gitutil

import "strings"

// shellQuote quotes a value for the POSIX shell used by Git.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// ConfigureFilters installs the clean/smudge filter and the diff textconv
// driver, pointing them at the given Envapor executable. The filter is marked
// required so a missing filter aborts rather than silently committing
// plaintext.
func ConfigureFilters(exe string) error {
	q := shellQuote(exe)
	settings := [][2]string{
		{"filter.envapor.clean", q + " clean %f"},
		{"filter.envapor.smudge", q + " smudge %f"},
		{"filter.envapor.required", "true"},
		{"diff.envapor.textconv", q + " textconv"},
	}
	for _, s := range settings {
		if err := ConfigSet(s[0], s[1]); err != nil {
			return err
		}
	}
	return nil
}

// FiltersConfigured reports whether the required filter settings are present.
func FiltersConfigured() bool {
	return ConfigGet("filter.envapor.clean") != "" &&
		ConfigGet("filter.envapor.smudge") != "" &&
		ConfigGet("filter.envapor.required") == "true"
}
