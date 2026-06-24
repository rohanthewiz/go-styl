// Package token defines the lexical tokens produced by the lexer.
package token

// Kind enumerates the lexical token categories.
type Kind int

const (
	ILLEGAL Kind = iota
	EOF

	// Structural (indentation-based blocks)
	NEWLINE // end of a logical line
	INDENT  // increase in indentation (start of a block)
	DEDENT  // decrease in indentation (end of a block)

	// Literals & names
	IDENT  // identifiers, property names, bare keywords, units-bearing words
	NUMBER // numeric literal, with optional trailing unit captured in Text (e.g. "10px")
	STRING // quoted string literal, quotes stripped, Quote records the original quote rune
	COLOR  // hex color literal including '#', e.g. "#fff" or "#aabbcc"
	AT     // at-keyword including '@', e.g. "@media"

	// Operators
	ASSIGN  // =
	ASSIGNQ // ?=  (define if not already defined)
	PLUS    // +
	MINUS   // -
	STAR    // *
	SLASH   // /
	PERCENT // %
	EQ      // ==
	NEQ     // !=
	LT      // <
	GT      // >
	LE      // <=
	GE      // >=
	AND     // &&
	OR      // ||
	NOT     // !

	// Punctuation
	LPAREN   // (
	RPAREN   // )
	LBRACKET // [
	RBRACKET // ]
	COMMA    // ,
	COLON    // :
	SEMI     // ;
	AMP      // &  (parent-selector reference)
	ELLIPSIS // ... (rest parameter)

	COMMENT // /* ... */ block comment, preserved in output
)

// Token is a single lexical token with source position.
type Token struct {
	Kind        Kind
	Text        string // literal text (numbers keep their unit; strings drop quotes)
	Quote       rune   // original quote rune for STRING tokens ('"' or '\''), else 0
	Line        int    // 1-based source line
	Col         int    // 1-based source column
	SpaceBefore bool   // whitespace immediately preceded this token
}

var kindNames = map[Kind]string{
	ILLEGAL: "ILLEGAL", EOF: "EOF", NEWLINE: "NEWLINE", INDENT: "INDENT", DEDENT: "DEDENT",
	IDENT: "IDENT", NUMBER: "NUMBER", STRING: "STRING", COLOR: "COLOR", AT: "AT",
	ASSIGN: "ASSIGN", ASSIGNQ: "ASSIGNQ", PLUS: "PLUS", MINUS: "MINUS", STAR: "STAR",
	SLASH: "SLASH", PERCENT: "PERCENT", EQ: "EQ", NEQ: "NEQ", LT: "LT", GT: "GT",
	LE: "LE", GE: "GE", AND: "AND", OR: "OR", NOT: "NOT",
	LPAREN: "LPAREN", RPAREN: "RPAREN", LBRACKET: "LBRACKET", RBRACKET: "RBRACKET",
	COMMA: "COMMA", COLON: "COLON", SEMI: "SEMI", AMP: "AMP", ELLIPSIS: "ELLIPSIS", COMMENT: "COMMENT",
}

// String returns the token kind's name (handy in tests and errors).
func (k Kind) String() string {
	if s, ok := kindNames[k]; ok {
		return s
	}
	return "Kind(?)"
}
