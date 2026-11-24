package db

import (
	"strings"
	"unicode"
)

// SplitStatements splits a SQL script into individual statements on
// top-level semicolons. Semicolons inside single/double quotes,
// dollar-quoted strings ($$ … $$, $tag$ … $tag$), line comments (--) and
// (nested) block comments (/* … */) are ignored. Chunks made only of
// whitespace and comments are dropped. Each statement is trimmed and
// returned without its trailing semicolon.
func SplitStatements(sql string) []string {
	rs := []rune(sql)
	n := len(rs)
	var out []string
	var cur strings.Builder
	hasCode := false

	flush := func() {
		if hasCode {
			if s := strings.TrimSpace(cur.String()); s != "" {
				out = append(out, s)
			}
		}
		cur.Reset()
		hasCode = false
	}

	for i := 0; i < n; i++ {
		c := rs[i]
		switch {
		case c == '\'' || c == '"':
			// quoted literal/identifier; a doubled quote is an escaped quote
			j := i + 1
			for j < n {
				if rs[j] == c {
					if j+1 < n && rs[j+1] == c {
						j += 2
						continue
					}
					break
				}
				j++
			}
			if j >= n {
				j = n - 1 // unterminated: swallow the rest
			}
			cur.WriteString(string(rs[i : j+1]))
			hasCode = true
			i = j

		case c == '-' && i+1 < n && rs[i+1] == '-':
			j := i
			for j < n && rs[j] != '\n' {
				j++
			}
			cur.WriteString(string(rs[i:j]))
			i = j - 1

		case c == '/' && i+1 < n && rs[i+1] == '*':
			depth := 0
			j := i
			for j < n {
				if rs[j] == '/' && j+1 < n && rs[j+1] == '*' {
					depth++
					j += 2
					continue
				}
				if rs[j] == '*' && j+1 < n && rs[j+1] == '/' {
					depth--
					j += 2
					if depth == 0 {
						break
					}
					continue
				}
				j++
			}
			if j > n {
				j = n
			}
			cur.WriteString(string(rs[i:j]))
			i = j - 1

		case c == '$':
			tag, ok := dollarTag(rs, i)
			if !ok {
				cur.WriteRune(c)
				hasCode = true
				continue
			}
			end := indexRunes(rs, i+len(tag), tag)
			if end < 0 {
				cur.WriteString(string(rs[i:]))
				hasCode = true
				i = n
				continue
			}
			cur.WriteString(string(rs[i : end+len(tag)]))
			hasCode = true
			i = end + len(tag) - 1

		case c == ';':
			flush()

		default:
			cur.WriteRune(c)
			if !unicode.IsSpace(c) {
				hasCode = true
			}
		}
	}
	flush()
	return out
}

// dollarTag returns the dollar-quote tag ($$ or $tag$) starting at rs[i].
func dollarTag(rs []rune, i int) ([]rune, bool) {
	j := i + 1
	for j < len(rs) && rs[j] != '$' {
		r := rs[j]
		if j == i+1 {
			if !(unicode.IsLetter(r) || r == '_') {
				return nil, false
			}
		} else if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return nil, false
		}
		j++
	}
	if j >= len(rs) {
		return nil, false
	}
	return rs[i : j+1], true
}

// indexRunes returns the index of the first occurrence of sub in rs at or
// after from, or -1.
func indexRunes(rs []rune, from int, sub []rune) int {
	for i := from; i+len(sub) <= len(rs); i++ {
		match := true
		for k := range sub {
			if rs[i+k] != sub[k] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
