package parser

import "strings"

// Object literals share the `{` character with block braces and
// interpolation, so two parser stages need to recognize them before any
// expression is lexed:
//
//   - joinObjectLiterals folds a multi-line literal onto its first line, so
//     the indentation tree sees one statement:
//
//     theme = {                      theme = {, bg: #fff,, fg: #333, }
//     bg: #fff,              →      (blank)
//     fg: #333                      (blank)
//     }                             (blank)
//
//     Each newline inside the literal becomes ", " (the object parser skips
//     empty entries, so lines that already end in a comma are fine), and the
//     removed newlines are re-emitted after the statement's line so later
//     line numbers don't shift.
//
//   - the brace-syntax scanner (scanStructural) treats a literal's braces as
//     text rather than a block (isObjectBrace).
//
// An object literal is recognized by what precedes its `{` on the same line:
// an assignment (`x = {`, `x ?= {`), an argument position (`f({` or
// `f(a, {`), or `return {`. No block brace or interpolation can follow those,
// since a selector never ends in `=`, `(` or `,` right before its block, and an
// interpolation after `=`/`(`/`,` is glued (`[a={v}]`, `:not({s})`), holds no
// newline and isn't touched by the fold.

// joinObjectLiterals folds each multi-line object literal onto one line.
// Sources without an object opener followed by a newline are returned as-is.
func joinObjectLiterals(src string) string {
	if !strings.Contains(src, "{") {
		return src
	}
	// Comments are removed first, so a `// note` inside a literal can't
	// swallow the rest of the folded line. stripComments keeps line structure
	// and the parser drops every comment anyway, so this changes nothing else.
	runes := []rune(stripComments(src))
	var b strings.Builder
	changed := false
	pending := 0 // newlines removed from the current line, re-emitted at its end
	brackets := 0
	lineStart := 0

	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case c == '"' || c == '\'':
			j := skipString(runes, i)
			b.WriteString(string(runes[i:j]))
			i = j - 1
			continue
		case c == '[':
			brackets++
		case c == ']':
			if brackets > 0 {
				brackets--
			}
		case c == '\n':
			b.WriteRune('\n')
			b.WriteString(strings.Repeat("\n", pending))
			pending, brackets, lineStart = 0, 0, i+1
			continue
		case c == '{' && brackets == 0 && objectOpener(runes[lineStart:i]):
			end := matchObjectBrace(runes, i)
			if end < 0 {
				break
			}
			body := string(runes[i : end+1])
			if n := strings.Count(body, "\n"); n > 0 {
				b.WriteString(foldLines(body))
				pending += n
				changed = true
				i = end
				continue
			}
		}
		b.WriteRune(c)
	}
	b.WriteString(strings.Repeat("\n", pending))
	if !changed {
		return src
	}
	return b.String()
}

// foldLines joins a literal's lines with ", ", dropping each line's
// indentation.
func foldLines(body string) string {
	lines := strings.Split(body, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return strings.Join(lines, ", ")
}

// objectOpener reports whether the text before a `{` on its line (prefix)
// places the brace in value position: after `=`, `(` or `,`, or after the
// `return` keyword.
func objectOpener(prefix []rune) bool {
	s := strings.TrimRight(string(prefix), " \t")
	if s == "" {
		return false
	}
	switch s[len(s)-1] {
	case '=', '(', ',':
		return true
	}
	t := strings.TrimSpace(s)
	return t == "return"
}

// isObjectBrace reports whether the `{` at open begins an object literal (see
// objectOpener), looking back to the start of its line.
func isObjectBrace(runes []rune, open int) bool {
	start := open
	for start > 0 && runes[start-1] != '\n' {
		start--
	}
	return objectOpener(runes[start:open])
}

// matchObjectBrace returns the index of the '}' closing the '{' at open,
// skipping string literals, or -1.
func matchObjectBrace(runes []rune, open int) int {
	depth := 0
	for i := open; i < len(runes); i++ {
		switch runes[i] {
		case '"', '\'':
			i = skipString(runes, i) - 1
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// skipString returns the index just past the string literal starting at i
// (escapes honored; an unterminated string runs to the end of its line).
func skipString(runes []rune, i int) int {
	q := runes[i]
	j := i + 1
	for j < len(runes) && runes[j] != '\n' {
		if runes[j] == '\\' && j+1 < len(runes) {
			j += 2
			continue
		}
		if runes[j] == q {
			return j + 1
		}
		j++
	}
	return j
}
