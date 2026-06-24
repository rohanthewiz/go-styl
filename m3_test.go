package styl_test

import (
	"strings"
	"testing"

	styl "github.com/rohanthewiz/go-styl"
)

// TestBuiltins exercises one representative call per built-in across the
// color/math/list/string/type categories, compiled compressed for compact
// comparison. Each source is a single rule `a` with one declaration `v`.
func TestBuiltins(t *testing.T) {
	cases := []struct {
		name string
		expr string
		want string // expected value of `v`
	}{
		// color
		{"rgb", "rgb(255, 0, 0)", "#f00"},
		{"rgba set alpha", "rgba(#000, 0.5)", "rgba(0,0,0,.5)"},
		{"hsl", "hsl(120, 50%, 50%)", "#40bf40"},
		{"hsla", "hsla(0, 100%, 50%, 0.5)", "rgba(255,0,0,.5)"},
		{"red channel", "red(#ff8800)", "255"},
		{"alpha getter", "alpha(rgba(0, 0, 0, 0.4))", ".4"},
		{"hue", "hue(#00ffff)", "180deg"},
		{"saturation", "saturation(#00ffff)", "100%"},
		{"lighten", "lighten(#000, 50%)", "#808080"},
		{"darken", "darken(#fff, 25%)", "#bfbfbf"},
		{"mix", "mix(#fff, #000)", "#808080"},
		{"tint", "tint(#000, 50%)", "#808080"},
		{"complement", "complement(red)", "#0ff"},
		{"invert", "invert(#000)", "#fff"},
		// math
		{"abs", "abs(-5px)", "5px"},
		{"ceil", "ceil(4.1)", "5"},
		{"floor", "floor(4.9)", "4"},
		{"round", "round(3.5px)", "4px"},
		{"min", "min(3px, 9px, 5px)", "3px"},
		{"max", "max(3px, 9px, 5px)", "9px"},
		{"pow", "pow(2, 10)", "1024"},
		{"percentage", "percentage(0.25)", "25%"},
		// list
		{"length", "length(1 2 3)", "3"},
		{"index", "index(a b c, b)", "1"},
		{"join", `join("-", 1 2 3)`, "1-2-3"},
		{"last", "last(1 2 3)", "3"},
		{"push", "length(push(1 2, 3))", "3"},
		// string
		{"quote", "quote(hi)", `"hi"`},
		{"unquote", `unquote("hi")`, "hi"},
		{"uppercase", `uppercase("abc")`, `"ABC"`},
		{"substr", `substr("hello", 1, 3)`, `"ell"`},
		{"replace", `replace("o", "0", "foo")`, `"f00"`},
		{"sprintf", `s("%s-%s", a, b)`, "a-b"},
		// type
		{"typeof unit", "typeof(10px)", "unit"},
		{"typeof color", "typeof(#fff)", "color"},
		{"unit getter", "unit(10px)", "px"},
		{"unit setter", "unit(10, em)", "10em"},
		{"match true", `match("^a", "abc")`, "true"},
		{"light", "light(#fff)", "true"},
		{"dark", "dark(#000)", "true"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := "a\n  v " + c.expr + "\n"
			got, err := styl.Compile(src, styl.Options{Pretty: false})
			if err != nil {
				t.Fatalf("compile %q: %v", c.expr, err)
			}
			want := "a{v:" + c.want + "}"
			if strings.TrimRight(got, "\n") != want {
				t.Errorf("%s => %q, want %q", c.expr, got, want)
			}
		})
	}
}
