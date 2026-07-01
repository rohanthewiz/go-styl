package parser

import (
	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/diag"
	"github.com/rohanthewiz/go-styl/internal/lexer"
	"github.com/rohanthewiz/go-styl/internal/token"
)

// exprParser is a Pratt parser over a token slice (terminated by EOF).
//
// Grammar (low to high precedence):
//
//	value      := spaceList ("," spaceList)*          // comma list
//	spaceList  := binary (binary)*                    // juxtaposition (space list)
//	binary     := unary (op unary)*                   // precedence-climbing
//	unary      := ("-" | "!" | "+") unary | primary
//	primary    := NUMBER | COLOR | STRING | IDENT ["(" args ")"] | "(" value ")"
type exprParser struct {
	toks []token.Token
	pos  int
	line int
	// propValue marks a declaration-value expression, where a `/` outside
	// parentheses is a literal slash (font: 14px/1.5), not division.
	propValue bool
	depth     int // current parenthesis nesting
}

// ParseExpr lexes and parses a single value expression from raw source text. It
// is the entry point used by the evaluator to resolve `{...}` interpolation.
func ParseExpr(src string, line int) (ast.Expr, error) {
	toks, err := lexer.Lex(src, line)
	if err != nil {
		return nil, err
	}
	return parseExpr(toks, line)
}

// parseExpr parses a value expression from toks (which must end with EOF).
func parseExpr(toks []token.Token, line int) (ast.Expr, error) {
	return parseExprMode(toks, line, false)
}

// parsePropExpr parses a declaration value. It differs from parseExpr in one
// way, matching reference Stylus: an unparenthesized `/` is not division — its
// operands still evaluate, but the slash renders literally (font: 14px/1.5).
func parsePropExpr(toks []token.Token, line int) (ast.Expr, error) {
	return parseExprMode(toks, line, true)
}

func parseExprMode(toks []token.Token, line int, propValue bool) (ast.Expr, error) {
	if len(toks) == 0 || toks[len(toks)-1].Kind != token.EOF {
		toks = append(toks, token.Token{Kind: token.EOF, Line: line})
	}
	p := &exprParser{toks: toks, line: line, propValue: propValue}
	e, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	if p.cur().Kind != token.EOF {
		return nil, diag.Errorf(line, 0, "unexpected %q in expression", p.cur().Text)
	}
	return e, nil
}

func (p *exprParser) cur() token.Token { return p.toks[p.pos] }
func (p *exprParser) next() token.Token {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

// parseValue handles the lowest precedence: comma-separated lists.
func (p *exprParser) parseValue() (ast.Expr, error) {
	first, err := p.parseSpaceList()
	if err != nil {
		return nil, err
	}
	if p.cur().Kind != token.COMMA {
		return first, nil
	}

	items := []ast.Expr{first}
	for p.cur().Kind == token.COMMA {
		p.next()
		it, err := p.parseSpaceList()
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return &ast.List{Items: items, Comma: true}, nil
}

// parseSpaceList handles juxtaposition: adjacent terms form a space-separated list.
func (p *exprParser) parseSpaceList() (ast.Expr, error) {
	first, err := p.parseBinary(0)
	if err != nil {
		return nil, err
	}
	if !startsTerm(p.cur().Kind) && !p.unaryPos(p.pos) {
		return first, nil
	}

	items := []ast.Expr{first}
	for startsTerm(p.cur().Kind) || p.unaryPos(p.pos) {
		it, err := p.parseBinary(0)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return &ast.List{Items: items, Comma: false}, nil
}

// unaryPos reports whether the operator at index i is a sign-prefix that begins a
// new space-list term rather than a binary operator. Stylus distinguishes these
// by whitespace: `10px -5px` (space before, none after) is a two-item list, while
// `10px - 5px` (space both sides) and `10px-5px` (none) are subtraction.
func (p *exprParser) unaryPos(i int) bool {
	k := p.toks[i].Kind
	if k != token.MINUS && k != token.PLUS {
		return false
	}
	if !p.toks[i].SpaceBefore || i+1 >= len(p.toks) {
		return false
	}
	nxt := p.toks[i+1]
	return !nxt.SpaceBefore && startsTerm(nxt.Kind)
}

// parseBinary is precedence-climbing over infix operators.
func (p *exprParser) parseBinary(minBP int) (ast.Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for {
		// A sign-prefixed term (e.g. the `-5px` in `10px -5px`) ends this binary
		// expression so the space-list loop can pick it up as a new item.
		if p.unaryPos(p.pos) {
			break
		}
		bp := infixBP(p.cur().Kind)
		if bp == 0 || bp < minBP {
			break
		}
		op := p.next().Kind
		right, err := p.parseBinary(bp + 1) // +1 => left-associative
		if err != nil {
			return nil, err
		}
		literal := op == token.SLASH && p.propValue && p.depth == 0
		left = &ast.Binary{Op: op, L: left, R: right, Literal: literal}
	}
	return left, nil
}

func (p *exprParser) parseUnary() (ast.Expr, error) {
	switch p.cur().Kind {
	case token.MINUS, token.NOT, token.PLUS:
		op := p.next().Kind
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &ast.Unary{Op: op, X: x}, nil
	default:
		return p.parsePrimary()
	}
}

func (p *exprParser) parsePrimary() (ast.Expr, error) {
	t := p.cur()
	switch t.Kind {
	case token.NUMBER:
		p.next()
		return &ast.NumberLit{Text: t.Text}, nil
	case token.COLOR:
		p.next()
		return &ast.ColorLit{Text: t.Text}, nil
	case token.STRING:
		p.next()
		return &ast.StringLit{Value: t.Text, Quote: t.Quote}, nil
	case token.IDENT:
		p.next()
		// A call requires the '(' glued to the identifier (CSS function-token
		// rule): `gutter (x)` is a space list, `gutter(x)` is a call.
		if p.cur().Kind == token.LPAREN && !p.cur().SpaceBefore {
			args, err := p.parseArgs()
			if err != nil {
				return nil, err
			}
			return &ast.Call{Name: t.Text, Args: args}, nil
		}
		return &ast.Ident{Name: t.Text}, nil
	case token.AMP:
		p.next()
		return &ast.Ident{Name: "&"}, nil
	case token.LPAREN:
		p.next()
		p.depth++
		e, err := p.parseValue()
		p.depth--
		if err != nil {
			return nil, err
		}
		if p.cur().Kind != token.RPAREN {
			return nil, diag.Errorf(p.line, 0, "expected ')'")
		}
		p.next()
		return e, nil
	default:
		return nil, diag.Errorf(p.line, 0, "unexpected %q in expression", t.Text)
	}
}

// parseArgs parses a parenthesized, comma-separated argument list. The opening
// '(' is the current token. Each argument may itself be a space-separated list.
func (p *exprParser) parseArgs() ([]ast.Expr, error) {
	p.next() // consume '('
	p.depth++
	defer func() { p.depth-- }()
	var args []ast.Expr
	if p.cur().Kind == token.RPAREN {
		p.next()
		return args, nil
	}
	for {
		arg, err := p.parseSpaceList()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if p.cur().Kind == token.COMMA {
			p.next()
			continue
		}
		break
	}
	if p.cur().Kind != token.RPAREN {
		return nil, diag.Errorf(p.line, 0, "expected ')' or ',' in argument list")
	}
	p.next()
	return args, nil
}

// startsTerm reports whether a token can begin a primary expression (used to
// detect juxtaposition for space-separated lists).
func startsTerm(k token.Kind) bool {
	switch k {
	case token.NUMBER, token.COLOR, token.STRING, token.IDENT, token.LPAREN, token.AMP:
		return true
	}
	return false
}

// infixBP returns the binding power of an infix operator, or 0 if not infix.
func infixBP(k token.Kind) int {
	switch k {
	case token.STAR, token.SLASH, token.PERCENT:
		return 60
	case token.PLUS, token.MINUS:
		return 50
	case token.LT, token.GT, token.LE, token.GE:
		return 40
	case token.EQ, token.NEQ:
		return 30
	case token.AND:
		return 20
	case token.OR:
		return 10
	}
	return 0
}
