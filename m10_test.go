package styl_test

import (
	"strings"
	"testing"

	styl "github.com/rohanthewiz/go-styl"
)

// TestM10CompatGaps covers the reference-Stylus features added for parity:
// the ** operator, range loops, color arithmetic, implicit function returns,
// transparent mixin calls, and compressed zero-length units. Expected values
// verified against npm stylus 0.64.0.
func TestM10CompatGaps(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		// ** shares the multiplicative precedence level and associates left
		{"pow", "a\n  z-index 2 ** 3", "a{z-index:8}"},
		{"pow left assoc", "a\n  z-index 2 ** 3 ** 2", "a{z-index:64}"},
		{"pow same level as star", "a\n  z-index 2 * 3 ** 2", "a{z-index:36}"},
		{"pow keeps left unit", "a\n  width 2px ** 2", "a{width:4px}"},
		{"pow of negation", "a\n  z-index -2 ** 2", "a{z-index:4}"},

		// ranges: .. inclusive, ... excludes the bound, descending works
		{"range inclusive", "for n in 1..3\n  .w-{n}\n    x n", ".w-1{x:1}.w-2{x:2}.w-3{x:3}"},
		{"range exclusive", "for n in 0...3\n  .h-{n}\n    x n", ".h-0{x:0}.h-1{x:1}.h-2{x:2}"},
		{"range descending", "for n in 3..1\n  .d-{n}\n    x n", ".d-3{x:3}.d-2{x:2}.d-1{x:1}"},
		{"range is a list", "r = 1..3\na\n  len length(r)", "a{len:3}"},

		// color arithmetic (channel-wise, rounded, clamped)
		{"color plus color", "a\n  color #111 + #222", "a{color:#333}"},
		{"color plus clamps", "a\n  color #888 + #888", "a{color:#fff}"},
		{"color minus color", "a\n  color #333 - #111", "a{color:#222}"},
		{"color plus number", "a\n  color #333 + 10", "a{color:#3d3d3d}"},
		{"color times number", "a\n  color #112233 * 2", "a{color:#246}"},
		{"alpha subtracts when translucent", "a\n  color rgba(#888, 0.8) - rgba(#111, 0.3)", "a{color:rgba(119,119,119,.5)}"},
		{"alpha kept when rhs opaque", "a\n  color rgba(#888, 0.5) - #111", "a{color:rgba(119,119,119,.5)}"},

		// implicit returns: a function body's last expression is its value
		{"implicit return", "double(n)\n  n * 2\na\n  width double(15px)", "a{width:30px}"},
		{"implicit return in branches", "w(k)\n  if k == heavy\n    700\n  else\n    400\na\n  font-weight w(heavy)", "a{font-weight:700}"},
		{"implicit return bare ident", "f(n)\n  n\na\n  width f(5px)", "a{width:5px}"},
		{"last expression wins", "f()\n  1\n  2\na\n  z-index f()", "a{z-index:2}"},
		{"explicit return beats later exprs", "f()\n  return 1\n  2\na\n  z-index f()", "a{z-index:1}"},

		// transparent mixin calls: `m args...` invokes a mixin in scope
		{"transparent space args", "m(a, b = 9px)\n  wa a\n  wb b\n.t\n  m 1px 2px", ".t{wa:1px;wb:2px}"},
		{"transparent comma args", "m(a, b)\n  wa a\n  wb b\n.t\n  m 1px, 2px", ".t{wa:1px;wb:2px}"},
		{"transparent default arg", "m(a, b = 9px)\n  wa a\n  wb b\n.t\n  m 1px", ".t{wa:1px;wb:9px}"},
		{"transparent self is property", "border-radius(n)\n  -webkit-border-radius n\n  border-radius n\n.t\n  border-radius 3px", ".t{-webkit-border-radius:3px;border-radius:3px}"},

		// compressed zero drops length units, keeps %/time/angle
		{"zero px compressed", "a\n  height 0px", "a{height:0}"},
		{"zero percent kept", "a\n  height 0%", "a{height:0%}"},
		{"zero seconds kept", "a\n  transition-delay 0s", "a{transition-delay:0s}"},
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

// TestM10ZeroUnitPretty checks pretty output keeps zero-length units (only
// compressed output strips them, as in reference stylus).
func TestM10ZeroUnitPretty(t *testing.T) {
	got, err := styl.Compile("a\n  height 0px\n", styl.Options{Pretty: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "height: 0px;") {
		t.Errorf("pretty output should keep 0px, got %q", got)
	}
}

// TestM10RangeGuards checks oversized ranges error instead of exhausting memory.
func TestM10RangeGuards(t *testing.T) {
	_, err := styl.Compile("for n in 1..99999999\n  .x-{n}\n    y n\n", styl.Options{})
	if err == nil || !strings.Contains(err.Error(), "range") {
		t.Errorf("want range-size error, got %v", err)
	}
}
