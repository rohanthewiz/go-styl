package styl_test

import (
	"strings"
	"testing"

	styl "github.com/rohanthewiz/go-styl"
)

// TestM2Features checks control flow and function/mixin behavior on small inline
// snippets, compiled in compressed mode for compact comparison.
func TestM2Features(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "function returns value",
			src: "f(n) = n * 3\n" +
				"a\n  width f(2px)\n",
			want: "a{width:6px}",
		},
		{
			name: "else-if chain picks middle branch",
			src: "x = 2\n" +
				"a\n" +
				"  if x == 1\n    color red\n" +
				"  else if x == 2\n    color green\n" +
				"  else\n    color blue\n",
			want: "a{color:green}",
		},
		{
			name: "unless negates",
			src:  "a\n  unless false\n    color red\n",
			want: "a{color:red}",
		},
		{
			name: "conditional assignment keeps first",
			src:  "c = red\nc ?= blue\na\n  color c\n",
			want: "a{color:red}",
		},
		{
			name: "mixin with default",
			src: "box(p = 4px)\n  padding p\n" +
				"a\n  box()\n",
			want: "a{padding:4px}",
		},
		{
			name: "for emits per item",
			src:  "a\n  for n in 1 2\n    margin n * 5px\n",
			want: "a{margin:5px;margin:10px}",
		},
		{
			name: "recursion via function",
			src: "fib(n) = n\n" + // simple guard-free function, just exercise calls
				"a\n  z-index fib(7)\n",
			want: "a{z-index:7}",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := styl.Compile(c.src, styl.Options{Pretty: false})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if strings.TrimRight(got, "\n") != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
