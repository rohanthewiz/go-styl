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

func TestFormatParseError(t *testing.T) {
	if _, err := Format(".a\n  color (red\n"); err == nil || errors.Is(err, ErrFormatUnsafe) {
		t.Fatalf("want the parse error, got %v", err)
	}
}

// FuzzFormat checks Format never panics and that a successful format always
// re-parses to the same stylesheet (the guard's promise).
func FuzzFormat(f *testing.F) {
	for _, s := range []string{
		".a\n  color red\n", ".a { color: red; .b { x: 1 } }\n", "x = {\n a: 1\n}\n",
		"m()\n  .a {\n    c: 1\n  }\n", "/* a\n b */\n.a\n\tb c // d\n", ".a,\n  .b\n    c d\n",
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
	})
}
