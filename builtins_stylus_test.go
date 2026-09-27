package styl_test

import (
	"strings"
	"testing"

	"github.com/rohanthewiz/go-styl"
)

// TestStylusBuiltins covers built-ins added for N-004 (and the sin/cos/tan
// degree handling, and saturate()/invert() passing through as CSS filter
// functions). Each expected value is reference stylus 0.64's compressed
// output for the same expression, except that go-styl drops the leading
// zero of a compressed rgba() alpha (.8 where Stylus prints 0.8), a
// formatting difference the difftest normalizer already erases.
func TestStylusBuiltins(t *testing.T) {
	const vars = "cl = 1, 2\npairs = (a 1) (b 2)\n"
	cases := []struct {
		name string
		expr string
		want string
	}{
		{"sin of degrees", `sin(90deg)`, "1"},
		{"cos", `cos(0)`, "1"},
		{"tan of degrees", `tan(45deg)`, "1"},
		{"sin rounds to 9 places", `sin(180deg)`, "0"},
		{"asin", `asin(1)`, "90deg"},
		{"acos", `acos(0)`, "90deg"},
		{"atan", `atan(1)`, "45deg"},
		{"atan in rad", `atan(1, rad)`, ".785398163rad"},
		{"sum", `sum(1 2 3)`, "6"},
		{"sum keeps unit", `sum(1px 2 3)`, "6px"},
		{"avg", `avg(1 2 3 4)`, "2.5"},
		{"odd even", `odd(3) even(3)`, "true false"},
		{"remove-unit", `remove-unit(10px)`, "10"},
		{"percent-to-decimal", `percent-to-decimal(50%)`, ".5"},
		{"radians-to-degrees", `radians-to-degrees(1)`, "57.29577951308232"},
		{"degrees-to-radians", `degrees-to-radians(180)`, "3.141592653589793"},
		{"base-convert", `base-convert(255, 16)`, "ff"},
		{"base-convert width", `base-convert(10, 2, 8)`, "00001010"},
		{"fade-out", `fade-out(#000, 20%)`, "rgba(0,0,0,.8)"},
		{"fade-in", `fade-in(rgba(0,0,0,.5), 0.2)`, "rgba(0,0,0,.7)"},
		{"grayscale", `grayscale(#f00)`, "#808080"},
		{"grayscale filter", `grayscale(100%)`, "grayscale(100%)"},
		{"saturate filter", `saturate(2)`, "saturate(2)"},
		{"invert filter", `invert(1)`, "invert(1)"},
		{"luminosity", `luminosity(red)`, ".2126"},
		{"blend", `blend(rgba(#FFF, 0.5), #000)`, "#808080"},
		{"blend alphas", `blend(rgba(lime, 0.5), rgba(red, 0.25))`, "rgba(128,128,0,.625)"},
		{"transparentify", `transparentify(#808080)`, "rgba(0,0,0,.5)"},
		{"transparentify over black", `transparentify(#414141, #000)`, "rgba(255,255,255,.25)"},
		{"transparentify alpha", `transparentify(#91974C, #F34949, 0.5)`, "rgba(47,229,79,.5)"},
		{"component rgb", `component(#f00, 'red')`, "255"},
		{"component hue", `component(hsl(120, 50%, 50%), 'hue')`, "120deg"},
		{"opposite-position", `opposite-position(top left)`, "bottom right"},
		{"basename", `basename("a/b/c.png")`, "'c.png'"},
		{"basename ext", `basename("a/b/c.png", ".png")`, "'c'"},
		{"dirname", `dirname("a/b/c.png")`, "'a/b'"},
		{"extname", `extname("a/b/c.png")`, "'.png'"},
		{"pathjoin", `pathjoin("a", "b", "c.png")`, "'a/b/c.png'"},
		{"pop shift", `pop(1 2 3) shift(1 2 3)`, "3 1"},
		{"range step", `range(1, 5, 2)`, "1 3 5"},
		{"range units", `range(0px, 2px)`, "0 1px 2px"},
		{"list-separator", `list-separator(1 2) list-separator(cl)`, "' ' ','"},
		{"keys values", `keys(pairs) values(pairs)`, "a b 1 2"},
		{"convert", `convert("10px") + 1`, "11px"},
		{"clone", `clone(1 2)`, "1 2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := compileMin(t, vars+".a\n  k "+c.expr+"\n")
			if want := ".a{k:" + c.want + "}"; got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// TestBuiltinAsStatement checks a built-in called on its own line: its
// value is a function's implicit return, as in Stylus.
func TestBuiltinAsStatement(t *testing.T) {
	if got, want := compileMin(t, "f(n)\n  round(n)\n.a\n  w f(1.4)\n"), ".a{w:1}"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestErrorBuiltin checks error(msg) stops compilation with the message at
// the calling line.
func TestErrorBuiltin(t *testing.T) {
	src := "check(n)\n  if n < 0\n    error('n must be positive')\n  n\n.a\n  width check(-1)\n"
	_, err := styl.Compile(src, styl.Options{})
	if err == nil || !strings.Contains(err.Error(), "n must be positive") {
		t.Fatalf("want the error() message, got %v", err)
	}
}
