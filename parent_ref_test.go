package styl_test

import "testing"

// TestParentReferenceAnywhere covers `&` placed anywhere in a nested selector
// (N-025): each `&` is replaced by the parent selector and nothing is prepended.
// Every expected value was checked against reference stylus 0.64 (which
// additionally rewrites named colors to hex).
func TestParentReferenceAnywhere(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"trailing parent ref", ".x\n  .a\n    .checkbox &\n      color red", ".checkbox .x .a{color:red}"},
		{"parent group", ".a, .b\n  .c &\n    color red", ".c .a,.c .b{color:red}"},
		{"two refs", ".x\n  & + &\n    color red", ".x + .x{color:red}"},
		{"ref mid-selector", ".x\n  html.ie &.y\n    color red", "html.ie .x.y{color:red}"},
		{"suffix still glued", ".x\n  &-icon\n    color red", ".x-icon{color:red}"},
		{"inside :not()", ".x\n  :not(&)\n    color red", ":not(.x){color:red}"},
		{"inside attribute string", ".x\n  [data-a=\"&\"]\n    color red", `[data-a=".x"]{color:red}`},
		{"mixed group", ".x\n  .p &, .q\n    color red", ".p .x,.x .q{color:red}"},
		{"children of a ref rule", ".x\n  .a &\n    .b\n      color red", ".a .x .b{color:red}"},
		{"nested ref under a ref rule", ".x\n  .a &\n    &:hover\n      color red", ".a .x:hover{color:red}"},
		{"after a combinator", ".x\n  > &\n    color red", "> .x{color:red}"},
		{"top-level ref resolves to nothing", ".checkbox &\n  color red", ".checkbox{color:red}"},
		{"top-level leading ref", "& .a\n  color red", ".a{color:red}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := compileMin(t, c.src+"\n"); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
