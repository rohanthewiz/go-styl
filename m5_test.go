package styl_test

import (
	"strings"
	"testing"

	styl "github.com/rohanthewiz/go-styl"
)

func compileMin(t *testing.T, src string) string {
	t.Helper()
	got, err := styl.Compile(src, styl.Options{Pretty: false})
	if err != nil {
		t.Fatalf("compile %q: %v", src, err)
	}
	return strings.TrimRight(got, "\n")
}

// TestM5AtRules covers at-rule kinds compiled compressed.
func TestM5AtRules(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			"media block",
			"@media screen and (min-width: 100px)\n  .a\n    x 1",
			"@media screen and (min-width:100px){.a{x:1}}",
		},
		{
			"media bubbles enclosing selector",
			".a\n  color red\n  @media print\n    color gray",
			".a{color:red}@media print{.a{color:gray}}",
		},
		{
			"media param interpolation",
			"bp = 50em\n@media (min-width: {bp})\n  .a\n    x 1",
			"@media (min-width:50em){.a{x:1}}",
		},
		{
			"keyframes frames not combined",
			"@keyframes k\n  from\n    opacity 0\n  to\n    opacity 1",
			"@keyframes k{from{opacity:0}to{opacity:1}}",
		},
		{
			"keyframes percent frames",
			"@keyframes k\n  0%\n    top 0\n  100%\n    top 9px",
			"@keyframes k{0%{top:0}100%{top:9px}}",
		},
		{
			"font-face decl block",
			`@font-face` + "\n  font-family \"F\"\n  src url(\"/f.woff\")",
			`@font-face{font-family:"F";src:url("/f.woff")}`,
		},
		{
			"leaf at-rule passthrough",
			`@charset "utf-8"` + "\na\n  x 1",
			`@charset "utf-8";a{x:1}`,
		},
		{
			"vendor-prefixed keyframes",
			"@-webkit-keyframes k\n  to\n    opacity 1",
			"@-webkit-keyframes k{to{opacity:1}}",
		},
		{
			"empty at-rule produces nothing",
			"@media print\n  $ph\n    x 1",
			"",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := compileMin(t, c.src+"\n"); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestM5BraceSyntax checks that brace/semicolon syntax compiles to the same CSS as
// the equivalent indentation source, and that interpolation still works.
func TestM5BraceSyntax(t *testing.T) {
	cases := []struct {
		name  string
		brace string
		want  string
	}{
		{
			"nested rules and semicolons",
			"body {\n  color: red;\n  a { color: blue; }\n}",
			"body{color:red}body a{color:blue}",
		},
		{
			"parent ref single line",
			"a {\n  &:hover { color: green; }\n}",
			"a:hover{color:green}",
		},
		{
			"arithmetic in brace value",
			"x = 4px;\n.a { width: x * 2; }",
			".a{width:8px}",
		},
		{
			"selector interpolation in brace mode",
			"p = col;\n.{p}-1 { width: 1px; }",
			".col-1{width:1px}",
		},
		{
			"media inside braces",
			"@media print {\n  .a { color: black; }\n}",
			"@media print{.a{color:black}}",
		},
		{
			"if/else in brace mode",
			"t = dark;\nbody {\n  if t == dark { color: white; } else { color: black; }\n}",
			"body{color:white}",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := compileMin(t, c.brace+"\n"); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestM5IndentationUnaffected confirms an indentation file using value
// interpolation is not misdetected as brace syntax.
func TestM5IndentationUnaffected(t *testing.T) {
	got := compileMin(t, "n = 5\na\n  width {n}\n  margin {n}px\n")
	if want := "a{width:5;margin:5px}"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
