// Package parser turns Stylus source into an AST. It first builds an indentation
// line-tree (comments stripped), then classifies each line as a ruleset,
// declaration, or assignment. Selectors are kept as raw strings; only values and
// conditions are tokenized and parsed as expressions (see expr.go).
package parser

import (
	"strings"

	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/diag"
	"github.com/rohanthewiz/go-styl/internal/lexer"
	"github.com/rohanthewiz/go-styl/internal/token"
)

const tabWidth = 4

// line is a node in the indentation tree.
type line struct {
	text     string // content with leading indentation and comments removed
	indent   int    // indentation width (tabs expanded to tabWidth)
	lineNo   int    // 1-based source line number
	children []*line
}

// Parse parses Stylus source into a Stylesheet AST.
func Parse(src string) (*ast.Stylesheet, error) {
	// A multi-line object literal (`theme = {` … `}`) is folded onto one line
	// first, so neither the brace normalizer nor the indentation tree sees
	// its braces and pair lines as structure.
	src = joinObjectLiterals(src)

	// Brace/semicolon syntax is normalized into the indentation form first.
	if usesBraces(src) {
		src = bracesToIndent(src)
	}

	roots, err := buildTree(src)
	if err != nil {
		return nil, err
	}

	stmts, err := parseBlock(roots)
	if err != nil {
		return nil, err
	}
	return &ast.Stylesheet{Statements: stmts}, nil
}

// buildTree strips comments and assembles the indentation tree's root lines.
func buildTree(src string) ([]*line, error) {
	cleaned := stripComments(src)

	var roots []*line
	// stack holds the current ancestry; stack[0] is a synthetic root.
	root := &line{indent: -1}
	stack := []*line{root}

	// cont is the line whose text ends in a trailing comma, if any. As in
	// Stylus, a trailing comma continues the statement on the next non-blank
	// line whatever that line's indentation, so the next line's content is
	// appended to cont instead of entering the tree:
	//
	//	.a,          →  ".a, .b"  (line 1, indent 0)
	//	  .b         ┘
	//	  color red  →  child of ".a, .b"
	//
	// This covers multi-line selector groups and multi-line value lists
	// (`transition opacity 1s,` / `transform 2s`) in both syntaxes, since
	// bracesToIndent leaves each group line as its own output line.
	var cont *line

	for i, raw := range strings.Split(cleaned, "\n") {
		indent, content := splitIndent(raw)
		if content == "" {
			continue // blank or comment-only line
		}

		if cont != nil {
			cont.text += " " + content
			if !strings.HasSuffix(content, ",") {
				cont = nil
			}
			continue
		}

		ln := &line{text: content, indent: indent, lineNo: i + 1}
		if strings.HasSuffix(content, ",") {
			cont = ln
		}

		// Pop until we find a parent with smaller indentation.
		for len(stack) > 1 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]
		parent.children = append(parent.children, ln)
		stack = append(stack, ln)
	}

	roots = root.children
	return roots, nil
}

// splitIndent returns the indentation width and the trimmed content of a line.
func splitIndent(raw string) (int, string) {
	width := 0
	i := 0
	for i < len(raw) {
		switch raw[i] {
		case ' ':
			width++
		case '\t':
			width += tabWidth - (width % tabWidth)
		default:
			return width, strings.TrimRight(raw[i:], " \t\r")
		}
		i++
	}
	return width, ""
}

// lexLine tokenizes a single line's text (a thin wrapper over the lexer so other
// files in this package need not import it directly).
func lexLine(text string, line int) ([]token.Token, error) {
	return lexer.Lex(text, line)
}

// onlyEOF reports whether toks is just a trailing EOF (i.e. nothing of substance).
func onlyEOF(toks []token.Token) bool {
	return len(toks) == 1 && toks[0].Kind == token.EOF
}

// parseBlock parses a list of sibling lines into statements, grouping
// if/else-if/else chains into single If statements.
func parseBlock(lines []*line) ([]ast.Stmt, error) {
	// Semicolons split first, so a selector run or an if/else chain sees
	// each statement as its own line.
	lines = expandSemicolons(lines)
	lines = groupSelectorLines(lines)
	var stmts []ast.Stmt
	for i := 0; i < len(lines); i++ {
		ln := lines[i]
		if isCondStart(ln.text) {
			stmt, next, err := parseConds(lines, i)
			if err != nil {
				return nil, diag.WrapPos(err, "", ln.lineNo, ln.indent+1)
			}
			stmts = append(stmts, stmt)
			i = next - 1
			continue
		}
		stmt, err := parseLine(ln)
		if err != nil {
			// Anchor errors from the line's lexing/expression parsing (which only
			// know the line) at the line's leading column.
			return nil, diag.WrapPos(err, "", ln.lineNo, ln.indent+1)
		}
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}
	return stmts, nil
}

// parseLine classifies a single line (and its children) into a statement.
func parseLine(ln *line) (ast.Stmt, error) {
	text := ln.text

	// --- Block lines (have an indented body) ---
	if len(ln.children) > 0 {
		if strings.HasPrefix(text, "@") {
			return parseAtRule(ln)
		}
		if wordPrefix(text, "for") {
			return parseFor(ln)
		}
		// Block mixin call: `+name(args)` with an indented body, which is
		// passed to the mixin as its block.
		if strings.HasPrefix(text, "+") {
			if call, ok, err := parseBlockMixinCall(ln); ok || err != nil {
				return call, err
			}
		}
		// Function/mixin definition (block form): `name(params)` with a body.
		if toks, err := lexLine(text, ln.lineNo); err == nil {
			if name, inner, rest, ok := callSignature(toks); ok && onlyEOF(rest) {
				params, err := parseParams(inner, ln.lineNo)
				if err != nil {
					return nil, err
				}
				body, err := parseBlock(ln.children)
				if err != nil {
					return nil, err
				}
				return &ast.FuncDef{Name: name, Params: params, Body: body, Line: ln.lineNo, Col: ln.indent + 1}, nil
			}
		}
		// Otherwise a ruleset; selectors are kept raw (not tokenized).
		body, err := parseBlock(ln.children)
		if err != nil {
			return nil, err
		}
		return &ast.RuleSet{Selectors: splitSelectors(text), Body: body, Line: ln.lineNo, Col: ln.indent + 1}, nil
	}

	// --- Leaf lines ---

	// @extend / @extends <selector-or-$placeholder>
	if wordPrefix(text, "@extend") || wordPrefix(text, "@extends") {
		kw := "@extend"
		if wordPrefix(text, "@extends") {
			kw = "@extends"
		}
		target := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text[len(kw):]), ";"))
		if target == "" {
			return nil, diag.Errorf(ln.lineNo, ln.indent+1, "@extend requires a selector")
		}
		return &ast.Extend{Target: target, Line: ln.lineNo, Col: ln.indent + 1}, nil
	}

	// @import / @require <string | url(...)>. @require shares @import's
	// syntax; the only difference (import once) is applied by the evaluator.
	for _, kw := range []string{"@import", "@require"} {
		if wordPrefix(text, kw) {
			return parseImport(kw, strings.TrimSpace(text[len(kw):]), ln.lineNo, ln.indent+1)
		}
	}

	// Any other leaf at-rule (@charset, @namespace, …) passes through verbatim.
	if strings.HasPrefix(text, "@") {
		return parseAtRule(ln)
	}

	// return [expr]
	if wordPrefix(text, "return") {
		rest := strings.TrimSpace(text[len("return"):])
		if rest == "" {
			return &ast.Return{Line: ln.lineNo, Col: ln.indent + 1}, nil
		}
		toks, err := lexLine(rest, ln.lineNo)
		if err != nil {
			return nil, err
		}
		val, err := parseExpr(toks, ln.lineNo)
		if err != nil {
			return nil, err
		}
		return &ast.Return{Value: val, Line: ln.lineNo, Col: ln.indent + 1}, nil
	}

	toks, err := lexLine(text, ln.lineNo)
	if err != nil {
		return nil, err
	}

	// Explicit mixin call: +name or +name(args)
	if toks[0].Kind == token.PLUS && len(toks) >= 2 && toks[1].Kind == token.IDENT {
		if name, inner, rest, ok := callSignature(toks[1:]); ok && onlyEOF(rest) {
			args, err := parseArgs(inner, ln.lineNo)
			if err != nil {
				return nil, err
			}
			return &ast.MixinCall{Name: name, Args: args, Line: ln.lineNo, Col: ln.indent + 1}, nil
		}
		if len(toks) == 3 && toks[2].Kind == token.EOF {
			return &ast.MixinCall{Name: toks[1].Text, Line: ln.lineNo, Col: ln.indent + 1}, nil
		}
	}

	// Variable assignment: name = expr  or  name ?= expr
	if toks[0].Kind == token.IDENT && len(toks) >= 2 &&
		(toks[1].Kind == token.ASSIGN || toks[1].Kind == token.ASSIGNQ) {
		val, err := parseExpr(toks[2:], ln.lineNo)
		if err != nil {
			return nil, err
		}
		return &ast.Assignment{Name: toks[0].Text, Op: toks[1].Kind, Value: val, Line: ln.lineNo, Col: ln.indent + 1}, nil
	}

	// Object member assignment: obj.key = expr, obj[k] = expr, a.b[c] = expr.
	if ma, ok, err := parseMemberAssign(toks, ln); ok || err != nil {
		return ma, err
	}

	// Single-line function definition: name(params) = expr
	if name, inner, rest, ok := callSignature(toks); ok && len(rest) >= 1 && rest[0].Kind == token.ASSIGN {
		params, err := parseParams(inner, ln.lineNo)
		if err != nil {
			return nil, err
		}
		val, err := parseExpr(rest[1:], ln.lineNo)
		if err != nil {
			return nil, err
		}
		return &ast.FuncDef{Name: name, Params: params, Body: []ast.Stmt{&ast.Return{Value: val, Line: ln.lineNo, Col: ln.indent + 1}}, Line: ln.lineNo, Col: ln.indent + 1}, nil
	}

	// Bare mixin call: a single identifier on its own line.
	if len(toks) == 2 && toks[0].Kind == token.IDENT && toks[1].Kind == token.EOF {
		return &ast.MixinCall{Name: toks[0].Text, Line: ln.lineNo, Col: ln.indent + 1}, nil
	}

	// Mixin call: name(args) consuming the whole line.
	if name, inner, rest, ok := callSignature(toks); ok && onlyEOF(rest) {
		args, err := parseArgs(inner, ln.lineNo)
		if err != nil {
			return nil, err
		}
		return &ast.MixinCall{Name: name, Args: args, Line: ln.lineNo, Col: ln.indent + 1}, nil
	}

	// Declaration: `property value...`, with an optional colon after the property.
	if toks[0].Kind != token.IDENT {
		// Not declaration-shaped: a bare expression statement (`700` or
		// `(x + y) / 2` as a function body's implicit return value).
		if e, exprErr := parseExpr(toks, ln.lineNo); exprErr == nil {
			return &ast.ExprStmt{X: e, Line: ln.lineNo, Col: ln.indent + 1}, nil
		}
		return nil, diag.Errorf(ln.lineNo, ln.indent+1, "expected a property declaration, got %q", text)
	}
	valToks := toks[1:]
	if valToks[0].Kind == token.COLON {
		valToks = valToks[1:]
	}
	valToks, important := stripImportant(valToks)
	val, err := parsePropExpr(valToks, ln.lineNo)
	if err != nil {
		// Not `property value` after all: lines like `n * 2` are a bare
		// expression statement when the whole line parses as one.
		if e, exprErr := parseExpr(toks, ln.lineNo); exprErr == nil {
			return &ast.ExprStmt{X: e, Line: ln.lineNo, Col: ln.indent + 1}, nil
		}
		return nil, err
	}
	return &ast.Declaration{Property: toks[0].Text, Value: val, Important: important, Line: ln.lineNo, Col: ln.indent + 1}, nil
}

// parseBlockMixinCall parses a `+name(args)` header whose indented children
// form the block passed to the mixin. ok is false when the header is not
// shaped like a call, and the line is then a ruleset. The parentheses are
// required and the name must touch the '+': a nested `+ li` or `+li` is an
// adjacent-sibling selector, not a call.
func parseBlockMixinCall(ln *line) (ast.Stmt, bool, error) {
	toks, err := lexLine(ln.text[1:], ln.lineNo)
	if err != nil || len(toks) < 3 || toks[0].SpaceBefore {
		return nil, false, nil
	}
	name, inner, rest, ok := callSignature(toks)
	if !ok || !onlyEOF(rest) {
		return nil, false, nil
	}
	args, err := parseArgs(inner, ln.lineNo)
	if err != nil {
		return nil, true, err
	}
	body, err := parseBlock(ln.children)
	if err != nil {
		return nil, true, err
	}
	return &ast.MixinCall{Name: name, Args: args, Block: body, Line: ln.lineNo, Col: ln.indent + 1}, true, nil
}

// parseMemberAssign recognizes `name(.key | [expr])+ (= | ?=) expr`. ok is
// false when toks don't have that shape; err reports a malformed target or
// value once the shape matched.
func parseMemberAssign(toks []token.Token, ln *line) (ast.Stmt, bool, error) {
	if len(toks) < 4 || toks[0].Kind != token.IDENT {
		return nil, false, nil
	}
	if k := toks[1]; k.Kind != token.DOT && (k.Kind != token.LBRACKET || k.SpaceBefore) {
		return nil, false, nil
	}
	// The first '=' or '?=' outside brackets and parentheses splits target
	// from value.
	depth := 0
	eq := -1
	for i, t := range toks {
		switch t.Kind {
		case token.LBRACKET, token.LPAREN, token.LBRACE:
			depth++
		case token.RBRACKET, token.RPAREN, token.RBRACE:
			depth--
		case token.ASSIGN, token.ASSIGNQ:
			if depth == 0 && eq < 0 {
				eq = i
			}
		}
	}
	if eq < 0 {
		return nil, false, nil
	}
	target, err := parseExpr(append(toks[:eq:eq], token.Token{Kind: token.EOF, Line: ln.lineNo}), ln.lineNo)
	if err != nil {
		return nil, true, err
	}
	switch target.(type) {
	case *ast.Member, *ast.Index:
	default:
		return nil, true, diag.Errorf(ln.lineNo, ln.indent+1, "invalid assignment target")
	}
	val, err := parseExpr(toks[eq+1:], ln.lineNo)
	if err != nil {
		return nil, true, err
	}
	return &ast.MemberAssign{Target: target, Op: toks[eq].Kind, Value: val, Line: ln.lineNo, Col: ln.indent + 1}, true, nil
}

// stripImportant removes a trailing `!important` (lexed as NOT IDENT("important"))
// from a declaration's value tokens, reporting whether it was present. The
// terminating EOF token is preserved.
func stripImportant(toks []token.Token) ([]token.Token, bool) {
	n := len(toks)
	if n >= 3 && toks[n-1].Kind == token.EOF &&
		toks[n-2].Kind == token.IDENT && strings.EqualFold(toks[n-2].Text, "important") &&
		toks[n-3].Kind == token.NOT {
		trimmed := append(toks[:n-3:n-3], toks[n-1])
		return trimmed, true
	}
	return toks, false
}

// parseAtRule parses an at-rule line into an ast.AtRule, splitting the header into
// the at-keyword (without '@') and its raw parameters. Any indented children form
// the body; a childless at-rule has a nil body (rendered as a passthrough line).
func parseAtRule(ln *line) (ast.Stmt, error) {
	head := strings.TrimRight(ln.text, ";")
	// The keyword runs from '@' up to the first space (vendor hyphens included).
	i := 1
	for i < len(head) && head[i] != ' ' && head[i] != '\t' {
		i++
	}
	name := head[1:i]
	if name == "" {
		return nil, diag.Errorf(ln.lineNo, ln.indent+1, "empty at-rule")
	}
	params := strings.TrimSpace(head[i:])

	at := &ast.AtRule{Name: name, Params: params, Line: ln.lineNo, Col: ln.indent + 1}
	if len(ln.children) > 0 {
		body, err := parseBlock(ln.children)
		if err != nil {
			return nil, err
		}
		at.Body = body
	}
	return at, nil
}

// parseImport parses the argument of an `@import` or `@require` line (kw names
// which, for error messages and the Once flag). The argument is a quoted
// path or a url(...). A `.css` path, an absolute URL, or a url(...) becomes a
// literal passthrough import; any other path is inlined from a .styl file.
func parseImport(kw, rest string, lineNo, col int) (ast.Stmt, error) {
	once := kw == "@require"
	rest = strings.TrimSpace(strings.TrimSuffix(rest, ";"))
	if rest == "" {
		return nil, diag.Errorf(lineNo, col, "%s requires a path", kw)
	}
	if strings.HasPrefix(strings.ToLower(rest), "url(") {
		return &ast.Import{Path: rest, Literal: true, Once: once, Line: lineNo, Col: col}, nil
	}

	path := rest
	if len(path) >= 2 && (path[0] == '"' || path[0] == '\'') && path[len(path)-1] == path[0] {
		path = path[1 : len(path)-1]
	}
	low := strings.ToLower(path)
	literal := strings.HasSuffix(low, ".css") ||
		strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") ||
		strings.HasPrefix(path, "//")
	return &ast.Import{Path: path, Literal: literal, Once: once, Line: lineNo, Col: col}, nil
}

// SplitSelectors splits a selector group on its top-level commas (see
// splitSelectors); the evaluator uses it for selector() arguments.
func SplitSelectors(text string) []string { return splitSelectors(text) }

// splitSelectors splits a selector line on top-level commas, ignoring commas
// inside brackets, parentheses, or string literals (e.g. a[href$="a,b"] or
// :not(.x, .y) stay intact).
func splitSelectors(text string) []string {
	var out []string
	var cur strings.Builder
	var quote rune // 0 when not inside a string
	depth := 0     // () and [] nesting

	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			out = append(out, s)
		}
		cur.Reset()
	}

	for _, c := range text {
		switch {
		case quote != 0:
			cur.WriteRune(c)
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
			cur.WriteRune(c)
		case c == '(' || c == '[':
			depth++
			cur.WriteRune(c)
		case c == ')' || c == ']':
			if depth > 0 {
				depth--
			}
			cur.WriteRune(c)
		case c == ',' && depth == 0:
			flush()
		default:
			cur.WriteRune(c)
		}
	}
	flush()
	return out
}
