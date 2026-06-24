package value

import "testing"

func TestNumberCSS(t *testing.T) {
	cases := []struct {
		num         float64
		unit        string
		pretty, min string
	}{
		{20, "px", "20px", "20px"},
		{0.5, "", "0.5", ".5"},
		{-0.25, "em", "-0.25em", "-.25em"},
		{1.5, "rem", "1.5rem", "1.5rem"},
	}
	for _, c := range cases {
		n := &Number{Num: c.num, Unit: c.unit}
		if got := n.CSS(true); got != c.pretty {
			t.Errorf("CSS(pretty) %v%s = %q, want %q", c.num, c.unit, got, c.pretty)
		}
		if got := n.CSS(false); got != c.min {
			t.Errorf("CSS(min) %v%s = %q, want %q", c.num, c.unit, got, c.min)
		}
	}
}

func TestArith(t *testing.T) {
	cases := []struct {
		op      string
		a, b    *Number
		wantNum float64
		wantU   string
	}{
		{"*", &Number{Num: 10, Unit: "px"}, &Number{Num: 2}, 20, "px"},
		{"+", &Number{Num: 1, Unit: "em"}, &Number{Num: 2, Unit: "em"}, 3, "em"},
		{"/", &Number{Num: 10, Unit: "px"}, &Number{Num: 4}, 2.5, "px"},
		{"-", &Number{Num: 5}, &Number{Num: 8, Unit: "pt"}, -3, "pt"},
		{"%", &Number{Num: 10}, &Number{Num: 3}, 1, ""},
	}
	for _, c := range cases {
		got, err := Arith(c.op, c.a, c.b)
		if err != nil {
			t.Fatalf("Arith(%s): %v", c.op, err)
		}
		if got.Num != c.wantNum || got.Unit != c.wantU {
			t.Errorf("Arith(%s) = %v%s, want %v%s", c.op, got.Num, got.Unit, c.wantNum, c.wantU)
		}
	}
}

func TestParseColorCSS(t *testing.T) {
	cases := []struct{ in, pretty string }{
		{"#fff", "#fff"},
		{"#ffffff", "#fff"},
		{"#abcdef", "#abcdef"},
		{"#000000", "#000"},
	}
	for _, c := range cases {
		col, err := ParseColor(c.in)
		if err != nil {
			t.Fatalf("ParseColor(%s): %v", c.in, err)
		}
		if got := col.CSS(true); got != c.pretty {
			t.Errorf("ParseColor(%s).CSS = %q, want %q", c.in, got, c.pretty)
		}
	}
}
