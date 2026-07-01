package parser

import (
	"strings"
	"unicode"
)

// Stylus accepts both the indentation syntax and a CSS-like brace/semicolon
// syntax. The rest of the parser is indentation-based, so when a source uses
// braces we normalize it into the equivalent indented form up front
// (bracesToIndent) and feed that to the existing line-tree builder.
//
// Interpolation (`{expr}`) shares the brace character with block delimiters. They
// are told apart structurally: an interpolation brace is glued (no space) to an
// identifier on its left or right, whereas a block brace stands alone. This means
// a stand-alone `{expr}` in value position (e.g. `width {x}`) is not supported in
// brace syntax — use the bare variable (`width x`) instead.

// usesBraces reports whether src contains a block-delimiting `{...}` (as opposed
// to only interpolation), in which case it should be normalized before parsing.
func usesBraces(src string) bool {
	runes := []rune(src)
	found := false
	scanStructural(runes, scanHandlers{
		block: func(open, close int) bool {
			// A block's body spans lines or carries a declaration separator.
			body := runes[open+1 : close]
			if strings.ContainsAny(string(body), "\n;:") {
				found = true
			}
			return found // stop early once we know
		},
	})
	return found
}

// bracesToIndent rewrites brace/semicolon source into indentation form: each block
// `{` opens an indented child block, `}` closes it, and `;` (and newlines)
// separate statements. Interpolation braces and string/comment contents are
// preserved verbatim; comments are dropped (the line-tree builder strips them
// anyway).
//
// Statements are emitted on their original source line (padding with blank
// lines as needed) so that error positions and source maps remain accurate.
// Only statements sharing a source line (one-liner blocks) drift downward.
func bracesToIndent(src string) string {
	runes := []rune(src)
	var out strings.Builder
	var lineBuf strings.Builder
	depth := 0
	srcLine := 1 // source line currently being scanned
	bufLine := 1 // source line where the buffered statement began
	outLine := 1 // line number the next output write lands on
	hasContent := false

	flush := func() {
		s := strings.TrimSpace(lineBuf.String())
		lineBuf.Reset()
		hasContent = false
		if s == "" {
			return
		}
		for outLine < bufLine {
			out.WriteByte('\n')
			outLine++
		}
		out.WriteString(strings.Repeat("  ", depth))
		out.WriteString(s)
		out.WriteByte('\n')
		outLine++
	}

	buffer := func(s string) {
		if !hasContent && strings.TrimSpace(s) != "" {
			hasContent = true
			bufLine = srcLine
		}
		lineBuf.WriteString(s)
		srcLine += strings.Count(s, "\n")
	}

	scanStructural(runes, scanHandlers{
		text:   buffer,
		interp: buffer,
		open: func() {
			flush() // the buffered text is the block header (selector/at-rule)
			depth++
		},
		close: func() {
			flush()
			if depth > 0 {
				depth--
			}
		},
		semi: func() { flush() },
		newline: func() {
			flush()
			srcLine++
		},
		skip: func(n int) { srcLine += n },
	})
	flush()
	return out.String()
}

// scanHandlers receives structural events from scanStructural. Any handler may be
// nil. block is only used by usesBraces to classify (and short-circuit on) a
// candidate block brace; returning true stops the scan.
type scanHandlers struct {
	text    func(string) // run of ordinary characters (and strings) to copy
	interp  func(string) // an interpolation `{...}` group, copied verbatim
	open    func()       // a block-opening brace
	close   func()       // a block-closing brace
	semi    func()       // a top-level ';'
	newline func()       // a source newline
	skip    func(int)    // newlines consumed silently (inside skipped comments)
	block   func(int, int) bool
}

// scanStructural walks runes outside of strings and comments, classifying braces
// as interpolation vs block delimiters and reporting structural events. Comments
// are skipped (dropped). String literals are passed through via the text handler.
func scanStructural(runes []rune, h scanHandlers) {
	n := len(runes)
	emitText := func(s string) {
		if h.text != nil && s != "" {
			h.text(s)
		}
	}

	for i := 0; i < n; i++ {
		c := runes[i]
		switch {
		case c == '"' || c == '\'':
			// String literal: copy verbatim, honoring escapes.
			j := i + 1
			for j < n {
				if runes[j] == '\\' && j+1 < n {
					j += 2
					continue
				}
				if runes[j] == c {
					j++
					break
				}
				j++
			}
			emitText(string(runes[i:j]))
			i = j - 1

		case c == '/' && i+1 < n && runes[i+1] == '*':
			j := i + 2
			for j+1 < n && !(runes[j] == '*' && runes[j+1] == '/') {
				j++
			}
			if h.skip != nil {
				nl := 0
				for k := i; k <= j+1 && k < n; k++ {
					if runes[k] == '\n' {
						nl++
					}
				}
				if nl > 0 {
					h.skip(nl)
				}
			}
			i = j + 1 // skip past "*/"

		case c == '/' && i+1 < n && runes[i+1] == '/' &&
			(i == 0 || isSpace(runes[i-1]) || runes[i-1] == '\n'):
			for i+1 < n && runes[i+1] != '\n' {
				i++
			}

		case c == '{':
			if end := matchRuneBrace(runes, i); end >= 0 && isInterpBrace(runes, i, end) {
				if h.interp != nil {
					h.interp(string(runes[i : end+1]))
				}
				i = end
			} else {
				if h.block != nil {
					if stop := h.block(i, braceEnd(runes, end)); stop {
						return
					}
				}
				if h.open != nil {
					h.open()
				}
			}

		case c == '}':
			if h.close != nil {
				h.close()
			}

		case c == ';':
			if h.semi != nil {
				h.semi()
			}

		case c == '\n':
			if h.newline != nil {
				h.newline()
			}

		default:
			emitText(string(c))
		}
	}
}

// braceEnd returns end if non-negative, else the slice length (used so usesBraces
// can examine a block body even when the matching brace is missing).
func braceEnd(runes []rune, end int) int {
	if end >= 0 {
		return end
	}
	return len(runes)
}

// matchRuneBrace returns the index of the '}' matching the '{' at open, or -1.
func matchRuneBrace(runes []rune, open int) int {
	depth := 0
	for i := open; i < len(runes); i++ {
		switch runes[i] {
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

// isInterpBrace reports whether the `{...}` spanning [open,close] is an
// interpolation group rather than a block delimiter: it is glued (no whitespace)
// to an identifier-like character on its left or immediately after its close.
func isInterpBrace(runes []rune, open, close int) bool {
	if open > 0 && isGlue(runes[open-1]) {
		return true
	}
	if close+1 < len(runes) && isGlue(runes[close+1]) {
		return true
	}
	return false
}

// isGlue reports whether c can be part of an interpolated identifier/selector
// adjacent to a brace.
func isGlue(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) ||
		c == '_' || c == '-' || c == '$' || c == '%' || c == ')' || c == '}'
}
