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
//	primary    := operand ("[" value "]" | "." IDENT)*   // subscripts, members
//	operand    := NUMBER | COLOR | STRING | IDENT ["(" args ")"] | "(" value ")"
//	            | "{" [pair ((","|";") pair)*] "}"      // object literal
//	pair       := (IDENT | STRING) ":" spaceList
type exprParser struct {
	toks []token.Token
	pos  int
	line int
	// propValue marks a declaration-value expression, where a `/` outside
	// parentheses is a literal slash (font: 14px/1.5), not division.
	propValue bool
	depth     int // current parenthesis nesting
	callDepth int // current call-argument nesting (f(…))
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
		bp := p.curInfixBP()
		if bp == 0 || bp < minBP {
			break
		}
		op := p.next().Kind
		if op == token.IDENT { // the `in` keyword (see curInfixBP)
			op = token.IN
		}
		right, err := p.parseBinary(bp + 1) // +1 => left-associative
		if err != nil {
			return nil, err
		}
		literal := op == token.SLASH && p.propValue && p.depth == 0
		// CSS itself uses the word `in` (color interpolation:
		// linear-gradient(to right in oklch, …)). In a property value or a
		// call's arguments, an `in` whose right side turns out to be a bare
		// word stays text; see ast.Binary.InText.
		inText := op == token.IN && (p.propValue || p.callDepth > 0)
		left = &ast.Binary{Op: op, L: left, R: right, Literal: literal, InText: inText}
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

// curInfixBP returns the binding power of the current token as an infix
// operator, or 0. Besides the operator tokens it recognizes the keyword `in`
// (`'key' in obj`, `3 in list`), which Stylus binds between equality and
// relational comparison, so `a in l == true` is `(a in l) == true`.
func (p *exprParser) curInfixBP() int {
	t := p.cur()
	if t.Kind == token.IDENT && t.Text == "in" {
		return 35
	}
	return infixBP(t.Kind)
}

// parsePrimary parses one operand and any subscripts or member accesses glued
// to it: `r[1]`, `f(x)[0]`, `(1 2 3)[-1]`, `r[0][1]`, `obj.key`,
// `theme.colors[mode]`. The `[` must touch the operand; with whitespace before
// it (`1fr [main-start]`) it is not a subscript, which leaves room for CSS
// grid line names. The lexer only produces DOT when the '.' is glued to the
// operand and followed by a name.
func (p *exprParser) parsePrimary() (ast.Expr, error) {
	x, err := p.parseOperand()
	if err != nil {
		return nil, err
	}
	for {
		if p.cur().Kind == token.DOT {
			p.next()
			if p.cur().Kind != token.IDENT {
				return nil, diag.Errorf(p.line, 0, "expected a key name after '.'")
			}
			x = &ast.Member{X: x, Name: p.next().Text}
			continue
		}
		if p.cur().Kind != token.LBRACKET || p.cur().SpaceBefore {
			break
		}
		p.next()
		p.depth++
		idx, err := p.parseValue()
		p.depth--
		if err != nil {
			return nil, err
		}
		if p.cur().Kind != token.RBRACKET {
			return nil, diag.Errorf(p.line, 0, "expected ']'")
		}
		p.next()
		x = &ast.Index{X: x, Index: idx}
	}
	return x, nil
}

// parseObject parses an object literal; the current token is its '{'. Pairs
// are separated by commas or semicolons, and empty entries are skipped, so
// the multi-line form (which the parser folds onto one line with commas, see
// joinObjectLiterals) may also end its lines with commas: `{, a: 1,, b: 2, }`.
// A key is a bare name or a quoted string; a value is a space list (a comma
// would end the pair).
func (p *exprParser) parseObject() (ast.Expr, error) {
	p.next() // consume '{'
	p.depth++
	defer func() { p.depth-- }()
	obj := &ast.Object{}
	for {
		switch p.cur().Kind {
		case token.COMMA, token.SEMI:
			p.next()
			continue
		case token.RBRACE:
			p.next()
			return obj, nil
		case token.IDENT, token.STRING:
		default:
			return nil, diag.Errorf(p.line, 0, "expected an object key or '}', got %q", p.cur().Text)
		}
		key := p.next().Text
		if p.cur().Kind != token.COLON {
			return nil, diag.Errorf(p.line, 0, "expected ':' after object key %q", key)
		}
		p.next()
		val, err := p.parseSpaceList()
		if err != nil {
			return nil, err
		}
		obj.Pairs = append(obj.Pairs, ast.ObjectPair{Key: key, Value: val})
	}
}

// parseOperand parses a single literal, identifier, call or parenthesized
// expression.
func (p *exprParser) parseOperand() (ast.Expr, error) {
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
	case token.LBRACE:
		return p.parseObject()
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
	case token.EOF:
		// The line ended where an operand was due (`width 1px +`).
		return nil, diag.Errorf(p.line, 0, "unexpected end of expression")
	default:
		return nil, diag.Errorf(p.line, 0, "unexpected %q in expression", t.Text)
	}
}

// parseArgs parses a parenthesized, comma-separated argument list. The opening
// '(' is the current token. Each argument may itself be a space-separated list.
func (p *exprParser) parseArgs() ([]ast.Expr, error) {
	p.next() // consume '('
	p.depth++
	p.callDepth++
	defer func() { p.depth--; p.callDepth-- }()
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
	case token.NUMBER, token.COLOR, token.STRING, token.IDENT, token.LPAREN, token.AMP, token.LBRACE:
		return true
	}
	return false
}

// infixBP returns the binding power of an infix operator, or 0 if not infix.
func infixBP(k token.Kind) int {
	switch k {
	// `**` shares the multiplicative level and associates left, as in
	// reference Stylus: 2 * 3 ** 2 is 36 and 2 ** 3 ** 2 is 64.
	case token.STAR, token.POW, token.SLASH, token.PERCENT:
		return 60
	case token.PLUS, token.MINUS:
		return 50
	// Ranges bind below arithmetic (1..n + 1 is 1..(n+1)) but above
	// comparisons.
	case token.DOTDOT, token.ELLIPSIS:
		return 45
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
