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

func TestLexInterpolation(t *testing.T) {
	cases := []struct {
		src  string
		text string // expected text of the single IDENT token
	}{
		{"{x}", "{x}"},
		{"col-{i}", "col-{i}"},
		{"{prop}-top", "{prop}-top"},
		{"a{b}c", "a{b}c"},
		{"-{var}", "-{var}"},
	}
	for _, c := range cases {
		toks, err := Lex(c.src, 1)
		if err != nil {
			t.Fatalf("Lex(%q): %v", c.src, err)
		}
		if len(toks) != 2 || toks[0].Kind != token.IDENT || toks[1].Kind != token.EOF {
			t.Fatalf("Lex(%q) = %v, want [IDENT EOF]", c.src, kinds(toks))
		}
		if toks[0].Text != c.text {
			t.Errorf("Lex(%q) text = %q, want %q", c.src, toks[0].Text, c.text)
		}
	}
}

func TestLexRawCalls(t *testing.T) {
	cases := []string{
		"url(/img/x.png)",
		"url(\"/f.woff2\")",
		"calc(100% - 20px)",
		"calc(100% - {g})",
	}
	for _, src := range cases {
		toks, err := Lex(src, 1)
		if err != nil {
			t.Fatalf("Lex(%q): %v", src, err)
		}
		if len(toks) != 2 || toks[0].Kind != token.IDENT || toks[0].Text != src {
			t.Errorf("Lex(%q) = %v (text %q), want single IDENT", src, kinds(toks), toks[0].Text)
		}
	}
}

func TestLexSpaceBefore(t *testing.T) {
	// "10px -5px": the '-' is preceded by a space, the '5px' is not.
	toks, err := Lex("10px -5px", 1)
	if err != nil {
		t.Fatal(err)
	}
	// NUMBER MINUS NUMBER EOF
	if toks[1].Kind != token.MINUS || !toks[1].SpaceBefore {
		t.Errorf("MINUS SpaceBefore = %v, want true", toks[1].SpaceBefore)
	}
	if toks[2].Kind != token.NUMBER || toks[2].SpaceBefore {
		t.Errorf("NUMBER SpaceBefore = %v, want false", toks[2].SpaceBefore)
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
