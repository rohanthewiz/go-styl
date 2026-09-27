package styl_test

import (
	"strings"
	"testing"

	"github.com/rohanthewiz/go-styl"
)

// TestSemicolonsInIndentedSyntax covers `;` as a statement separator in the
// indentation syntax (N-027): a trailing `;` is dropped and several
// statements may share a line. Expected output matches reference stylus 0.64,
// except the data URI case, which Stylus fails to parse.
func TestSemicolonsInIndentedSyntax(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"trailing semicolon", `margin: 0 auto;`, `.a{margin:0 auto}`},
		{"two declarations", `color:black; background: #72962d`, `.a{color:black;background:#72962d}`},
		{"both, colonless", `padding 1px; margin 2px;`, `.a{padding:1px;margin:2px}`},
		{"doubled semicolon", `top 0;;`, `.a{top:0}`},
		{"lone semicolon", ";\n  color red", `.a{color:red}`},
		{"semicolon in string", `content "a;b"; width 1px`, `.a{content:"a;b";width:1px}`},
		{"escaped quote in string", `content 'it\';s'; width 1px`, `.a{content:'it\';s';width:1px}`},
		{"data uri", `background url(data:image/png;base64,AAA=); color red`, `.a{background:url(data:image/png;base64,AAA=);color:red}`},
		{"important", `color red !important;`, `.a{color:red!important}`},
		{"after a comment is stripped", "color red; // c;\n  width 1px", `.a{color:red;width:1px}`},
		{"assignment", "x = 5px;\n.a\n  width x;", `.a{width:5px}`},
		{"mixin call", "m()\n  padding 1px; margin 2px\n.a\n  m();", `.a{padding:1px;margin:2px}`},
		{"comma continuation", "transition a 1s,\n    b 2s;", `.a{transition:a 1s,b 2s}`},
		// The indented body belongs to the last statement on the line.
		{"body goes to last piece", ".a\n  color red; .b\n    top 0", `.a{color:red}.a .b{top:0}`},
		{"if condition", "x = 1\n.a\n  if x == 1\n    color red;\n  else\n    color blue;", `.a{color:red}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// A case without a top-level line of its own is the body of `.a`.
			src := c.src
			if !strings.HasPrefix(src, ".a") && !strings.Contains(src, "\n.a") {
				src = ".a\n  " + src
			}
			if got := compileMin(t, src+"\n"); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestSemicolonErrorColumn checks that an error in the second statement of a
// line points at that statement, not at the start of the line.
func TestSemicolonErrorColumn(t *testing.T) {
	_, err := styl.Compile(".a\n  color red; width 1px +\n", styl.Options{})
	if err == nil || !strings.Contains(err.Error(), "2:14") {
		t.Fatalf("want an error at 2:14, got %v", err)
	}
}

// TestSemicolonInParensBraceSyntax covers a `;` inside parentheses in brace
// syntax (N-037): it is part of the value, as in the indentation syntax, so
// an unquoted data URI survives. Stylus fails to parse an unquoted data URI
// in either syntax.
func TestSemicolonInParensBraceSyntax(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"data uri", ".a { background: url(data:image/png;base64,AAA=); color: red }",
			".a{background:url(data:image/png;base64,AAA=);color:red}"},
		{"multi-line block", ".a {\n  background: url(data:image/png;base64,AAA=);\n  top: 0;\n}",
			".a{background:url(data:image/png;base64,AAA=);top:0}"},
		{"statements after parens still split", ".a { width: calc(100% - 2px); top: 0 }",
			".a{width:calc(100% - 2px);top:0}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := compileMin(t, c.src+"\n"); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
