package envfile

import (
	"fmt"
	"strings"

	"github.com/automazeio/envapor/internal/crypto"
)

// Encrypt rewrites content so every non-public assignment value becomes an
// ENC[...] token. Public values, comments, blank lines, and formatting are
// preserved. Values that are already encrypted are left untouched, making the
// operation idempotent.
func Encrypt(content []byte, k *crypto.Key) ([]byte, error) {
	return transform(content, true, func(l *line) error {
		if l.public || crypto.IsEncrypted(l.value) {
			return nil
		}
		token, err := k.EncryptContext([]byte(l.value), l.key)
		if err != nil {
			return err
		}
		l.value = token
		return nil
	})
}

// Decrypt rewrites content so every ENC[...] value is restored to plaintext.
// Values that are not encrypted are left untouched, making the operation
// idempotent.
func Decrypt(content []byte, k *crypto.Key) ([]byte, error) {
	return transform(content, false, func(l *line) error {
		if !crypto.IsEncrypted(l.value) {
			return nil
		}
		plaintext, err := k.DecryptContext(l.value, l.key)
		if err != nil {
			return err
		}
		l.value = string(plaintext)
		return nil
	})
}

// Violation describes a non-public assignment whose value is still plaintext.
type Violation struct {
	Line int
	Key  string
}

// Verify returns the assignments that would leak plaintext if committed. It is
// the check backing the pre-commit guard and does not require the key, since it
// only inspects whether values are already encrypted or explicitly public.
func Verify(content []byte) []Violation {
	var out []Violation
	for _, core := range splitLines(content) {
		l := parseLine(core.text)
		if !l.assignment {
			if !safeNonAssignment(l.raw) {
				out = append(out, Violation{Line: core.line, Key: "<unparseable>"})
			}
			continue
		}
		if l.public {
			continue
		}
		if !crypto.IsEncrypted(l.value) {
			out = append(out, Violation{Line: core.line, Key: l.key})
		}
	}
	return out
}

func transform(content []byte, strict bool, fn func(*line) error) ([]byte, error) {
	segs := splitLines(content)
	var b strings.Builder
	b.Grow(len(content) + 64)
	for _, seg := range segs {
		l := parseLine(seg.text)
		if l.assignment {
			if err := fn(&l); err != nil {
				return nil, err
			}
			b.WriteString(l.head)
			b.WriteString(l.value)
			b.WriteString(l.tail)
		} else {
			if strict && !safeNonAssignment(l.raw) {
				return nil, fmt.Errorf("envapor: line %d is not a valid assignment or comment", seg.line)
			}
			b.WriteString(l.raw)
		}
		b.WriteString(seg.ending)
	}
	return []byte(b.String()), nil
}

func safeNonAssignment(raw string) bool {
	trimmed := strings.TrimSpace(strings.TrimPrefix(raw, "\uFEFF"))
	return trimmed == "" || strings.HasPrefix(trimmed, "#")
}

// segment is a physical line together with the line ending that followed it,
// which is preserved exactly (including CRLF and a possibly-absent final
// newline) so round trips are byte-faithful.
type segment struct {
	text   string
	ending string
	line   int
}

func splitLines(content []byte) []segment {
	s := string(content)
	var physical []segment
	line := 1
	for len(s) > 0 {
		nl := strings.IndexByte(s, '\n')
		if nl < 0 {
			physical = append(physical, segment{text: s, line: line})
			break
		}
		text := s[:nl]
		ending := "\n"
		if len(text) > 0 && text[len(text)-1] == '\r' {
			text = text[:len(text)-1]
			ending = "\r\n"
		}
		physical = append(physical, segment{text: text, ending: ending, line: line})
		s = s[nl+1:]
		line++
	}

	var segs []segment
	for i := 0; i < len(physical); i++ {
		seg := physical[i]
		l := parseLine(seg.text)
		if !l.assignment || !unterminatedQuotedValue(l.value) {
			segs = append(segs, seg)
			continue
		}
		for i+1 < len(physical) && unterminatedQuotedValue(l.value) {
			seg.text += seg.ending + physical[i+1].text
			seg.ending = physical[i+1].ending
			i++
			l = parseLine(seg.text)
		}
		segs = append(segs, seg)
	}
	return segs
}

func unterminatedQuotedValue(value string) bool {
	if value == "" || (value[0] != '"' && value[0] != '\'') {
		return false
	}
	return closingQuote(value, value[0]) < 0
}
