package parser

import (
	"errors"
	"reflect"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/ast"
)

// Format is `styl fmt`: it rewrites Stylus source into a canonical layout
// without changing what it means.
//
// Design. The AST can't be printed back as source: the parser drops comments
// before lexing (stripComments) and keeps no record of how an expression was
// spelled. So Format never prints the AST. It works on the source lines and
// only rewrites whitespace, using the parser's own structure rules to decide
// what each line's indentation should be:
//
//   - indentation becomes two spaces per nesting level (tabs and odd widths
//     are normalized); in brace syntax the level comes from the braces
//   - runs of spaces inside a line collapse to one (strings and comments are
//     left alone)
//   - trailing whitespace goes, blank-line runs collapse to one, leading and
//     trailing blank lines go, and the file ends in exactly one newline
//   - comments are kept, re-indented with the code around them
//   - statements are respelled one rule per kind (respell.go): `prop: value`
//     spacing and one declaration form per file, ` = ` in assignments and
//     parameter defaults, `, ` in values
//
// Safety. Whatever the rewrite does, the result is parsed and its AST compared
// with the original's (positions ignored). A mismatch means the re-indent
// guessed wrong on some unusual layout, so Format falls back to the
// whitespace-only pass (trailing spaces and blank lines), and to returning an
// error if even that changes the AST. A formatter that silently changes a
// stylesheet would be worse than none.
//
//	src ──Parse──▶ AST₀
//	 │
//	 ├─ tidy ──▶ reindent ──Parse──▶ AST₁ ── AST₁ ≡ AST₀ ? ──yes──▶ respell ─▶ result
//	 │                                              │ no
//	 └─ tidy ─────────────────Parse──▶ AST₂ ── AST₂ ≡ AST₀ ? ──yes──▶ respell ─▶ result
//	                                                │ no
//	                                                ▼
//	                                         ErrFormatUnsafe
//
// Source that doesn't parse is not formatted: the parse error is returned.
func Format(src string) (string, error) {
	orig, err := Parse(src)
	if err != nil {
		return "", err
	}
	tidied := tidy(src)
	for _, cand := range []string{reindent(tidied), tidied} {
		sheet, err := Parse(cand)
		if err == nil && sameAST(orig, sheet) {
			// Statement spelling (respell.go) runs on whichever layout
			// passed, with its own per-line guard.
			return respellSafe(cand, orig), nil
		}
	}
	return "", ErrFormatUnsafe
}

// ErrFormatUnsafe reports that no formatting of the source kept its AST, so
// the source was left as it was.
var ErrFormatUnsafe = errors.New("styl fmt: formatting would change the stylesheet's meaning; left unchanged")

// indentUnit is the output indentation per nesting level.
const indentUnit = 2

// tidy is the whitespace-only pass: CRLF → LF, trailing whitespace removed,
// blank-line runs collapsed to one, no blank lines at the start or end, and a
// single final newline. None of it can change structure: the line-tree builder
// skips blank lines and ignores trailing whitespace, and the brace scanner
// treats both as plain separators.
func tidy(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, l := range lines {
		l = strings.TrimRight(l, " \t\r")
		// Blank by classifyLines' test (TrimSpace), so a line of other
		// whitespace (`\f`) isn't kept here and then emptied by reindent,
		// which would take a second run to settle.
		if strings.TrimSpace(l) == "" {
			blank = len(out) > 0 // never keep a leading blank line
			continue
		}
		if blank {
			out = append(out, "")
			blank = false
		}
		out = append(out, l)
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

// lineKind classifies a source line for the re-indent pass.
type lineKind int

const (
	lkBlank     lineKind = iota
	lkContent            // a statement line (has code once comments are stripped)
	lkComment            // only a comment (starts outside any block comment)
	lkBlockCont          // begins inside a multi-line /* */ comment
	lkObjCont            // inside a multi-line object literal, after its opener line
	lkCommaCont          // continues a statement whose previous line ends in ','
	lkVerbatim           // code preceded by a comment: its indentation is left alone
)

// fmtLine is the per-line state of the re-indent pass.
type fmtLine struct {
	raw    string // source line (tidied, so no trailing whitespace)
	clean  string // the same line with comments stripped
	kind   lineKind
	oldInd int // source indentation width of raw (tabs expanded)
	anchor int // lkObjCont/lkCommaCont: index of the statement's first line
	newInd int // output indentation width, set by the mode-specific pass
}

// reindent rewrites each line's leading whitespace to its nesting depth and
// collapses internal whitespace runs. src must already be tidied.
func reindent(src string) string {
	if src == "" {
		return src
	}
	lines := classifyLines(src)
	if usesBraces(joinObjectLiterals(src)) {
		indentBraces(lines, src)
	} else {
		indentTree(lines)
	}
	shiftBlockComments(lines)

	var b strings.Builder
	for i := range lines {
		l := &lines[i]
		switch l.kind {
		case lkBlank:
		case lkVerbatim:
			b.WriteString(l.raw)
		case lkBlockCont:
			// Comment bodies keep their own text; only the indentation moves.
			b.WriteString(strings.Repeat(" ", max(l.newInd, 0)))
			_, rest := splitIndent(l.raw)
			b.WriteString(rest)
		default:
			b.WriteString(strings.Repeat(" ", l.newInd))
			_, rest := splitIndent(l.raw)
			if l.kind != lkComment {
				rest = collapseSpaces(rest)
			}
			b.WriteString(rest)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// classifyLines splits src into lines and assigns each its kind. The comment
// states come from a scan that mirrors stripComments (so a `//` in an unquoted
// URL, or delimiters in a string, are treated exactly as the parser treats
// them), and object-literal bodies from the same opener rule the parser's
// joinObjectLiterals uses.
func classifyLines(src string) []fmtLine {
	rawLines := strings.Split(strings.TrimSuffix(src, "\n"), "\n")
	cleanLines := strings.Split(strings.TrimSuffix(stripComments(src), "\n"), "\n")
	startsInBlock := blockCommentStarts(src, len(rawLines))
	objAnchor := objectLiteralLines(stripComments(src), len(rawLines))

	lines := make([]fmtLine, len(rawLines))
	commaAnchor := -1 // first line of a statement still continuing via ','
	for i, raw := range rawLines {
		clean := ""
		if i < len(cleanLines) {
			clean = cleanLines[i]
		}
		oldInd, _ := splitIndent(raw + "x")
		l := fmtLine{raw: raw, clean: clean, oldInd: oldInd, anchor: -1}
		cleanInd, cleanText := splitIndent(clean)

		switch {
		case strings.TrimSpace(raw) == "":
			l.kind = lkBlank
		case startsInBlock[i]:
			l.kind = lkBlockCont
		case objAnchor[i] >= 0:
			l.kind, l.anchor = lkObjCont, objAnchor[i]
		case cleanText == "":
			l.kind = lkComment
		case cleanInd != oldInd:
			// A leading comment (`/* x */ color red`): the parser measures the
			// indentation after removing the comment, so rewriting the raw
			// indentation would shift the code's measured depth.
			l.kind = lkVerbatim
		case commaAnchor >= 0:
			l.kind, l.anchor = lkCommaCont, commaAnchor
		default:
			l.kind = lkContent
		}

		// A trailing comma carries the statement onto the next code line
		// (buildTree's `cont`), whatever that line's indentation.
		if cleanText != "" && l.kind != lkBlockCont && l.kind != lkObjCont {
			if strings.HasSuffix(cleanText, ",") {
				if commaAnchor < 0 {
					commaAnchor = i
				}
			} else {
				commaAnchor = -1
			}
		}
		lines[i] = l
	}
	return lines
}

// blockCommentStarts reports, for each of n lines, whether the line begins
// inside a /* */ comment opened on an earlier line. The scan follows
// stripComments' rules for strings and `//`.
func blockCommentStarts(src string, n int) []bool {
	starts := make([]bool, n)
	runes := []rune(src)
	inBlock, inLine := false, false
	var quote rune
	line := 0
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if c == '\n' {
			line++
			inLine, quote = false, 0 // strings don't span lines
			if line < n {
				starts[line] = inBlock
			}
			continue
		}
		switch {
		case inLine:
		case inBlock:
			if c == '*' && i+1 < len(runes) && runes[i+1] == '/' {
				inBlock = false
				i++
			}
		case quote != 0:
			if c == '\\' && i+1 < len(runes) && runes[i+1] != '\n' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '/' && i+1 < len(runes) && runes[i+1] == '*':
			inBlock = true
			i++
		case c == '/' && i+1 < len(runes) && runes[i+1] == '/' &&
			(i == 0 || isSpace(runes[i-1]) || runes[i-1] == '\n'):
			inLine = true
		}
	}
	return starts
}

// objectLiteralLines maps each line inside a multi-line object literal (after
// the line holding its `{`, through the line holding its `}`) to the index of
// that opener line; other lines map to -1. clean is comment-stripped source,
// scanned with joinObjectLiterals' rules.
func objectLiteralLines(clean string, n int) []int {
	anchor := make([]int, n)
	for i := range anchor {
		anchor[i] = -1
	}
	runes := []rune(clean)
	line, lineStart, brackets := 0, 0, 0
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case c == '"' || c == '\'':
			i = skipString(runes, i) - 1
		case c == '[':
			brackets++
		case c == ']':
			if brackets > 0 {
				brackets--
			}
		case c == '\n':
			line++
			lineStart, brackets = i+1, 0
		case c == '{' && brackets == 0 && objectOpener(runes[lineStart:i]):
			end := matchObjectBrace(runes, i)
			if end < 0 {
				continue
			}
			nl := strings.Count(string(runes[i:end+1]), "\n")
			for k := 1; k <= nl && line+k < n; k++ {
				anchor[line+k] = line
			}
			line += nl
			i = end
			// The literal's closing line is still the opener's statement;
			// lineStart moves so a second literal on it is seen correctly.
			for lineStart = i; lineStart > 0 && runes[lineStart-1] != '\n'; lineStart-- {
			}
		}
	}
	return anchor
}

// indentTree assigns output indentation for the indentation syntax, using the
// line-tree builder's rule: a line is a child of the nearest earlier line with
// smaller indentation. Depth, not width, is what the tree records, so any
// consistent re-spacing by depth yields the same tree.
func indentTree(lines []fmtLine) {
	type frame struct{ old, depth int }
	var stack []frame
	depthFor := func(ind int, push bool) int {
		s := stack
		for len(s) > 0 && s[len(s)-1].old >= ind {
			s = s[:len(s)-1]
		}
		d := len(s)
		if push {
			stack = append(s, frame{old: ind, depth: d})
		}
		return d
	}
	for i := range lines {
		l := &lines[i]
		switch l.kind {
		case lkContent:
			l.newInd = depthFor(l.oldInd, true) * indentUnit
		case lkComment:
			// A comment sits where the code at its indentation would sit,
			// but doesn't open a level (the tree never sees it).
			l.newInd = depthFor(l.oldInd, false) * indentUnit
		case lkObjCont, lkCommaCont:
			l.newInd = relativeIndent(lines, i)
		case lkVerbatim:
			// Unmoved; it still enters the tree at its measured depth.
			ind, _ := splitIndent(l.clean)
			depthFor(ind, true)
			l.newInd = l.oldInd
		}
	}
}

// relativeIndent keeps a continuation line's offset from its statement's
// first line (clamped at zero), so hand-aligned value lists and object bodies
// move with their statement instead of being flattened.
func relativeIndent(lines []fmtLine, i int) int {
	a := lines[lines[i].anchor]
	return a.newInd + max(lines[i].oldInd-a.oldInd, 0)
}

// indentBraces assigns output indentation for sources using brace syntax.
// Inside braces a line sits one level below the header of the innermost open
// `{` (and a line starting with `}` level with that header). Outside all
// braces indentation is structural in a mixed file (bracesToIndent resolves
// it with its own stack), so those lines keep their width, and a block opened
// on one takes its width as the header level.
//
// Brace events come from scanStructural, the same scanner bracesToIndent
// runs, so interpolation and object-literal braces are never counted.
func indentBraces(lines []fmtLine, src string) {
	events := braceEvents(src)

	// Outside all braces, a line's depth comes from its source indentation
	// by bracesToIndent's rule: pop entries at least as indented, then sit
	// one level below the top, or level with it when the top is a header
	// whose block was braced (it never adopts indentation children).
	type entry struct {
		old, depth int
		braced     bool
	}
	var stack []entry
	outside := func(ind int, push bool) int {
		s := stack
		for len(s) > 0 && s[len(s)-1].old >= ind {
			s = s[:len(s)-1]
		}
		d := 0
		if n := len(s); n > 0 {
			d = s[n-1].depth
			if !s[n-1].braced {
				d++
			}
		}
		if push {
			stack = append(s, entry{old: ind, depth: d})
		}
		return d
	}
	lastOutside := -1 // stack index of the last outside line placed, if on top

	var headers []int // output indentation of each open block's header
	ev := 0
	for i := range lines {
		l := &lines[i]
		// All whitespace, not just splitIndent's: a `\r}` line closes a
		// block like `}` does, and collapseSpaces drops that `\r`.
		text := strings.TrimSpace(l.raw)
		atTop := len(headers) == 0
		switch l.kind {
		case lkContent, lkComment:
			switch {
			case atTop && l.kind == lkContent && strings.HasPrefix(text, "{") && lastOutside >= 0:
				// A `{` on its own line opens the previous statement's
				// block and sits at that header's level.
				l.newInd = stack[lastOutside].depth * indentUnit
			case atTop && l.kind == lkContent:
				l.newInd = outside(l.oldInd, true) * indentUnit
				lastOutside = len(stack) - 1
			case atTop:
				l.newInd = outside(l.oldInd, false) * indentUnit
			case strings.HasPrefix(text, "}"):
				l.newInd = headers[len(headers)-1]
			default:
				l.newInd = headers[len(headers)-1] + indentUnit
			}
		case lkObjCont, lkCommaCont:
			l.newInd = relativeIndent(lines, i)
		case lkVerbatim:
			// Unmoved; outside braces it still enters the stack at the
			// indentation the parser measures (after its leading comment).
			l.newInd = l.oldInd
			if atTop {
				ind, _ := splitIndent(l.clean)
				outside(ind, true)
				lastOutside = len(stack) - 1
			}
		default:
			l.newInd = l.oldInd
		}

		// Apply this line's brace events. Each `{` records the level of its
		// header: this line's indentation, one level deeper per block already
		// opened (and not closed) earlier on the same line.
		openedHere := 0
		for ; ev < len(events) && events[ev].line == i; ev++ {
			if events[ev].open && len(headers) == 0 && lastOutside >= 0 && lastOutside < len(stack) {
				stack[lastOutside].braced = true
			}
			if events[ev].open {
				headers = append(headers, l.newInd+openedHere*indentUnit)
				openedHere++
			} else {
				if len(headers) > 0 {
					headers = headers[:len(headers)-1]
				}
				if openedHere > 0 {
					openedHere--
				}
			}
		}
	}
}

// braceEvent is a structural `{` (open) or `}` on a 0-based source line.
type braceEvent struct {
	line int
	open bool
}

// braceEvents lists src's block braces in order. They come from
// scanStructural, the same scanner bracesToIndent runs, so interpolation and
// object-literal braces are never counted.
func braceEvents(src string) []braceEvent {
	var events []braceEvent
	line := 0
	scanStructural([]rune(src), scanHandlers{
		text:    func(s string) { line += strings.Count(s, "\n") },
		interp:  func(s string) { line += strings.Count(s, "\n") },
		open:    func() { events = append(events, braceEvent{line, true}) },
		close:   func() { events = append(events, braceEvent{line, false}) },
		newline: func() { line++ },
		skip:    func(n int) { line += n },
	})
	return events
}

// braceLineStarts reports, for each of n lines, whether the line begins
// inside a brace block.
func braceLineStarts(src string, n int) []bool {
	out := make([]bool, n)
	depth, ev := 0, 0
	events := braceEvents(src)
	for i := 0; i < n; i++ {
		out[i] = depth > 0
		for ; ev < len(events) && events[ev].line == i; ev++ {
			if events[ev].open {
				depth++
			} else if depth > 0 {
				depth--
			}
		}
	}
	return out
}

// shiftBlockComments moves the continuation lines of each multi-line /* */
// comment by the same amount its opening line moved, preserving the
// comment's internal layout.
func shiftBlockComments(lines []fmtLine) {
	delta := 0
	for i := range lines {
		l := &lines[i]
		if l.kind == lkBlank {
			continue
		}
		if l.kind == lkBlockCont {
			l.newInd = max(l.oldInd+delta, 0)
			continue
		}
		delta = l.newInd - l.oldInd
	}
}

// collapseSpaces turns each run of spaces/tabs in a line's code into a single
// space, leaving string literals and any trailing comment (with the gap
// before it, so aligned comments stay aligned) untouched.
func collapseSpaces(s string) string {
	runes := []rune(s)
	var b strings.Builder
	space := false
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if c == '/' && i+1 < len(runes) &&
			(runes[i+1] == '*' || (runes[i+1] == '/' && i > 0 && isSpace(runes[i-1]))) {
			if space {
				// Restore the original gap before the comment.
				j := i
				for j > 0 && isSpace(runes[j-1]) {
					j--
				}
				b.WriteString(string(runes[j:i]))
			}
			b.WriteString(string(runes[i:]))
			return b.String()
		}
		if isSpace(c) {
			space = true
			continue
		}
		if space {
			// Whitespace the indentation split left at the start (a lone
			// `\r`, which splitIndent doesn't count as indentation) is
			// dropped: written as a space, the next run would read it as
			// indentation, and fmt wouldn't be idempotent.
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
		}
		if c == '"' || c == '\'' {
			j := skipString(runes, i)
			b.WriteString(string(runes[i:j]))
			i = j - 1
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}

// sameAST reports whether two parses describe the same stylesheet: equal
// node for node, ignoring source positions (Line/Col and a declaration's
// ValueLine/ValueCol fields) and, in the raw
// source-text fields the parser keeps verbatim (selectors, at-rule params,
// @extend targets), ignoring whitespace-run differences that collapseSpaces
// may introduce.
func sameAST(a, b *ast.Stylesheet) bool {
	return sameValue(reflect.ValueOf(a), reflect.ValueOf(b), "")
}

func sameValue(a, b reflect.Value, field string) bool {
	if a.Kind() != b.Kind() || a.Type() != b.Type() {
		return false
	}
	switch a.Kind() {
	case reflect.Pointer, reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		return sameValue(a.Elem(), b.Elem(), field)
	case reflect.Struct:
		t := a.Type()
		for i := 0; i < t.NumField(); i++ {
			name := t.Field(i).Name
			switch name {
			case "Line", "Col", "ValueLine", "ValueCol":
				continue
			}
			if !sameValue(a.Field(i), b.Field(i), name) {
				return false
			}
		}
		return true
	case reflect.Slice:
		if a.Len() != b.Len() {
			return false
		}
		for i := 0; i < a.Len(); i++ {
			if !sameValue(a.Index(i), b.Index(i), field) {
				return false
			}
		}
		return true
	case reflect.String:
		switch field {
		case "Selectors", "Params", "Target":
			return strings.Join(strings.Fields(a.String()), " ") ==
				strings.Join(strings.Fields(b.String()), " ")
		}
		return a.String() == b.String()
	default:
		return reflect.DeepEqual(a.Interface(), b.Interface())
	}
}
