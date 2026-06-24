package lexer

import (
	"testing"

	"github.com/rohanthewiz/go-styl/internal/token"
)

func kinds(toks []token.Token) []token.Kind {
	out := make([]token.Kind, 0, len(toks))
	for _, t := range toks {
		out = append(out, t.Kind)
	}
	return out
}

func TestLexBasics(t *testing.T) {
	cases := []struct {
		src   string
		kinds []token.Kind
	}{
		{"base * 2", []token.Kind{token.IDENT, token.STAR, token.NUMBER, token.EOF}},
		{"rgba(0, 0, 0, 0.5)", []token.Kind{
			token.IDENT, token.LPAREN, token.NUMBER, token.COMMA, token.NUMBER,
			token.COMMA, token.NUMBER, token.COMMA, token.NUMBER, token.RPAREN, token.EOF,
		}},
		{"#fff", []token.Kind{token.COLOR, token.EOF}},
		{`"hello"`, []token.Kind{token.STRING, token.EOF}},
		{"-webkit-box", []token.Kind{token.IDENT, token.EOF}},
		{"10px solid black", []token.Kind{token.NUMBER, token.IDENT, token.IDENT, token.EOF}},
	}
	for _, c := range cases {
		toks, err := Lex(c.src, 1)
		if err != nil {
			t.Fatalf("Lex(%q): %v", c.src, err)
		}
		got := kinds(toks)
		if len(got) != len(c.kinds) {
			t.Fatalf("Lex(%q) kinds = %v, want %v", c.src, got, c.kinds)
		}
		for i := range got {
			if got[i] != c.kinds[i] {
				t.Errorf("Lex(%q)[%d] = %v, want %v", c.src, i, got[i], c.kinds[i])
			}
		}
	}
}

func TestLexNumberUnit(t *testing.T) {
	toks, err := Lex("10px", 1)
	if err != nil {
		t.Fatal(err)
	}
	if toks[0].Kind != token.NUMBER || toks[0].Text != "10px" {
		t.Errorf("got %v %q, want NUMBER \"10px\"", toks[0].Kind, toks[0].Text)
	}
}
