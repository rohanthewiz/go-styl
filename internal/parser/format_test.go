package parser

import (
	"errors"
	"testing"
)

func TestFormat(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{
			name: "indented syntax: depth-based indentation, comments kept",
			in: "// header\n\n\n\nbase   =   10px\r\n.a,\n.b\n\tcolor   red   // trailing\n\t/* multi\n\t   line */\n" +
				"\t&:hover\n\t\t\tcolor blue\n    // deeper note\n\n\n.c\n      margin 0\n      transition opacity 1s,\n        transform 2s\n\n\n",
			want: "// header\n\nbase = 10px\n.a,\n.b\n  color red   // trailing\n  /* multi\n     line */\n" +
				"  &:hover\n    color blue\n  // deeper note\n\n.c\n  margin 0\n  transition opacity 1s,\n    transform 2s\n",
		},
		{
			name: "object literal body moves with its statement",
			in:   ".x\n    theme = {\n        bg: #fff,\n        fg:   #333\n    }\n    color theme.fg\n",
			want: ".x\n  theme = {\n      bg: #fff,\n      fg: #333\n  }\n  color theme.fg\n",
		},
		{
			name: "brace syntax: indentation from the braces",
			in:   ".a {\ncolor: red;\n      .b { x: 1 }\n    .c {\n  margin:   0;\n}\n}\n@media (max-width: 10px) {\n.d { color: blue }\n}\n",
			want: ".a {\n  color: red;\n  .b { x: 1 }\n  .c {\n    margin: 0;\n  }\n}\n@media (max-width: 10px) {\n  .d { color: blue }\n}\n",
		},
		{
			name: "strings and interpolation untouched",
			in:   "a\n    content   \"two  spaces\"\n    {prop}   value\n",
			want: "a\n  content \"two  spaces\"\n  {prop} value\n",
		},
		{
			name: "assignment and parameter-default spacing",
			in:   "x=1\ny  ?=2\nobj = {a: 1}\nobj.a=2\nm(a=1,b  =  f(1,2), c...)\n  width a\nif x==1\n  z = 1\n",
			want: "x = 1\ny ?= 2\nobj = {a: 1}\nobj.a = 2\nm(a = 1, b = f(1, 2), c...)\n  width a\nif x==1\n  z = 1\n",
		},
		{
			name: "comma spacing in values, not in selectors, strings or url()",
			in:   "a,b\n  font-family x ,y,z\n  content 'a,b'\n  background url(a.png?x=1,2)\n  +m(1 ,2)\n  transition a 1s,\n    b 2s ,c 3s\n",
			want: "a,b\n  font-family x, y, z\n  content 'a,b'\n  background url(a.png?x=1,2)\n  +m(1, 2)\n  transition a 1s,\n    b 2s, c 3s\n",
		},
		{
			name: "declaration form: the file's majority in indentation syntax",
			in:   ".a\n  color:red\n  margin : 0\n  padding 0\n",
			want: ".a\n  color: red\n  margin: 0\n  padding: 0\n",
		},
		{
			name: "declaration form: a tie keeps each as written, spacing still fixed",
			in:   ".a\n  color:red\n  padding 0\n",
			want: ".a\n  color: red\n  padding 0\n",
		},
		{
			name: "declaration form: always a colon inside braces",
			in:   ".a {\n  color red;\n  margin:0;\n}\n.b\n  width 1px\n  height 1px\n",
			want: ".a {\n  color: red;\n  margin: 0;\n}\n.b\n  width 1px\n  height 1px\n",
		},
		{
			name: "trailing comments are kept in place",
			in:   ".a\n  color:red   // why\n  margin: 0\n",
			want: ".a\n  color: red   // why\n  margin: 0\n",
		},
		{
			name: "mixed syntax: lines outside braces re-indented by depth",
			in:   "m()\n    .a {\n        color: red;\n    }\n    .d\n          color blue\n.e { x: 1 }\n     .f\n        y 2\n",
			want: "m()\n  .a {\n    color: red;\n  }\n  .d\n    color blue\n.e { x: 1 }\n.f\n  y 2\n",
		},
		{
			name: "empty input",
			in:   "\n\n  \n",
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Format(c.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, c.want)
			}
			again, err := Format(got)
			if err != nil || again != got {
				t.Errorf("not idempotent: %q (err=%v)", again, err)
			}
		})
	}
}

// TestRespellFallback: when the edits together change the AST, each is
// tried on its own and only the bad one is dropped. The bad edit here is
// synthetic (turning a declaration into a selector group); see the note in
// respell.go.
func TestRespellFallback(t *testing.T) {
	in := ".x\n  a b\n  .c\n    d e\n  f:g\n"
	orig, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	got := applyGuarded(in, orig, []lineEdit{{1, "  a b,"}, {4, "  f: g"}})
	if want := ".x\n  a b\n  .c\n    d e\n  f: g\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatParseError(t *testing.T) {
	if _, err := Format(".a\n  color (red\n"); err == nil || errors.Is(err, ErrFormatUnsafe) {
		t.Fatalf("want the parse error, got %v", err)
	}
}

// FuzzFormat checks Format never panics, that a successful format always
// re-parses to the same stylesheet (the guard's promise), and that
// formatting is idempotent. The saved corpus (testdata/fuzz/FuzzFormat) holds
// the stray-whitespace inputs (`\r`, `\f`) that once took two runs to
// settle.
func FuzzFormat(f *testing.F) {
	for _, s := range []string{
		".a\n  color red\n", ".a { color: red; .b { x: 1 } }\n", "x = {\n a: 1\n}\n",
		"m()\n  .a {\n    c: 1\n  }\n", "/* a\n b */\n.a\n\tb c // d\n", ".a,\n  .b\n    c d\n",
		"x=1\nm(a=1,b)\n  c:d ,e\n", ".x\n  a: b\n  .c\n    d e\n  f g\n", "m()\n    .a { b: c }\n        .d\n  e f\n",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		orig, err := Parse(src)
		if err != nil {
			return
		}
		out, err := Format(src)
		if err != nil {
			return
		}
		sheet, err := Parse(out)
		if err != nil || !sameAST(orig, sheet) {
			t.Fatalf("format changed the AST:\n%q\n→\n%q", src, out)
		}
		again, err := Format(out)
		if err != nil || again != out {
			t.Fatalf("not idempotent:\n%q\n→\n%q\n→\n%q (err=%v)", src, out, again, err)
		}
	})
}
