// Package parser turns Stylus source into an AST. It first builds an indentation
// line-tree (comments stripped), then classifies each line as a ruleset,
// declaration, or assignment. Selectors are kept as raw strings; only values and
// conditions are tokenized and parsed as expressions (see expr.go).
package parser

import (
	"fmt"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/ast"
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

	for i, raw := range strings.Split(cleaned, "\n") {
		indent, content := splitIndent(raw)
		if content == "" {
			continue // blank or comment-only line
		}

		ln := &line{text: content, indent: indent, lineNo: i + 1}

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
	var stmts []ast.Stmt
	for i := 0; i < len(lines); i++ {
		ln := lines[i]
		if isCondStart(ln.text) {
			stmt, next, err := parseConds(lines, i)
			if err != nil {
				return nil, err
			}
			stmts = append(stmts, stmt)
			i = next - 1
			continue
		}
		stmt, err := parseLine(ln)
		if err != nil {
			return nil, err
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
			return nil, fmt.Errorf("line %d: at-rules are not supported yet", ln.lineNo)
		}
		if wordPrefix(text, "for") {
			return parseFor(ln)
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
				return &ast.FuncDef{Name: name, Params: params, Body: body}, nil
			}
		}
		// Otherwise a ruleset; selectors are kept raw (not tokenized).
		body, err := parseBlock(ln.children)
		if err != nil {
			return nil, err
		}
		return &ast.RuleSet{Selectors: splitSelectors(text), Body: body}, nil
	}

	// --- Leaf lines ---

	// return [expr]
	if wordPrefix(text, "return") {
		rest := strings.TrimSpace(text[len("return"):])
		if rest == "" {
			return &ast.Return{}, nil
		}
		toks, err := lexLine(rest, ln.lineNo)
		if err != nil {
			return nil, err
		}
		val, err := parseExpr(toks, ln.lineNo)
		if err != nil {
			return nil, err
		}
		return &ast.Return{Value: val}, nil
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
			return &ast.MixinCall{Name: name, Args: args}, nil
		}
		if len(toks) == 3 && toks[2].Kind == token.EOF {
			return &ast.MixinCall{Name: toks[1].Text}, nil
		}
	}

	// Variable assignment: name = expr  or  name ?= expr
	if toks[0].Kind == token.IDENT && len(toks) >= 2 &&
		(toks[1].Kind == token.ASSIGN || toks[1].Kind == token.ASSIGNQ) {
		val, err := parseExpr(toks[2:], ln.lineNo)
		if err != nil {
			return nil, err
		}
		return &ast.Assignment{Name: toks[0].Text, Op: toks[1].Kind, Value: val}, nil
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
		return &ast.FuncDef{Name: name, Params: params, Body: []ast.Stmt{&ast.Return{Value: val}}}, nil
	}

	// Bare mixin call: a single identifier on its own line.
	if len(toks) == 2 && toks[0].Kind == token.IDENT && toks[1].Kind == token.EOF {
		return &ast.MixinCall{Name: toks[0].Text}, nil
	}

	// Mixin call: name(args) consuming the whole line.
	if name, inner, rest, ok := callSignature(toks); ok && onlyEOF(rest) {
		args, err := parseArgs(inner, ln.lineNo)
		if err != nil {
			return nil, err
		}
		return &ast.MixinCall{Name: name, Args: args}, nil
	}

	// Declaration: `property value...`, with an optional colon after the property.
	if toks[0].Kind != token.IDENT {
		return nil, fmt.Errorf("line %d: expected a property declaration, got %q", ln.lineNo, text)
	}
	valToks := toks[1:]
	if valToks[0].Kind == token.COLON {
		valToks = valToks[1:]
	}
	val, err := parseExpr(valToks, ln.lineNo)
	if err != nil {
		return nil, err
	}
	return &ast.Declaration{Property: toks[0].Text, Value: val}, nil
}

// splitSelectors splits a selector line on top-level commas.
// NOTE: this does not yet account for commas inside brackets/strings,
// e.g. a[href$="a,b"]; that is a known limitation tracked for a later milestone.
func splitSelectors(text string) []string {
	parts := strings.Split(text, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}
