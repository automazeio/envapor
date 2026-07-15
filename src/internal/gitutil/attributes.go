package gitutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	attrBegin = "# >>> envapor >>>"
	attrEnd   = "# <<< envapor <<<"
)

// AttributesBlock renders the managed .gitattributes block, including the
// negated example-file exclusions. The negations must follow the ".env.*"
// pattern so they win, since later matching lines override earlier ones.
func AttributesBlock(exclusions []string) string {
	var b strings.Builder
	b.WriteString(attrBegin + "\n")
	b.WriteString(".env          filter=envapor diff=envapor\n")
	b.WriteString(".env.*        filter=envapor diff=envapor\n")
	for _, e := range exclusions {
		fmt.Fprintf(&b, "%-13s -filter -diff\n", e)
	}
	b.WriteString(attrEnd + "\n")
	return b.String()
}

// EnsureAttributes writes or updates the managed block in the repository's
// .gitattributes file, leaving any user-authored content outside the markers
// untouched.
func EnsureAttributes(root string, exclusions []string) error {
	path := filepath.Join(root, ".gitattributes")
	block := AttributesBlock(exclusions)

	existing, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return os.WriteFile(path, []byte(block), 0o644)
	}
	if err != nil {
		return fmt.Errorf("envapor: reading .gitattributes: %w", err)
	}

	content := string(existing)
	if start := strings.Index(content, attrBegin); start >= 0 {
		if end := strings.Index(content, attrEnd); end >= 0 {
			end += len(attrEnd)
			if end < len(content) && content[end] == '\n' {
				end++
			}
			content = content[:start] + block + content[end:]
			return os.WriteFile(path, []byte(content), 0o644)
		}
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += block
	return os.WriteFile(path, []byte(content), 0o644)
}

// HasAttributesBlock reports whether the managed block is present.
func HasAttributesBlock(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, ".gitattributes"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), attrBegin)
}
