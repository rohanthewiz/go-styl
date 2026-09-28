// Package lexer tokenizes a single Stylus expression or value string into a flat
// token slice. It deliberately does NOT handle indentation or whole-program
// structure: the parser builds an indentation line-tree and selectors are kept as
// raw strings, so the lexer only ever sees the right-hand side of a declaration,
// an assignment value, or a control-flow condition.
package lexer

import (
	"strings"
	"unicode"

	"github.com/rohanthewiz/go-styl/internal/diag"
	"github.com/rohanthewiz/go-styl/internal/token"
)

type lexer struct {
	src  []rune
	pos  int
	line int
	toks []token.Token
}

// Lex tokenizes s and returns the tokens terminated by an EOF token. line is the
// 1-based source line the expression came from, used only for error positions.
func Lex(s string, line int) ([]token.Token, error) {
	l := &lexer{src: []rune(s), line: line}
	if err := l.run(); err != nil {
		return nil, err
	}
	setSpacing(l.toks)
	return l.toks, nil
}

// setSpacing marks each token with whether whitespace preceded it, derived from
// the gap between adjacent token columns. This drives whitespace-sensitive
// disambiguation of unary vs binary '-'/'+'.
func setSpacing(toks []token.Token) {
	prevEnd := 1
	for i := range toks {
		t := &toks[i]
		t.SpaceBefore = t.Col > prevEnd
		end := t.Col + len([]rune(t.Text))
		if t.Kind == token.STRING {
			end += 2 // Text has the surrounding quotes stripped
		}
		prevEnd = end
	}
}

func (l *lexer) run() error {
	for l.pos < len(l.src) {
		c := l.src[l.pos]

		switch {
		case c == ' ' || c == '\t':
			l.pos++

		case c == '"' || c == '\'':
			if err := l.scanString(); err != nil {
				return err
			}

		case c == '#':
			if err := l.scanColor(); err != nil {
				return err
			}

		case unicode.IsDigit(c) || (c == '.' && l.peekIsDigit(1)):
			l.scanNumber()

		case l.atRawCall():
			// url(...) and calc(...) are captured verbatim so slashes, operators,
			// and units inside survive untouched (interpolation still resolves).
			l.scanRawCall()

		case c == '{' && l.atObjectLiteral():
			// `{key: value}` or `{}` is an object literal; its pairs lex as
			// ordinary tokens and the matching '}' becomes RBRACE.
			l.toks = append(l.toks, token.Token{Kind: token.LBRACE, Text: "{", Line: l.line, Col: l.pos + 1})
			l.pos++

		case isIdentStart(c) || c == '{':
			// '{' starts an interpolation that scanIdent folds into the token.
			l.scanIdent()

		case c == '-' && (isIdentLetter(l.peek(1)) || l.peek(1) == '-' || l.peek(1) == '{'):
			// Leading-hyphen identifier: -webkit-box, --custom-prop, -{var}.
			l.scanIdent()

		default:
			if err := l.scanOperator(); err != nil {
				return err
			}
		}
	}

	l.emit(token.EOF, "")
	return nil
}

func (l *lexer) scanString() error {
	quote := l.src[l.pos]
	col := l.pos + 1
	l.pos++ // opening quote
	var b strings.Builder

	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == '\\' && l.pos+1 < len(l.src) {
			// Escapes are kept verbatim (backslash included), as Stylus
			// does: the string's value is its raw source text, so CSS escapes
			// like "\2022" or "\f101" reach the output intact, and
			// length("\2022") is 5. Pairing the backslash with the next rune
			// only keeps an escaped quote (`'it\'s'`) from ending the string.
			// Stylus instead fails to parse that case.
			b.WriteRune(c)
			b.WriteRune(l.src[l.pos+1])
			l.pos += 2
			continue
		}
		if c == quote {
			l.pos++
			l.toks = append(l.toks, token.Token{Kind: token.STRING, Text: b.String(), Quote: quote, Line: l.line, Col: col})
			return nil
		}
		b.WriteRune(c)
		l.pos++
	}
	return diag.Errorf(l.line, 0, "unterminated string literal")
}

func (l *lexer) scanColor() error {
	start := l.pos
	l.pos++ // '#'
	digits := 0
	for l.pos < len(l.src) && isHex(l.src[l.pos]) {
		l.pos++
		digits++
	}
	if digits != 3 && digits != 4 && digits != 6 && digits != 8 {
		return diag.Errorf(l.line, 0, "invalid hex color %q", string(l.src[start:l.pos]))
	}
	l.toks = append(l.toks, token.Token{Kind: token.COLOR, Text: string(l.src[start:l.pos]), Line: l.line, Col: start + 1})
	return nil
}

func (l *lexer) scanNumber() {
	start := l.pos
	for l.pos < len(l.src) && unicode.IsDigit(l.src[l.pos]) {
		l.pos++
	}
	if l.pos < len(l.src) && l.src[l.pos] == '.' && l.peekIsDigit(1) {
		l.pos++ // '.'
		for l.pos < len(l.src) && unicode.IsDigit(l.src[l.pos]) {
			l.pos++
		}
	}
	// Optional unit: a run of letters, or '%'.
	if l.pos < len(l.src) && l.src[l.pos] == '%' {
		l.pos++
	} else {
		for l.pos < len(l.src) && unicode.IsLetter(l.src[l.pos]) {
			l.pos++
		}
	}
	l.toks = append(l.toks, token.Token{Kind: token.NUMBER, Text: string(l.src[start:l.pos]), Line: l.line, Col: start + 1})
}

// scanIdent reads an identifier, folding any embedded `{...}` interpolation
// groups into the token text (e.g. `col-{i}`, `{prop}-top`, or a bare `{x}`).
// The braces are kept verbatim; the evaluator resolves them later.
// rawCallNames are functions whose argument list is kept as a literal token
// rather than parsed as an expression (so url(/a.png) and calc(100% - 2px) work).
var rawCallNames = map[string]bool{"url": true, "calc": true}

// atRawCall reports whether the lexer is positioned at the start of a raw-call
// name (url/calc, case-insensitive) immediately followed by '('.
func (l *lexer) atRawCall() bool {
	start := l.pos
	i := start
	for i < len(l.src) && isIdentLetter(l.src[i]) {
		i++
	}
	if i == start || i >= len(l.src) || l.src[i] != '(' {
		return false
	}
	return rawCallNames[strings.ToLower(string(l.src[start:i]))]
}

// scanRawCall consumes `name( ... )` with balanced parentheses (strings honored)
// and emits it as a single IDENT token carrying the verbatim text.
func (l *lexer) scanRawCall() {
	start := l.pos
	for l.pos < len(l.src) && l.src[l.pos] != '(' {
		l.pos++
	}
	depth := 0
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == '"' || c == '\'' {
			quote := c
			l.pos++
			for l.pos < len(l.src) && l.src[l.pos] != quote {
				if l.src[l.pos] == '\\' && l.pos+1 < len(l.src) {
					l.pos++
				}
				l.pos++
			}
			if l.pos < len(l.src) {
				l.pos++ // closing quote
			}
			continue
		}
		l.pos++
		if c == '(' {
			depth++
		} else if c == ')' {
			depth--
			if depth == 0 {
				break
			}
		}
	}
	l.toks = append(l.toks, token.Token{Kind: token.IDENT, Text: string(l.src[start:l.pos]), Line: l.line, Col: start + 1})
}

func (l *lexer) scanIdent() {
	start := l.pos
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case isIdentPart(c):
			l.pos++
		case c == '{':
			l.skipInterp()
		default:
			l.toks = append(l.toks, token.Token{Kind: token.IDENT, Text: string(l.src[start:l.pos]), Line: l.line, Col: start + 1})
			return
		}
	}
	l.toks = append(l.toks, token.Token{Kind: token.IDENT, Text: string(l.src[start:l.pos]), Line: l.line, Col: start + 1})
}

// skipInterp advances past a balanced `{...}` interpolation group (nested braces
// counted). An unterminated group is consumed to end-of-input.
func (l *lexer) skipInterp() {
	depth := 0
	for l.pos < len(l.src) {
		switch l.src[l.pos] {
		case '{':
			depth++
		case '}':
			depth--
		}
		l.pos++
		if depth == 0 {
			return
		}
	}
}

// scanOperator handles operators and punctuation, preferring two-character forms.
func (l *lexer) scanOperator() error {
	col := l.pos + 1

	// Ellipsis (rest parameter): ...
	if l.src[l.pos] == '.' && l.peek(1) == '.' && l.peek(2) == '.' {
		l.toks = append(l.toks, token.Token{Kind: token.ELLIPSIS, Text: "...", Line: l.line, Col: col})
		l.pos += 3
		return nil
	}

	two := ""
	if l.pos+1 < len(l.src) {
		two = string(l.src[l.pos : l.pos+2])
	}

	if k, ok := twoCharOps[two]; ok {
		l.toks = append(l.toks, token.Token{Kind: k, Text: two, Line: l.line, Col: col})
		l.pos += 2
		return nil
	}

	c := l.src[l.pos]
	// Member access: a '.' glued to the end of an operand and followed by a
	// name (obj.key, obj[k].key, f().key). A spaced or unglued '.' is not
	// valid in an expression, so this cannot shadow another reading.
	if c == '.' && isIdentStart(l.peek(1)) && l.gluedToOperand() {
		l.toks = append(l.toks, token.Token{Kind: token.DOT, Text: ".", Line: l.line, Col: col})
		l.pos++
		return nil
	}
	if k, ok := oneCharOps[c]; ok {
		l.toks = append(l.toks, token.Token{Kind: k, Text: string(c), Line: l.line, Col: col})
		l.pos++
		return nil
	}
	return diag.Errorf(l.line, 0, "unexpected character %q", string(c))
}

// gluedToOperand reports whether the previous token ends exactly at the
// current position and can end an operand (a name, `]`, `)` or `}`).
func (l *lexer) gluedToOperand() bool {
	if len(l.toks) == 0 || l.pos == 0 {
		return false
	}
	switch l.src[l.pos-1] {
	case ' ', '\t':
		return false
	}
	switch l.toks[len(l.toks)-1].Kind {
	case token.IDENT, token.RBRACKET, token.RPAREN, token.RBRACE:
		return true
	}
	return false
}

// atObjectLiteral reports whether the '{' at the current position opens an
// object literal rather than an interpolation group. Interpolation glued to a
// name on the left never reaches here (scanIdent folds it in), so the
// remaining cases are told apart by content:
//
//	{}             → object (empty)
//	{a: 1, b: 2}   → object (a top-level ':' separates a key from its value)
//	{x}            → interpolation (a lone expression)
//	{x}-top        → interpolation (glued to a name on the right)
//
// ':' inside strings, parentheses, brackets or a nested group doesn't count.
// go-styl has no ternary operator, so an expression can't contain a bare ':'.
func (l *lexer) atObjectLiteral() bool {
	depth := 0
	var quote rune
	colon := false
	for i := l.pos; i < len(l.src); i++ {
		c := l.src[i]
		if quote != 0 {
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '{', '(', '[':
			depth++
		case '}', ')', ']':
			depth--
			if depth > 0 {
				continue
			}
			// The group closed. Glued to a name on the right, it is
			// interpolation (`{side}-width`).
			if i+1 < len(l.src) && (isIdentPart(l.src[i+1]) || l.src[i+1] == '{') {
				return false
			}
			return colon || strings.TrimSpace(string(l.src[l.pos+1:i])) == ""
		case ':':
			if depth == 1 {
				colon = true
			}
		}
	}
	return false // unterminated: scanIdent keeps its old behavior
}

func (l *lexer) emit(k token.Kind, text string) {
	l.toks = append(l.toks, token.Token{Kind: k, Text: text, Line: l.line, Col: l.pos + 1})
}

func (l *lexer) peek(n int) rune {
	if l.pos+n < len(l.src) {
		return l.src[l.pos+n]
	}
	return 0
}

func (l *lexer) peekIsDigit(n int) bool { return unicode.IsDigit(l.peek(n)) }

var twoCharOps = map[string]token.Kind{
	"==": token.EQ, "!=": token.NEQ, "<=": token.LE, ">=": token.GE,
	"&&": token.AND, "||": token.OR, "?=": token.ASSIGNQ,
	"**": token.POW, "..": token.DOTDOT, // ("..." is checked before this map)
}

var oneCharOps = map[rune]token.Kind{
	'=': token.ASSIGN, '+': token.PLUS, '-': token.MINUS, '*': token.STAR,
	'/': token.SLASH, '%': token.PERCENT, '<': token.LT, '>': token.GT, '!': token.NOT,
	'(': token.LPAREN, ')': token.RPAREN, '[': token.LBRACKET, ']': token.RBRACKET,
	',': token.COMMA, ':': token.COLON, ';': token.SEMI, '&': token.AMP,
	'}': token.RBRACE,
}

func isIdentStart(c rune) bool  { return isIdentLetter(c) || c == '_' }
func isIdentLetter(c rune) bool { return unicode.IsLetter(c) }
func isIdentPart(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' || c == '-'
}

func isHex(c rune) bool {
	return unicode.IsDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
