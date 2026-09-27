package styl_test

import "testing"

// TestMultiLineSelectors covers selector groups that span lines (N-024): a
// trailing-comma continuation, and stacked selector lines sharing the block of
// the last one. Unless a case notes otherwise, its expected value was checked
// against reference stylus 0.64. Stylus also rewrites named colors to hex and
// keeps spaces around combinators in compressed output.
func TestMultiLineSelectors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		// --- (a) trailing-comma continuation ---
		{"trailing comma at top level", ".a,\n.b\n  color red", ".a,.b{color:red}"},
		{"trailing comma nested", ".x\n  .a,\n  .b\n    color red", ".x .a,.x .b{color:red}"},
		{"continuation indented deeper", ".a,\n  .b\n  color red", ".a,.b{color:red}"},
		{"several commas on a line", ".a, .b,\n.c\n  color red", ".a,.b,.c{color:red}"},
		{"blank line inside group", ".x\n  .a,\n\n  .b\n    color red", ".x .a,.x .b{color:red}"},
		{"comment inside group", ".x\n  .a,\n    // c\n  .b\n    color red", ".x .a,.x .b{color:red}"},
		{"brace syntax", ".a,\n.b {\n  color: red;\n}", ".a,.b{color:red}"},
		{"multi-line value list", ".x\n  transition opacity 1s,\n    transform 2s", ".x{transition:opacity 1s,transform 2s}"},

		// --- (b) stacked selector lines ---
		{"parent-ref pseudos", ".x\n  &:after\n  &:before\n    content ''", ".x:after,.x:before{content:''}"},
		{"nth-child not a declaration", "table\n  td:nth-child(1)\n  td:nth-child(2)\n    width 10px", "table td:nth-child(1),table td:nth-child(2){width:10px}"},
		{"top-level stack", ".a\n.b\n  color red", ".a,.b{color:red}"},
		{"bare type selectors", ".x\n  a\n  b\n    width 1px", ".x a,.x b{width:1px}"},
		{"combinators", ".x\n  > .a\n  + .b\n  ~ .c\n    width 1px", ".x>.a,.x+.b,.x~.c{width:1px}"},
		{"pseudo-classes", ".x\n  a:hover\n  a:focus\n    width 1px", ".x a:hover,.x a:focus{width:1px}"},
		{"attribute then type", ".x\n  [type=text]\n  input\n    width 1px", ".x [type=text],.x input{width:1px}"},
		{"universal", ".x\n  *\n  .b\n    color red", ".x *,.x .b{color:red}"},
		{"brace syntax stack", ".x {\n  &:after\n  &:before {\n    content: '';\n  }\n}", ".x:after,.x:before{content:''}"},
		// Stylus reads a bare identifier above a selector block as a type
		// selector, even when a mixin of that name exists.
		{"bare ident over block is a selector", "m()\n  color red\n.x\n  m\n  .y\n    width 1px", ".x m,.x .y{width:1px}"},

		// --- not selector groups ---
		{"declaration then block", ".x\n  color red\n  .y\n    width 1px", ".x{color:red}.x .y{width:1px}"},
		{"colon declaration then block", ".x\n  display:block\n  &:hover\n    color red", ".x{display:block}.x:hover{color:red}"},
		{"pseudo-named value", ".x\n  cursor:default\n  .y\n    width 1px", ".x{cursor:default}.x .y{width:1px}"},
		{"mixin call with parens then block", "m()\n  color red\n.x\n  m()\n  .y\n    width 1px", ".x{color:red}.x .y{width:1px}"},
		// A go-styl extension: Stylus evaluates a bare `m` here to nothing and
		// outputs only `.x{width:1px}`. This checks that the stacking rule
		// leaves the bare call alone when no selector block follows.
		{"bare mixin call then declaration", "m()\n  color red\n.x\n  m\n  width 1px", ".x{color:red;width:1px}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := compileMin(t, c.src+"\n"); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
