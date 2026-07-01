package parser

import (
	"strings"

	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/diag"
	"github.com/rohanthewiz/go-styl/internal/token"
)

// wordPrefix reports whether text is exactly word or begins with "word ".
func wordPrefix(text, word string) bool {
	return text == word || strings.HasPrefix(text, word+" ")
}

// isCondStart reports whether a line begins an if/unless chain.
func isCondStart(text string) bool {
	return wordPrefix(text, "if") || wordPrefix(text, "unless")
}

// parseConds consumes an if/unless line at lines[i] together with any following
// `else if` / `else` sibling lines, returning the If statement and the index of
// the next unconsumed line.
func parseConds(lines []*line, i int) (ast.Stmt, int, error) {
	head := lines[i]
	if len(head.children) == 0 {
		return nil, 0, diag.Errorf(head.lineNo, head.indent+1, "%q requires an indented block", head.text)
	}

	ifStmt := &ast.If{Line: head.lineNo, Col: head.indent + 1}

	// Leading if / unless.
	switch {
	case wordPrefix(head.text, "if"):
		cond, err := condExpr(head.text[len("if"):], head.lineNo)
		if err != nil {
			return nil, 0, err
		}
		body, err := parseBlock(head.children)
		if err != nil {
			return nil, 0, err
		}
		ifStmt.Branches = append(ifStmt.Branches, ast.CondBranch{Cond: cond, Body: body})
	case wordPrefix(head.text, "unless"):
		cond, err := condExpr(head.text[len("unless"):], head.lineNo)
		if err != nil {
			return nil, 0, err
		}
		body, err := parseBlock(head.children)
		if err != nil {
			return nil, 0, err
		}
		ifStmt.Branches = append(ifStmt.Branches, ast.CondBranch{Cond: &ast.Unary{Op: token.NOT, X: cond}, Body: body})
	}

	i++
	for i < len(lines) {
		ln := lines[i]
		switch {
		case wordPrefix(ln.text, "else if"):
			cond, err := condExpr(ln.text[len("else if"):], ln.lineNo)
			if err != nil {
				return nil, 0, err
			}
			body, err := parseBlock(ln.children)
			if err != nil {
				return nil, 0, err
			}
			ifStmt.Branches = append(ifStmt.Branches, ast.CondBranch{Cond: cond, Body: body})
			i++
		case ln.text == "else":
			body, err := parseBlock(ln.children)
			if err != nil {
				return nil, 0, err
			}
			ifStmt.Else = body
			i++
			return ifStmt, i, nil // else is terminal
		default:
			return ifStmt, i, nil
		}
	}
	return ifStmt, i, nil
}

// condExpr parses a control-flow condition expression from raw text.
func condExpr(text string, line int) (ast.Expr, error) {
	toks, err := lexLine(text, line)
	if err != nil {
		return nil, err
	}
	return parseExpr(toks, line)
}

// parseFor parses a `for val in expr` or `for key, val in expr` header (the line
// must have an indented body).
func parseFor(ln *line) (ast.Stmt, error) {
	toks, err := lexLine(ln.text[len("for"):], ln.lineNo)
	if err != nil {
		return nil, err
	}

	// Collect variable names up to the `in` keyword.
	var vars []string
	pos := 0
	for pos < len(toks) {
		t := toks[pos]
		if t.Kind == token.IDENT && t.Text == "in" {
			break
		}
		if t.Kind == token.IDENT {
			vars = append(vars, t.Text)
			pos++
			if pos < len(toks) && toks[pos].Kind == token.COMMA {
				pos++
			}
			continue
		}
		return nil, diag.Errorf(ln.lineNo, ln.indent+1, "malformed for-loop header")
	}
	if pos >= len(toks) || toks[pos].Text != "in" {
		return nil, diag.Errorf(ln.lineNo, ln.indent+1, "for-loop missing 'in'")
	}
	pos++ // consume 'in'

	iter, err := parseExpr(toks[pos:], ln.lineNo)
	if err != nil {
		return nil, err
	}

	body, err := parseBlock(ln.children)
	if err != nil {
		return nil, err
	}

	f := &ast.For{Iterable: iter, Body: body, Line: ln.lineNo, Col: ln.indent + 1}
	switch len(vars) {
	case 1:
		f.Value = vars[0]
	case 2:
		f.Index, f.Value = vars[0], vars[1]
	default:
		return nil, diag.Errorf(ln.lineNo, ln.indent+1, "for-loop expects 1 or 2 variables, got %d", len(vars))
	}
	return f, nil
}

// callSignature checks whether toks start with `IDENT ( ... )` and, if so,
// returns the name, the tokens inside the parentheses, and the tokens following
// the closing parenthesis (excluding the trailing EOF).
func callSignature(toks []token.Token) (name string, inner, rest []token.Token, ok bool) {
	// The '(' must be glued to the identifier (CSS function-token rule), so a
	// declaration like `margin (10px)` is not mistaken for a mixin call.
	if len(toks) < 3 || toks[0].Kind != token.IDENT || toks[1].Kind != token.LPAREN ||
		toks[1].SpaceBefore {
		return "", nil, nil, false
	}
	depth := 0
	for i := 1; i < len(toks); i++ {
		switch toks[i].Kind {
		case token.LPAREN:
			depth++
		case token.RPAREN:
			depth--
			if depth == 0 {
				return toks[0].Text, toks[2:i], toks[i+1:], true
			}
		}
	}
	return "", nil, nil, false
}

// parseParams parses a parameter list from the tokens between the signature
// parentheses (defaults and a trailing rest parameter are supported).
func parseParams(inner []token.Token, line int) ([]ast.Param, error) {
	segments := splitTopLevel(inner, token.COMMA)
	var params []ast.Param
	for _, seg := range segments {
		if len(seg) == 0 {
			continue
		}
		if seg[0].Kind != token.IDENT {
			return nil, diag.Errorf(line, 0, "invalid parameter")
		}
		p := ast.Param{Name: seg[0].Text}
		switch {
		case len(seg) == 1:
			// bare name
		case seg[1].Kind == token.ELLIPSIS:
			p.Rest = true
		case seg[1].Kind == token.ASSIGN:
			def, err := parseExpr(seg[2:], line)
			if err != nil {
				return nil, err
			}
			p.Default = def
		default:
			return nil, diag.Errorf(line, 0, "invalid parameter %q", seg[0].Text)
		}
		params = append(params, p)
	}
	return params, nil
}

// parseArgs parses a comma-separated argument list from the tokens between call
// parentheses. Each argument may itself be a space-separated list.
func parseArgs(inner []token.Token, line int) ([]ast.Expr, error) {
	segments := splitTopLevel(inner, token.COMMA)
	var args []ast.Expr
	for _, seg := range segments {
		if len(seg) == 0 {
			continue
		}
		e, err := parseExpr(seg, line)
		if err != nil {
			return nil, err
		}
		args = append(args, e)
	}
	return args, nil
}

// splitTopLevel splits tokens on sep at paren/bracket depth zero.
func splitTopLevel(toks []token.Token, sep token.Kind) [][]token.Token {
	var out [][]token.Token
	var cur []token.Token
	depth := 0
	for _, t := range toks {
		switch t.Kind {
		case token.LPAREN, token.LBRACKET:
			depth++
		case token.RPAREN, token.RBRACKET:
			depth--
		}
		if depth == 0 && t.Kind == sep {
			out = append(out, cur)
			cur = nil
			continue
		}
		cur = append(cur, t)
	}
	out = append(out, cur)
	return out
}
