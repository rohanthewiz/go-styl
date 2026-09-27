package styl_test

import (
	"strings"
	"testing"
)

// TestStringOperations covers string concatenation with `+` and length() of
// a string (N-036). Expected values match reference stylus 0.64.
func TestStringOperations(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"length counts characters", `a length("abc")`, `.a{a:3}`},
		{"length of empty string", `a length("")`, `.a{a:0}`},
		{"length counts runes", `a length("héllo")`, `.a{a:5}`},
		{"length of a list", `a length(a b c)`, `.a{a:3}`},
		{"length of a list of strings", `a length(("abc" "de"))`, `.a{a:2}`},
		{"length of an ident", `a length(foo)`, `.a{a:1}`},
		{"string + string", `b "x" + "y"`, `.a{b:'xy'}`},
		{"mixed quotes", `b 'x' + "y"`, `.a{b:'xy'}`},
		{"string + ident", "y = foo\n.a\n  b \"x\" + y", `.a{b:'xfoo'}`},
		{"string + unit", `b "x" + 1px`, `.a{b:'x1px'}`},
		{"string + color", `b "x" + #fff`, `.a{b:'x#fff'}`},
		{"string + list", `b "x" + (1 2)`, `.a{b:'x1 2'}`},
		{"string + null", `b "w" + null`, `.a{b:'wnull'}`},
		{"chained", `b "a" + "b" + "c"`, `.a{b:'abc'}`},
		{"unquoted left stays unquoted", `b unquote("a") + "b"`, `.a{b:ab}`},
		{"s() result + unit", `b s("%s", 1) + "px"`, `.a{b:1px}`},
		{"number + number unchanged", `b 1px + 2`, `.a{b:3px}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := c.src
			if !strings.Contains(src, "\n") {
				src = ".a\n  " + src
			}
			if got := compileMin(t, src+"\n"); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
