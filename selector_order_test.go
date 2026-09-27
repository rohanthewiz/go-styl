package styl_test

import "testing"

// TestNestedGroupOrder pins the order of selectors when both a parent and a
// child are groups: child-major, as in reference stylus 0.64 (found while
// verifying N-022 against cema). Order within a group doesn't change what
// matches, but it keeps output byte-compatible with Stylus.
func TestNestedGroupOrder(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"two levels", ".a, .b\n  .x, .y\n    top 0", ".a .x,.b .x,.a .y,.b .y{top:0}"},
		{"pseudo-classes", ".a, .b\n  &:hover, &:focus\n    top 0", ".a:hover,.b:hover,.a:focus,.b:focus{top:0}"},
		{"three levels", ".a, .b\n  .c, .d\n    .e, .f\n      top 0",
			".a .c .e,.b .c .e,.a .d .e,.b .d .e,.a .c .f,.b .c .f,.a .d .f,.b .d .f{top:0}"},
		{"trailing parent refs", ".p\n  .a, .b\n    .checkbox &, .radio &\n      top 0",
			".checkbox .p .a,.checkbox .p .b,.radio .p .a,.radio .p .b{top:0}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := compileMin(t, c.src+"\n"); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
