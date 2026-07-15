// Package envfile parses and rewrites .env files, encrypting only the values
// while preserving keys, comments, ordering, and formatting verbatim.
package envfile

import "strings"

// line represents a single physical line of a .env file.
//
// Non-assignment lines (blanks, full-line comments, anything unparseable) are
// preserved exactly via raw. Assignment lines are split into head (leading
// whitespace, optional export, key, and '='), value (the raw value token,
// including surrounding quotes if present), and tail (trailing whitespace and
// any inline comment) so that head + value + tail == raw.
type line struct {
	assignment bool
	raw        string
	head       string
	key        string
	value      string
	tail       string
	public     bool
}

// parseLine splits a single line (without its trailing newline) into its parts.
func parseLine(core string) line {
	l := line{raw: core}
	prefix := ""
	if strings.HasPrefix(core, "\uFEFF") {
		prefix = "\uFEFF"
		core = strings.TrimPrefix(core, prefix)
	}

	lead := 0
	for lead < len(core) && (core[lead] == ' ' || core[lead] == '\t') {
		lead++
	}
	rest := core[lead:]
	if rest == "" || rest[0] == '#' {
		return l
	}

	cursor := lead
	if after, ok := trimExport(rest); ok {
		cursor += len(rest) - len(after)
		rest = after
	}

	keyLen := scanKey(rest)
	if keyLen == 0 || keyLen >= len(rest) || rest[keyLen] != '=' {
		return l
	}

	l.assignment = true
	l.key = rest[:keyLen]
	l.head = prefix + core[:cursor+keyLen+1] // through '='
	valpart := core[cursor+keyLen+1:]

	value, tail := splitValue(valpart)
	l.value = value
	l.tail = tail
	l.public = isPublicComment(tail)
	return l
}

// trimExport strips an optional "export " prefix used by shell-style .env files.
func trimExport(s string) (string, bool) {
	const kw = "export"
	if !strings.HasPrefix(s, kw) {
		return s, false
	}
	r := s[len(kw):]
	if r == "" || (r[0] != ' ' && r[0] != '\t') {
		return s, false
	}
	for len(r) > 0 && (r[0] == ' ' || r[0] == '\t') {
		r = r[1:]
	}
	return r, true
}

// scanKey returns the length of a valid leading environment-variable name.
func scanKey(s string) int {
	i := 0
	for i < len(s) {
		c := s[i]
		valid := c == '_' ||
			(c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(i > 0 && (c >= '0' && c <= '9')) ||
			(i > 0 && (c == '.' || c == '-'))
		if !valid {
			break
		}
		i++
	}
	return i
}

// splitValue separates the raw value token from any trailing inline comment.
//
// It fails closed: whenever the boundary between value and comment is
// ambiguous (for example an unterminated quote), the entire remainder is
// treated as the value so it gets encrypted rather than leaked as plaintext.
func splitValue(valpart string) (value, tail string) {
	if valpart == "" {
		return "", ""
	}
	if q := valpart[0]; q == '"' || q == '\'' {
		if end := closingQuote(valpart, q); end >= 0 {
			return valpart[:end+1], valpart[end+1:]
		}
		return valpart, "" // unterminated quote: encrypt everything
	}
	if idx := inlineCommentStart(valpart); idx >= 0 {
		return valpart[:idx], valpart[idx:]
	}
	return valpart, ""
}

// closingQuote returns the index of the matching closing quote, or -1 if none.
// Backslash escapes are honoured inside double quotes only.
func closingQuote(s string, q byte) int {
	for i := 1; i < len(s); i++ {
		if q == '"' && s[i] == '\\' {
			i++
			continue
		}
		if s[i] == q {
			return i
		}
	}
	return -1
}

// inlineCommentStart returns the index where trailing-comment whitespace begins,
// or -1 when the value has no inline comment. A comment requires a '#' at the
// start of the segment or preceded by whitespace, matching dotenv convention.
func inlineCommentStart(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != '#' {
			continue
		}
		if i == 0 {
			return 0
		}
		if s[i-1] == ' ' || s[i-1] == '\t' {
			j := i - 1
			for j > 0 && (s[j-1] == ' ' || s[j-1] == '\t') {
				j--
			}
			return j
		}
	}
	return -1
}

// isPublicComment reports whether tail holds an unambiguous "# PUBLIC" marker.
func isPublicComment(tail string) bool {
	i := strings.IndexByte(tail, '#')
	if i < 0 {
		return false
	}
	body := strings.TrimLeft(tail[i+1:], " \t")
	const marker = "PUBLIC"
	if !strings.HasPrefix(body, marker) {
		return false
	}
	rest := body[len(marker):]
	if rest == "" {
		return true
	}
	switch rest[0] {
	case ' ', '\t', ':', '-':
		return true
	}
	return false
}
