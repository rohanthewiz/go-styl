package parser

import "strings"

// Stacked selector lines. In the indentation syntax, several selector lines
// at one level can share the block of the last one:
//
//	.x
//	  &:after          ┐
//	  &:before         ├─ one ruleset: ".x:after, .x:before"
//	    content ''     ┘
//
// The line-tree builder sees `&:after` as a childless leaf and `&:before` as a
// block line, so on its own the parser would read the leaf as a declaration
// (or fail on it). groupSelectorLines folds such runs into a single block line
// before the siblings are classified.
//
// Stylus decides this with token lookahead (Parser#looksLikeSelector): a line
// that looks like a selector takes in the following lines until one with an
// indented body. Here the same decision is made on whole lines, and a run
// is folded only when every leaf in it looks like a selector and the run ends
// in a ruleset-shaped block line. A run that contains a declaration-shaped
// line (`color red`) is left alone. Stylus would glue it into the selector and
// output nonsense like `.x color red`, and not folding keeps the declaration
// that was clearly meant.
//
// One case follows Stylus even though it may surprise: a bare identifier
// directly above a selector block is a type selector, not a mixin call, even
// when a mixin of that name exists (`m` over `.y` gives `.x m, .x .y`).
// Stylus 0.64 behaves this way, and matching it keeps output byte-for-byte
// compatible.

// groupSelectorLines returns lines with every run of stacked selector leaves
// merged into the block line that ends the run. The merged line keeps the
// first leaf's position (for errors and source maps) and the block line's
// children. lines is not modified; the input is returned as-is when nothing
// merges.
func groupSelectorLines(lines []*line) []*line {
	var out []*line
	merged := false

	for i := 0; i < len(lines); i++ {
		ln := lines[i]
		if len(ln.children) > 0 || !looksLikeSelectorLine(ln.text) {
			out = append(out, ln)
			continue
		}

		// Scan forward over selector-shaped leaves to the run's end.
		j := i + 1
		for j < len(lines) && len(lines[j].children) == 0 && looksLikeSelectorLine(lines[j].text) {
			j++
		}
		if j >= len(lines) || !isRuleSetHeader(lines[j]) {
			// No block to share: not a selector group. Leave the leaf for
			// parseLine, which reads it as a declaration or call, or reports
			// an error.
			out = append(out, ln)
			continue
		}

		parts := make([]string, 0, j-i+1)
		for k := i; k <= j; k++ {
			parts = append(parts, lines[k].text)
		}
		out = append(out, &line{
			text:     strings.Join(parts, ", "),
			indent:   ln.indent,
			lineNo:   ln.lineNo,
			children: lines[j].children,
		})
		merged = true
		i = j
	}

	if !merged {
		return lines
	}
	return out
}

// isRuleSetHeader reports whether a block line would be parsed as a ruleset,
// as opposed to an at-rule, control flow, or a mixin/function definition
// (the other block forms parseBlock/parseLine recognize).
func isRuleSetHeader(ln *line) bool {
	if len(ln.children) == 0 {
		return false
	}
	text := ln.text
	if strings.HasPrefix(text, "@") {
		return false
	}
	for _, kw := range []string{"if", "unless", "else", "for"} {
		if wordPrefix(text, kw) {
			return false
		}
	}
	if toks, err := lexLine(text, ln.lineNo); err == nil {
		if _, _, rest, ok := callSignature(toks); ok && onlyEOF(rest) {
			return false // `name(params)` with a body is a definition
		}
	}
	return true
}

// looksLikeSelectorLine reports whether a childless line can only be (or,
// for a bare identifier, is taken to be) a selector when a selector block
// follows it. It mirrors the line-level cues of Stylus's looksLikeSelector:
//
//   - a leading combinator or selector sigil: `.a`, `#id`, `&:hover`,
//     `> .a`, `+ .b`, `~ .c`, `[type=text]`, `:hover`, `*`
//   - a bare identifier: `a`, `input`
//   - an identifier glued to a class, id, attribute, or pseudo part:
//     `a.b`, `a#c`, `input[x]`, `a::before`, `td:nth-child(1)`, `a:hover`
//
// A glued single colon counts only when a known pseudo-class or pseudo-element
// follows it (Stylus's list), so declarations such as `display:block` or
// `cursor:default` are not taken for selectors.
func looksLikeSelectorLine(text string) bool {
	if text == "" {
		return false
	}
	switch c := text[0]; c {
	case '&', '>', '~', '[', ':':
		return true
	case '.', '#':
		// `.5` or `#` + digit are not selectors in statement position.
		return len(text) > 1 && !isDigitByte(text[1])
	case '*', '+':
		// `+name` is an explicit mixin call and `*zoom 1` an IE hack
		// declaration; the combinator/universal forms are not followed by a
		// letter.
		return len(text) == 1 || !isLetterByte(text[1])
	}

	if !isLetterByte(text[0]) {
		return false
	}
	// Leading identifier.
	i := 0
	for i < len(text) && isIdentByte(text[i]) {
		i++
	}
	ident := text[:i]
	if i == len(text) {
		switch ident {
		case "if", "unless", "else", "for", "return":
			return false
		}
		return true // bare identifier: a type selector
	}

	switch text[i] {
	case '.', '#', '[':
		return true
	case ':':
		if i+1 < len(text) && text[i+1] == ':' {
			return true // pseudo-element
		}
		j := i + 1
		for j < len(text) && isIdentByte(text[j]) {
			j++
		}
		return pseudoSelectors[text[i+1:j]]
	}
	return false
}

// pseudoSelectors is Stylus 0.64's list of pseudo-classes and pseudo-elements
// that mark `ident:name` as a selector rather than a `property:value`.
var pseudoSelectors = map[string]bool{
	"is": true, "has": true, "where": true, "not": true,
	"dir": true, "lang": true,
	"any-link": true, "link": true, "visited": true, "local-link": true, "target": true, "scope": true,
	"hover": true, "active": true, "focus": true, "drop": true,
	"current": true, "past": true, "future": true,
	"enabled": true, "disabled": true, "read-only": true, "read-write": true,
	"placeholder-shown": true, "checked": true, "indeterminate": true, "valid": true,
	"invalid": true, "in-range": true, "out-of-range": true, "required": true,
	"optional": true, "user-error": true,
	"root": true, "empty": true, "blank": true,
	"nth-child": true, "nth-last-child": true, "first-child": true, "last-child": true,
	"only-child": true, "nth-of-type": true, "nth-last-of-type": true,
	"first-of-type": true, "last-of-type": true, "only-of-type": true,
	"nth-match": true, "nth-last-match": true,
	"nth-column": true, "nth-last-column": true,
	"first-line": true, "first-letter": true, "before": true, "after": true,
	"selection": true,
}

func isDigitByte(c byte) bool  { return c >= '0' && c <= '9' }
func isLetterByte(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isIdentByte(c byte) bool {
	return isLetterByte(c) || isDigitByte(c) || c == '_' || c == '-'
}
