package styl_test

import (
	"strings"
	"testing"
)

// TestListIndexing covers subscripts on lists (N-008). Expected values match
// reference stylus 0.64.
func TestListIndexing(t *testing.T) {
	const vars = "r = 1px 2px 3px\nlst = a, b, x\ns = 5px\ni = 1\n"
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"second item", `a r[1]`, `.a{a:2px}`},
		{"negative counts from the end", `a r[-1]`, `.a{a:3px}`},
		{"past the end is null", `a r[5]`, `.a{a:}`},
		{"comma list", `a lst[2]`, `.a{a:x}`},
		{"scalar index 0 is itself", `a s[0]`, `.a{a:5px}`},
		{"scalar index 1 is null", `a s[1]`, `.a{a:}`},
		{"expression index", `a r[i + 1]`, `.a{a:3px}`},
		{"in arithmetic", `a r[0] + 1`, `.a{a:2px}`},
		{"parenthesized list", `a (1 2 3)[1]`, `.a{a:2}`},
		{"list of indexes", `a r[0 1]`, `.a{a:1px 2px}`},
		{"range index", `a r[0..1]`, `.a{a:1px 2px}`},
		{"parenthesized variable", `a (r)[1]`, `.a{a:2px}`},
		{"two subscripts in a list", `a r[0] r[2]`, `.a{a:1px 3px}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := compileMin(t, vars+".a\n  "+c.src+"\n"); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestListIndexingInControlFlow checks subscripts in a function body, an if
// condition and a for loop's list.
func TestListIndexingInControlFlow(t *testing.T) {
	src := strings.Join([]string{
		"second(l)",
		"  l[1]",
		"r = a b c",
		".a",
		"  x second(r)",
		"  if r[0] == a",
		"    y yes",
		"  for v in r[1..2]",
		"    z v",
	}, "\n") + "\n"
	if got, want := compileMin(t, src), `.a{x:b;y:yes;z:b;z:c}`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
