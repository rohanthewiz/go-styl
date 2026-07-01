package styl_test

import (
	"strings"
	"testing"

	styl "github.com/rohanthewiz/go-styl"
)

// TestM6Correctness covers the M6a correctness fixes, compiled compressed.
func TestM6Correctness(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		// url() / calc() literals
		{"unquoted url", "a\n  background url(/img/x.png)", "a{background:url(/img/x.png)}"},
		{"url with list", "a\n  background url(/x.png) no-repeat", "a{background:url(/x.png) no-repeat}"},
		{"calc literal", "a\n  width calc(100% - 20px)", "a{width:calc(100% - 20px)}"},
		{"calc interpolation", "g = 16px\na\n  width calc(100% - {g})", "a{width:calc(100% - 16px)}"},

		// !important
		{"important pretty-less", "a\n  color red !important", "a{color:red!important}"},
		{"important keeps value", "a\n  margin 0 auto !important", "a{margin:0 auto!important}"},

		// whitespace-sensitive call parens: `f(x)` is a call, `f (x)` a list
		{"spaced paren is list", "g = 12px\na\n  padding g (g * 2)", "a{padding:12px 24px}"},
		{"spaced paren declaration", "a\n  margin (10px)", "a{margin:10px}"},
		{"glued paren is call", "a\n  color rgba(0,0,0,.5)", "a{color:rgba(0,0,0,.5)}"},

		// whitespace-sensitive -/+
		{"space-dash is list", "a\n  margin 10px -5px", "a{margin:10px -5px}"},
		{"binary subtraction", "a\n  width 10px - 5px", "a{width:5px}"},
		{"glued is subtraction", "a\n  width 10px-5px", "a{width:5px}"},
		{"leading negative", "a\n  top -5px", "a{top:-5px}"},
		{"multi list with negatives", "a\n  inset 1px 2px -3px 4px", "a{inset:1px 2px -3px 4px}"},

		// property-value slash: literal unless parenthesized (font: 14px/1.5)
		{"prop slash literal", "x = 20px\na\n  line-height x/2", "a{line-height:20px/2}"},
		{"prop slash parens divide", "x = 20px\na\n  width (x/2)", "a{width:10px}"},
		{"prop slash in list", "x = 20px\na\n  margin x/2 auto", "a{margin:20px/2 auto}"},
		{"font shorthand slash", "a\n  font 14px/1.5 Arial", "a{font:14px/1.5 Arial}"},
		{"assignment slash divides", "x = 20px\nz = x/2\na\n  width z", "a{width:10px}"},
		{"call args slash divides", "a\n  width min(10px/2, 9px)", "a{width:5px}"},

		// selector comma-splitting
		{"comma in attr", `a[href$="a,b"]` + "\n  color red", `a[href$="a,b"]{color:red}`},

		// media query variable
		{"media var", "bp = 50em\n@media (min-width: bp)\n  .a\n    x 1", "@media (min-width:50em){.a{x:1}}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := styl.Compile(c.src+"\n", styl.Options{Pretty: false})
			if err != nil {
				t.Fatalf("compile %q: %v", c.src, err)
			}
			if g := strings.TrimRight(got, "\n"); g != c.want {
				t.Errorf("got %q, want %q", g, c.want)
			}
		})
	}
}

// TestM6SelectorSplitParens checks commas inside :not() do not split the group.
func TestM6SelectorSplitParens(t *testing.T) {
	got, err := styl.Compile(":not(.x, .y)\n  color red\n", styl.Options{Pretty: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(got), ":not(.x, .y) {") {
		t.Errorf("selector was split inside :not(): %q", got)
	}
}

// TestM6ImportantPretty verifies the spaced form in expanded output.
func TestM6ImportantPretty(t *testing.T) {
	got, err := styl.Compile("a\n  color red !important\n", styl.Options{Pretty: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !strings.Contains(got, "color: red !important;") {
		t.Errorf("missing spaced !important: %q", got)
	}
}
