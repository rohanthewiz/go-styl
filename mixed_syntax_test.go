package styl_test

import "testing"

// TestMixedSyntax covers files that combine the indentation and brace syntaxes.
// Outside braces the source indentation decides nesting; inside braces the
// braces do. Every expected value was checked against reference stylus 0.64
// (which additionally rewrites named colors to hex).
func TestMixedSyntax(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			"indented mixin with brace children",
			"m()\n  c = red\n  .a {\n    color: c;\n    .b { width: 1px }\n  }\n  .d\n    color blue\n.x\n  m()",
			".x .a{color:red}.x .a .b{width:1px}.x .d{color:blue}",
		},
		{
			"brace block then indented rules",
			".a {\n  color: red;\n}\n.b\n  color blue\n  .c\n    width 1px",
			".a{color:red}.b{color:blue}.b .c{width:1px}",
		},
		{
			"stray indent after a closed brace block is a sibling",
			".a {\n  color: red;\n}\n  .b { color: blue; }",
			".a{color:red}.b{color:blue}",
		},
		{
			"brace on its own line opens the previous header",
			".p\n  .a\n  {\n    color: red;\n  }\n  .b\n    color blue",
			".p .a{color:red}.p .b{color:blue}",
		},
		{
			"same-line brace blocks then indented rule",
			".a { color: red; } .b { color: blue; }\n.c\n  color green",
			".a{color:red}.b{color:blue}.c{color:green}",
		},
		{
			"brace one-liner inside indented rule",
			".p\n  .a { color: red }\n  .b\n    color blue",
			".p .a{color:red}.p .b{color:blue}",
		},
		{
			"semicolons in indented lines of a mixed file",
			"m()\n  .w\n    position: relative;\n    width: 100%;\n    .h {\n      top: 0;\n    }\n    .k\n      left: 0\n.x\n  m()",
			".x .w{position:relative;width:100%}.x .w .h{top:0}.x .w .k{left:0}",
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
