package parser

import (
	"strings"

	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/token"
)

// Statement spelling: the second half of `styl fmt`, run on the re-indented
// text. Where the first half only moves whitespace at line starts, this
// rewrites how a statement is spelled, one rule per statement kind:
//
//	rule                      before                 after
//	declaration colon         color:red              color: red
//	  (one form per file)     width 1px  ┐ majority  width: 1px
//	                          color: red ┘ "prop: v" color: red
//	assignment operator       x=1   y ?=2            x = 1   y ?= 2
//	parameter defaults        m(a=1,b = 2)           m(a = 1, b = 2)
//	commas in values          f(1 ,2,3)              f(1, 2, 3)
//
// Which statement a line holds comes from the parse, never from guessing at
// the text: a line is respelled only when exactly one statement starts on
// it, and the rule for that statement's kind knows which part of the line is
// its name, its operator and its value. Strings, raw url(...) tokens,
// selectors and at-rule params are never touched.
//
// Declaration form. Stylus accepts `prop value` and `prop: value`, and a
// file is usually written one way. Each file is made consistent: inside
// braces it's always `prop: value` (the CSS spelling), and in indentation
// syntax the form most of the file's declarations already use wins. A tie
// leaves every declaration as written, so the rule never flips a file that
// has no clear style.
//
// Safety. The respelled text goes through the same AST guard as the
// re-indent (Format). If it fails, the edits are applied one line at a time
// and each is kept only if the AST still matches, so one odd line (a
// colon-less declaration that would read as a selector) loses only its own
// respelling. (The parser is robust enough that no such line is known: a
// colon-less declaration still reads as one. The fallback is there so that
// one never costs the whole file.)

// respellSafe returns src with every statement respelled whose change keeps
// the AST equal to orig.
func respellSafe(src string, orig *ast.Stylesheet) string {
	return applyGuarded(src, orig, respellEdits(src))
}

// applyGuarded applies edits to src, all at once when the result keeps
// orig's AST, else one by one, keeping each that does.
func applyGuarded(src string, orig *ast.Stylesheet, edits []lineEdit) string {
	if len(edits) == 0 {
		return src
	}
	lines := strings.Split(src, "\n")
	ok := func(ls []string) bool {
		sheet, err := Parse(strings.Join(ls, "\n"))
		return err == nil && sameAST(orig, sheet)
	}
	all := append([]string(nil), lines...)
	for _, e := range edits {
		all[e.line] = e.text
	}
	if ok(all) {
		return strings.Join(all, "\n")
	}
	// maxGreedyEdits bounds the one-parse-per-edit fallback on huge files.
	const maxGreedyEdits = 2000
	if len(edits) > maxGreedyEdits {
		return src
	}
	for _, e := range edits {
		prev := lines[e.line]
		lines[e.line] = e.text
		if !ok(lines) {
			lines[e.line] = prev
		}
	}
	return strings.Join(lines, "\n")
}

// lineEdit replaces line (0-based) with text.
type lineEdit struct {
	line int
	text string
}

// respellEdits computes the respelled text of every line that changes.
func respellEdits(src string) []lineEdit {
	sheet, err := Parse(src)
	if err != nil || src == "" {
		return nil
	}
	lines := classifyLines(src)
	inBrace := make([]bool, len(lines))
	if usesBraces(joinObjectLiterals(src)) {
		inBrace = braceLineStarts(src, len(lines))
	}

	// Statements by their 0-based start line.
	byLine := map[int][]ast.Stmt{}
	var walk func(stmts []ast.Stmt)
	walk = func(stmts []ast.Stmt) {
		for _, s := range stmts {
			if l, _ := ast.Pos(s); l > 0 {
				byLine[l-1] = append(byLine[l-1], s)
			}
			for _, b := range stmtBodies(s) {
				walk(b)
			}
		}
	}
	walk(sheet.Statements)

	// only returns the single statement on line i, if its line can be
	// respelled at all.
	only := func(i int) ast.Stmt {
		if lines[i].kind != lkContent || len(byLine[i]) != 1 {
			return nil
		}
		return byLine[i][0]
	}

	// The file's declaration form in indentation syntax.
	colons, bare := 0, 0
	for i := range lines {
		if d, ok := only(i).(*ast.Declaration); ok && !inBrace[i] {
			_, code, _ := splitCode(lines[i].raw)
			if c, ok := declColon(code, d); ok {
				if c {
					colons++
				} else {
					bare++
				}
			}
		}
	}
	var indentForm *bool // nil: keep as written
	if colons != bare {
		f := colons > bare
		indentForm = &f
	}

	var out []lineEdit
	valueStmt := map[int]bool{} // statement lines whose continuation lines are values
	for i := range lines {
		l := lines[i]
		ind, code, comment := splitCode(l.raw)
		var next string
		switch l.kind {
		case lkContent:
			s := only(i)
			if s == nil {
				continue
			}
			form := indentForm
			if inBrace[i] {
				t := true
				form = &t
			}
			var ok bool
			next, ok = respellStmt(code, s, form)
			if !ok {
				continue
			}
			switch s.(type) {
			case *ast.Declaration, *ast.Assignment, *ast.MemberAssign, *ast.MixinCall, *ast.Return, *ast.ExprStmt:
				valueStmt[i] = true
			}
		case lkCommaCont:
			// A continuation line of a multi-line value list.
			if !valueStmt[l.anchor] {
				continue
			}
			next = commaSpace(code)
		default:
			continue
		}
		if text := ind + next + comment; text != l.raw {
			out = append(out, lineEdit{line: i, text: text})
		}
	}
	return out
}

// stmtBodies returns a statement's nested statement lists.
func stmtBodies(s ast.Stmt) [][]ast.Stmt {
	switch n := s.(type) {
	case *ast.RuleSet:
		return [][]ast.Stmt{n.Body}
	case *ast.FuncDef:
		return [][]ast.Stmt{n.Body}
	case *ast.AtRule:
		return [][]ast.Stmt{n.Body}
	case *ast.MixinCall:
		return [][]ast.Stmt{n.Block}
	case *ast.For:
		return [][]ast.Stmt{n.Body}
	case *ast.If:
		out := [][]ast.Stmt{n.Else}
		for _, b := range n.Branches {
			out = append(out, b.Body)
		}
		return out
	}
	return nil
}

// respellStmt respells one statement's code (the line without indentation
// or trailing comment). form is the declaration form to use (true for
// `prop: value`), nil to keep a declaration's own. ok is false when the line
// isn't spelled the way the rule expects, and is left alone.
func respellStmt(code string, s ast.Stmt, form *bool) (string, bool) {
	switch n := s.(type) {
	case *ast.Declaration:
		colon, ok := declColon(code, n)
		if !ok {
			return "", false
		}
		if form != nil {
			colon = *form
		}
		value := strings.TrimLeft(code[len(n.Property):], " \t")
		value = strings.TrimLeft(strings.TrimPrefix(value, ":"), " \t")
		if colon {
			return n.Property + ": " + commaSpace(value), true
		}
		return n.Property + " " + commaSpace(value), true
	case *ast.Assignment:
		if !strings.HasPrefix(code, n.Name) {
			return "", false
		}
		op := "="
		if n.Op == token.ASSIGNQ {
			op = "?="
		}
		rest := strings.TrimLeft(code[len(n.Name):], " \t")
		if !strings.HasPrefix(rest, op) || strings.HasPrefix(rest[len(op):], "=") {
			return "", false
		}
		return n.Name + " " + op + " " + commaSpace(strings.TrimLeft(rest[len(op):], " \t")), true
	case *ast.MemberAssign:
		start, end := assignOp(code)
		if start < 0 {
			return "", false
		}
		return strings.TrimRight(code[:start], " \t") + " " + code[start:end] + " " +
			commaSpace(strings.TrimLeft(code[end:], " \t")), true
	case *ast.FuncDef:
		// Rune indexes throughout: matchParen counts runes.
		runes := []rune(code)
		open := strings.IndexRune(string(runes), '(')
		if open < 0 || !strings.HasPrefix(code, n.Name) {
			return "", false
		}
		open = len([]rune(code[:open]))
		close := matchParen(code, open)
		if close < 0 {
			return "", false
		}
		return string(runes[:open+1]) + paramSpace(string(runes[open+1:close])) + string(runes[close:]), true
	case *ast.MixinCall, *ast.Return, *ast.ExprStmt, *ast.For, *ast.If:
		return commaSpace(code), true
	}
	return "", false
}

// declColon reports whether a declaration line is spelled `prop: value`
// (true) or `prop value` (false); ok is false for a line the rule leaves
// alone (an interpolated property, or a spelling it doesn't recognize).
func declColon(code string, d *ast.Declaration) (colon, ok bool) {
	if strings.Contains(d.Property, "{") || !strings.HasPrefix(code, d.Property) {
		return false, false
	}
	rest := code[len(d.Property):]
	trimmed := strings.TrimLeft(rest, " \t")
	if strings.HasPrefix(trimmed, ":") {
		return true, strings.TrimSpace(trimmed[1:]) != ""
	}
	// Without a colon, a space must separate the property from the value.
	return false, trimmed != rest && trimmed != ""
}

// splitCode splits a line into its indentation, its code, and a trailing
// comment with the gap before it (so aligned comments stay aligned). The
// comment rules are collapseSpaces'.
func splitCode(raw string) (indent, code, comment string) {
	body := strings.TrimLeft(raw, " \t")
	indent = raw[:len(raw)-len(body)]
	runes := []rune(body)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if c == '"' || c == '\'' {
			i = skipString(runes, i) - 1
			continue
		}
		if c == '/' && i+1 < len(runes) &&
			(runes[i+1] == '*' || (runes[i+1] == '/' && i > 0 && isSpace(runes[i-1]))) {
			j := i
			for j > 0 && isSpace(runes[j-1]) {
				j--
			}
			return indent, string(runes[:j]), string(runes[j:])
		}
	}
	return indent, body, ""
}

// commaSpace normalizes the spacing around top-level and nested commas in
// value text: none before, one after (none at the end of the text, where a
// trailing comma continues the value on the next line). Strings and raw
// url(...) tokens are copied verbatim.
func commaSpace(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case c == '"' || c == '\'':
			j := skipString(runes, i)
			b.WriteString(string(runes[i:j]))
			i = j - 1
		case isURLStart(runes, i):
			j := matchParen(string(runes), i+3)
			if j < 0 {
				b.WriteString(string(runes[i:]))
				return b.String()
			}
			b.WriteString(string(runes[i : j+1]))
			i = j
		case c == ',':
			trimmed := strings.TrimRight(b.String(), " \t")
			b.Reset()
			b.WriteString(trimmed)
			b.WriteByte(',')
			for i+1 < len(runes) && isSpace(runes[i+1]) {
				i++
			}
			if i+1 < len(runes) {
				b.WriteByte(' ')
			}
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}

// paramSpace is commaSpace for a parameter list, plus ` = ` around each
// default value's `=` (at the list's own depth, so a call in a default is
// left alone).
func paramSpace(s string) string {
	s = commaSpace(s)
	runes := []rune(s)
	var b strings.Builder
	depth := 0
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case c == '"' || c == '\'':
			j := skipString(runes, i)
			b.WriteString(string(runes[i:j]))
			i = j - 1
			continue
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == '=' && depth == 0 && isAssignEq(runes, i):
			trimmed := strings.TrimRight(b.String(), " \t")
			b.Reset()
			b.WriteString(trimmed)
			b.WriteString(" = ")
			for i+1 < len(runes) && isSpace(runes[i+1]) {
				i++
			}
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}

// isAssignEq reports whether the `=` at i is a plain assignment, not part of
// `==`, `!=`, `<=`, `>=` or `?=`.
func isAssignEq(r []rune, i int) bool {
	if i > 0 && strings.ContainsRune("=!<>?:", r[i-1]) {
		return false
	}
	return i+1 >= len(r) || r[i+1] != '='
}

// assignOp finds a member assignment's operator (`=` or `?=`) outside
// strings and brackets, returning its rune span in s (as byte offsets), or
// -1.
func assignOp(s string) (start, end int) {
	runes := []rune(s)
	depth := 0
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case c == '"' || c == '\'':
			i = skipString(runes, i) - 1
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == '=' && depth == 0:
			if i+1 < len(runes) && runes[i+1] == '=' {
				return -1, -1
			}
			from := i
			if i > 0 && runes[i-1] == '?' {
				from = i - 1
			} else if i > 0 && strings.ContainsRune("=!<>:", runes[i-1]) {
				return -1, -1
			}
			return len(string(runes[:from])), len(string(runes[:i+1]))
		}
	}
	return -1, -1
}

// isURLStart reports whether a raw `url(` token starts at i (not glued to a
// longer name).
func isURLStart(r []rune, i int) bool {
	if i+4 > len(r) || !strings.EqualFold(string(r[i:i+4]), "url(") {
		return false
	}
	return i == 0 || !(isIdentRune(r[i-1]))
}

func isIdentRune(c rune) bool {
	return c == '-' || c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// matchParen returns the rune index of the `)` closing the `(` at rune
// index open in s, skipping strings; -1 if unclosed.
func matchParen(s string, open int) int {
	runes := []rune(s)
	depth := 0
	for i := open; i < len(runes); i++ {
		switch c := runes[i]; {
		case c == '"' || c == '\'':
			i = skipString(runes, i) - 1
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
