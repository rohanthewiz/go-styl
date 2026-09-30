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
// lines as needed) so that comment attachment, which keys on line numbers,
// still lines up. Only statements sharing a source line (one-liner blocks)
// drift downward; the positions they report come from the returned position
// table instead (see "Source positions" below), so errors and source maps
// stay exact even for those.
//
// Mixed files. Stylus lets one file use both syntaxes — an indented mixin whose
// body contains brace blocks, or a brace block followed by indented rules — so
// the output depth of a statement comes from one of two places:
//
//   - Inside braces, structure is the braces': a statement sits one level below
//     the header of the innermost open `{`, whatever its source indentation.
//   - Outside all braces, structure is the source indentation, resolved with the
//     same pop-until-shallower stack the line-tree builder uses (indentStack).
//
// For example:
//
//	m()                 depth 0  (indentation)
//	  .a {              depth 1  (indentation) ── opens brace, base = 1
//	    color: red;     depth 2  (brace: base + 1)
//	    .b { x: 1 }     depth 2, its body 3
//	  }                 closes; `.a` is now a closed braced header
//	  .d                depth 1  (indentation)
//	    color blue      depth 2
//
// A header whose block was braced never adopts indentation children: a later
// line indented deeper than it becomes its sibling. This keeps pure brace files
// (where indentation is cosmetic) behaving exactly as before, e.g. a stray
// indent after `}` does not nest the next rule inside the closed one.
//
// Source positions. Re-emitting a statement at its output depth rewrites its
// indentation, and a one-line block (`.a { x: 1; y: 2 }`) spreads over several
// output lines, so neither the output line's indentation nor (for a one-liner)
// its line number says where the statement sits in the source. The second
// result records that per output line: pos[k] is the 1-based source line and
// column of the first character of the statement on output line k+1, and the
// zero srcPos for a padding line. buildTree copies it onto each line, and it
// is what statement and value positions (errors, source maps) report. The
// statement text itself is copied verbatim (only comments are dropped), so an
// offset into it is an offset from that column.
func bracesToIndent(src string) (string, []srcPos) {
	runes := []rune(src)
	var pos []srcPos
	var out strings.Builder
	var lineBuf strings.Builder
	srcLine := 1 // source line currently being scanned
	bufLine := 1 // source line where the buffered statement began
	outLine := 1 // line number the next output write lands on
	hasContent := false

	// braceDepths holds the output depth of the header of each open `{`,
	// innermost last. Empty means we are outside all braces.
	var braceDepths []int

	// indentStack is the indentation ancestry for statements outside braces.
	// braced marks a header whose `{…}` block has been opened: it can have
	// siblings at deeper indentation but never indentation children.
	type indentEntry struct {
		indent, depth int
		braced        bool
	}
	var indentStack []indentEntry

	// Per-statement facts captured when its first non-space character arrives.
	scanAt := 0             // rune index of the run being buffered (the `at` hook)
	bufCol := 0             // 1-based source column where the buffered statement began
	bufIndent := 0          // source indentation width of the buffered statement
	bufAtLineStart := false // statement begins its source line (vs after `;`/`}`)
	lineFlushed := false    // something was already emitted/consumed on this line

	lastDepth := 0     // output depth of the most recently emitted statement
	lastStmt := ""     // text of the most recently emitted statement
	lastIndented := -1 // indentStack index of the last statement placed by indentation

	// depthOf decides the output depth of the buffered statement s.
	depthOf := func(s string) int {
		if len(braceDepths) > 0 {
			lastIndented = -1
			return braceDepths[len(braceDepths)-1] + 1
		}
		// Outside braces. A statement that shares a line with a previous one
		// (`a; b`, `} .b {`) is its sibling; so is the line after a selector
		// that ends in a comma (a multi-line selector group).
		if !bufAtLineStart {
			lastIndented = -1
			return lastDepth
		}
		if strings.HasSuffix(lastStmt, ",") {
			// Continuation of a selector group: the group's first line keeps
			// the indentStack entry (lastIndented), so a `{` after the group
			// marks that entry braced.
			return lastDepth
		}
		for len(indentStack) > 0 && indentStack[len(indentStack)-1].indent >= bufIndent {
			indentStack = indentStack[:len(indentStack)-1]
		}
		depth := 0
		if n := len(indentStack); n > 0 {
			top := indentStack[n-1]
			if top.braced {
				depth = top.depth // sibling of a closed braced header
			} else {
				depth = top.depth + 1
			}
		}
		indentStack = append(indentStack, indentEntry{indent: bufIndent, depth: depth})
		lastIndented = len(indentStack) - 1
		return depth
	}

	// colOf returns the 1-based column of runes[at]. Statements arrive in
	// source order, so it scans forward from where the last call stopped,
	// tracking the latest line start; counting back from each statement
	// instead would be quadratic on a long single-line (minified) file.
	lineStart, scannedTo := 0, 0
	colOf := func(at int) int {
		for ; scannedTo < at; scannedTo++ {
			if runes[scannedTo] == '\n' {
				lineStart = scannedTo + 1
			}
		}
		return at - lineStart + 1
	}

	flush := func() {
		s := strings.TrimSpace(lineBuf.String())
		lineBuf.Reset()
		hasContent = false
		if s == "" {
			return
		}
		depth := depthOf(s)
		for outLine < bufLine {
			out.WriteByte('\n')
			outLine++
		}
		// outLine is the 1-based line this statement lands on; padding lines
		// before it have no statement and keep the zero position.
		for len(pos) < outLine-1 {
			pos = append(pos, srcPos{})
		}
		pos = append(pos, srcPos{line: bufLine, col: bufCol})
		out.WriteString(strings.Repeat("  ", depth))
		out.WriteString(s)
		out.WriteByte('\n')
		outLine++
		lastDepth, lastStmt = depth, s
		lineFlushed = true
	}

	buffer := func(s string) {
		if !hasContent && strings.TrimSpace(s) != "" {
			hasContent = true
			bufLine = srcLine
			// The run starts at scanAt; skip any leading blanks in it.
			// Measuring against the original runes (not lineBuf) keeps
			// comments dropped earlier on the line in the column.
			bufCol = colOf(scanAt + len([]rune(s)) - len([]rune(strings.TrimLeft(s, " \t"))))
			bufAtLineStart = !lineFlushed
			// lineBuf holds only this line's leading whitespace so far.
			bufIndent, _ = splitIndent(lineBuf.String() + "x")
		}
		lineBuf.WriteString(s)
		srcLine += strings.Count(s, "\n")
	}

	scanStructural(runes, scanHandlers{
		at:     func(i int) { scanAt = i },
		text:   buffer,
		interp: buffer,
		open: func() {
			// The buffered text is the block header (selector/at-rule). When it
			// is empty (`{` on its own line) the header is the previous statement.
			flush()
			if len(braceDepths) == 0 && lastIndented >= 0 && lastIndented < len(indentStack) {
				indentStack[lastIndented].braced = true
			}
			braceDepths = append(braceDepths, lastDepth)
			lineFlushed = true
		},
		close: func() {
			flush()
			if n := len(braceDepths); n > 0 {
				// The closed block's header becomes the reference point for a
				// statement that follows on the same line (`} .next {`).
				lastDepth = braceDepths[n-1]
				braceDepths = braceDepths[:n-1]
			}
			lastStmt = ""
			lastIndented = -1
			lineFlushed = true
		},
		semi: func() { flush() },
		newline: func() {
			flush()
			srcLine++
			lineFlushed = false
		},
		skip: func(n int) { srcLine += n },
	})
	flush()
	return out.String(), pos
}

// scanHandlers receives structural events from scanStructural. Any handler may be
// nil. block is only used by usesBraces to classify (and short-circuit on) a
// candidate block brace; returning true stops the scan.
type scanHandlers struct {
	text   func(string) // run of ordinary characters (and strings) to copy
	interp func(string) // an interpolation `{...}` group, copied verbatim
	// at, when set, is told the rune index in the scanned source where the
	// next text or interp run begins, just before that handler runs. It
	// lets bracesToIndent recover the source column of each statement it
	// re-emits (the text handlers carry no position of their own).
	at      func(i int)
	open    func()    // a block-opening brace
	close   func()    // a block-closing brace
	semi    func()    // a top-level ';'
	newline func()    // a source newline
	skip    func(int) // newlines consumed silently (inside skipped comments)
	block   func(int, int) bool
}

// scanStructural walks runes outside of strings and comments, classifying braces
// as interpolation vs block delimiters and reporting structural events. Comments
// are skipped (dropped). String literals are passed through via the text handler.
func scanStructural(runes []rune, h scanHandlers) {
	n := len(runes)
	emitText := func(at int, s string) {
		if h.text != nil && s != "" {
			if h.at != nil {
				h.at(at)
			}
			h.text(s)
		}
	}

	// parens is the ( ) nesting depth on the current line. A `;` inside
	// parentheses is text, not a statement end, so an unquoted data URI
	// (`url(data:image/png;base64,…)`) survives. The depth resets at each
	// newline and block brace, so an unbalanced `(` can only swallow the
	// `;` of its own line.
	parens := 0

	for i := 0; i < n; i++ {
		c := runes[i]
		switch {
		case c == '(':
			parens++
			emitText(i, "(")

		case c == ')':
			if parens > 0 {
				parens--
			}
			emitText(i, ")")

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
			emitText(i, string(runes[i:j]))
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
			// Interpolation, object literals (`x = {a: 1}`) and a block
			// mixin's `{block}` slot are copied verbatim; only a block
			// brace is structure.
			if end := matchRuneBrace(runes, i); end >= 0 &&
				(isInterpBrace(runes, i, end) || isObjectBrace(runes, i) || string(runes[i:end+1]) == "{block}") {
				if h.interp != nil {
					if h.at != nil {
						h.at(i)
					}
					h.interp(string(runes[i : end+1]))
				}
				i = end
			} else {
				if h.block != nil {
					if stop := h.block(i, braceEnd(runes, end)); stop {
						return
					}
				}
				parens = 0
				if h.open != nil {
					h.open()
				}
			}

		case c == '}':
			parens = 0
			if h.close != nil {
				h.close()
			}

		case c == ';' && parens > 0:
			emitText(i, ";")

		case c == ';':
			if h.semi != nil {
				h.semi()
			}

		case c == '\n':
			parens = 0
			if h.newline != nil {
				h.newline()
			}

		default:
			emitText(i, string(c))
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
