package parser

import "unicode/utf8"

// Semicolons in the indentation syntax. Stylus treats a top-level `;` as a
// statement separator in indented source too, so a trailing `;` is noise and
// several declarations may share one line:
//
//	.a
//	  margin: 0 auto;               →  margin: 0 auto
//	  color:black; background: #fff →  color:black
//	                                   background: #fff
//
// Brace-syntax sources never get here with a top-level `;`: bracesToIndent
// already splits statements on it. For indented source the split is done on
// the line tree rather than the raw text, at the top of parseBlock, for two
// reasons:
//
//   - A multi-line value joined by a trailing comma (`transition a 1s,` /
//     `b 2s;`) is one line by then, so its closing `;` is found like any other.
//   - The pieces of a line become siblings without disturbing the indentation
//     structure: they share the original line's level, and its indented body
//     (if any) goes to the last piece, as in Stylus (`color red; .b` over an
//     indented `top 0` gives `.a .b { top: 0 }`).

// expandSemicolons returns lines with every line that holds a top-level `;`
// replaced by one line per non-empty statement. The input is returned as-is
// when no line has a top-level `;`; lines themselves are never modified.
func expandSemicolons(lines []*line) []*line {
	var out []*line
	changed := false

	for i, ln := range lines {
		segs := splitSemicolons(ln.text)
		if segs == nil {
			if changed {
				out = append(out, ln)
			}
			continue
		}
		if !changed {
			out = append(out, lines[:i]...)
			changed = true
		}
		if len(segs) == 0 {
			// Only semicolons (`;` or `;;`). A childless one is an empty
			// statement and is dropped; one with a body is kept whole so the
			// parser reports the nonsense rather than losing the body silently.
			if len(ln.children) > 0 {
				out = append(out, ln)
			}
			continue
		}
		for k, sg := range segs {
			// Each piece keeps the source line and gets its own column, so
			// errors point at the statement that caused them. For a line
			// joined from a comma continuation the column of a later piece
			// is an approximation (it counts across the joined text).
			piece := &line{text: sg.text, indent: ln.indent + sg.col, lineNo: ln.lineNo}
			if k == len(segs)-1 {
				piece.children = ln.children
			}
			out = append(out, piece)
		}
	}

	if !changed {
		return lines
	}
	return out
}

// semiSeg is one statement of a line split on `;`: its trimmed text and the
// rune offset of that text within the line.
type semiSeg struct {
	text string
	col  int
}

// splitSemicolons splits text on top-level `;` and returns the trimmed,
// non-empty statements. It returns nil when text has no top-level `;` (the
// common case, so callers can skip allocation).
//
// A `;` is not top-level when it sits inside a string, parentheses, brackets
// or an interpolation brace. The paren rule keeps unquoted data URIs intact
// (`url(data:image/png;base64,…)`), which Stylus itself fails to parse.
// Strings honor backslash escapes, matching the lexer (an escaped quote does
// not end the string).
func splitSemicolons(text string) []semiSeg {
	var segs []semiSeg
	found := false
	var quote rune // 0 when not inside a string
	depth := 0     // (), [] and {} nesting
	start := 0     // byte offset where the current statement begins

	emit := func(end int) {
		s := text[start:end]
		// Trim by hand so the column of the trimmed text is known.
		lead := 0
		for lead < len(s) && (s[lead] == ' ' || s[lead] == '\t') {
			lead++
		}
		trail := len(s)
		for trail > lead && (s[trail-1] == ' ' || s[trail-1] == '\t') {
			trail--
		}
		if trail > lead {
			col := utf8.RuneCountInString(text[:start+lead])
			segs = append(segs, semiSeg{text: s[lead:trail], col: col})
		}
	}

	for i := 0; i < len(text); i++ {
		c := text[i]
		if quote != 0 {
			switch {
			case c == '\\':
				i++ // skip the escaped byte (multi-byte runes can't be quotes)
			case rune(c) == quote:
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = rune(c)
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ';':
			if depth == 0 {
				found = true
				emit(i)
				start = i + 1
			}
		}
	}

	if !found {
		return nil
	}
	emit(len(text))
	if segs == nil {
		segs = []semiSeg{} // only semicolons: non-nil, empty
	}
	return segs
}
