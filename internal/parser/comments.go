package parser

import "strings"

// stripComments removes `//` line comments and `/* ... */` block comments from
// the source while preserving line structure (newlines and leading indentation),
// so that line numbers and indentation are unaffected.
//
// A `//` only starts a comment at the beginning of content or when preceded by
// whitespace; this lets unquoted URLs like http://example.com survive. String
// literals are respected so delimiters inside them are not treated as comments.
func stripComments(src string) string {
	var b strings.Builder
	runes := []rune(src)
	n := len(runes)

	inBlock := false
	var strQuote rune // 0 when not inside a string

	for i := 0; i < n; i++ {
		c := runes[i]

		if inBlock {
			if c == '*' && i+1 < n && runes[i+1] == '/' {
				inBlock = false
				i++
			} else if c == '\n' {
				b.WriteRune(c)
			}
			continue
		}

		if strQuote != 0 {
			b.WriteRune(c)
			if c == '\\' && i+1 < n {
				b.WriteRune(runes[i+1])
				i++
			} else if c == strQuote {
				strQuote = 0
			}
			continue
		}

		switch {
		case c == '"' || c == '\'':
			strQuote = c
			b.WriteRune(c)
		case c == '/' && i+1 < n && runes[i+1] == '*':
			inBlock = true
			i++
		case c == '/' && i+1 < n && runes[i+1] == '/' && (i == 0 || isSpace(runes[i-1]) || runes[i-1] == '\n'):
			// Skip to end of line.
			for i < n && runes[i] != '\n' {
				i++
			}
			if i < n {
				b.WriteRune('\n')
			}
		default:
			b.WriteRune(c)
		}
	}

	return b.String()
}

func isSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\r' }
