package styl_test

import (
	"strings"
	"testing"
)

// TestURLVariables covers variables inside url() (N-001). As in reference
// stylus 0.64, a url() whose contents reference a variable is evaluated and
// printed quoted; go-styl leaves any other url() verbatim (Stylus would also
// quote those, a formatting-only difference).
func TestURLVariables(t *testing.T) {
	const vars = "p = \"a.png\"\nbase = \"/img/\"\n"
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"bare variable", `a url(p)`, `.a{a:url("a.png")}`},
		{"string concatenation", `a url(base + "x.png")`, `.a{a:url("/img/x.png")}`},
		{"two variables", `a url(base + p)`, `.a{a:url("/img/a.png")}`},
		{"list runs together", `a url(p p)`, `.a{a:url("a.pnga.png")}`},
		{"undefined name stays verbatim", `a url(nothere)`, `.a{a:url(nothere)}`},
		{"path stays verbatim", `a url(/x/y.png)`, `.a{a:url(/x/y.png)}`},
		{"quoted stays verbatim", `a url("q.png")`, `.a{a:url("q.png")}`},
		{"absolute url stays verbatim", `a url(http://x.com/a.png)`, `.a{a:url(http://x.com/a.png)}`},
		{"data uri stays verbatim", `a url(data:image/png;base64,AAA=)`, `.a{a:url(data:image/png;base64,AAA=)}`},
		{"slash path with variable-like word", `a url(p/b.png)`, `.a{a:url(p/b.png)}`},
		{"interpolation still works", "d = img\n.a\n  a url({d}/x.png)", `.a{a:url(img/x.png)}`},
		{"inside a mixin", "icon(f)\n  background url(base + f)\n.a\n  icon(\"i.svg\")", `.a{background:url("/img/i.svg")}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// A one-line case is a declaration inside `.a`; a multi-line
			// one is a full stylesheet after the shared variables.
			src := vars + ".a\n  " + c.src + "\n"
			if strings.Contains(c.src, "\n") {
				src = vars + c.src + "\n"
			}
			if got := compileMin(t, src); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
