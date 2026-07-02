package value

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// ParseNumber parses a numeric literal such as "10px", "1.5", or "50%" into a Number.
func ParseNumber(text string) (*Number, error) {
	i := 0
	for i < len(text) && (unicode.IsDigit(rune(text[i])) || text[i] == '.') {
		i++
	}
	num, err := strconv.ParseFloat(text[:i], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number %q: %w", text, err)
	}
	return &Number{Num: num, Unit: text[i:]}, nil
}

// ParseColor parses a hex color literal (with leading '#') into a Color.
func ParseColor(text string) (*Color, error) {
	hex := strings.TrimPrefix(text, "#")
	expand := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			b.WriteRune(r)
			b.WriteRune(r)
		}
		return b.String()
	}

	switch len(hex) {
	case 3: // #rgb
		hex = expand(hex)
	case 4: // #rgba
		hex = expand(hex)
	case 6, 8:
		// already full
	default:
		return nil, fmt.Errorf("invalid hex color %q", text)
	}

	val, err := strconv.ParseUint(hex, 16, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid hex color %q: %w", text, err)
	}

	c := &Color{A: 1}
	if len(hex) == 8 {
		c.R = uint8(val >> 24)
		c.G = uint8(val >> 16)
		c.B = uint8(val >> 8)
		c.A = float64(uint8(val)) / 255
	} else {
		c.R = uint8(val >> 16)
		c.G = uint8(val >> 8)
		c.B = uint8(val)
	}
	return c, nil
}

// Arith applies a binary arithmetic operator (one of + - * ** / %) to two
// numbers, with Stylus-style unit coercion: the non-empty unit wins; if both
// are present and differ, the left operand's unit is kept.
func Arith(op string, a, b *Number) (*Number, error) {
	var r float64
	switch op {
	case "+":
		r = a.Num + b.Num
	case "-":
		r = a.Num - b.Num
	case "*":
		r = a.Num * b.Num
	case "**":
		r = math.Pow(a.Num, b.Num)
	case "/":
		if b.Num == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		r = a.Num / b.Num
	case "%":
		r = math.Mod(a.Num, b.Num)
	default:
		return nil, fmt.Errorf("unknown operator %q", op)
	}

	unit := a.Unit
	if unit == "" {
		unit = b.Unit
	}
	return &Number{Num: r, Unit: unit}, nil
}

// ColorArith applies + - * / channel-wise to a color, matching reference
// Stylus: color op color combines R/G/B and alpha pair-wise; color op number
// applies the op between each R/G/B channel and the number, leaving alpha.
// Channels round and clamp to [0,255] (only mix/tint/shade floor).
func ColorArith(op string, c *Color, rhs Value) (Value, error) {
	apply := func(a, b float64) (float64, error) {
		switch op {
		case "+":
			return a + b, nil
		case "-":
			return a - b, nil
		case "*":
			return a * b, nil
		case "/":
			if b == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return a / b, nil
		}
		return 0, fmt.Errorf("cannot apply %q to a color", op)
	}
	channels := func(r2, g2, b2 float64) (*Color, error) {
		r, err := apply(float64(c.R), r2)
		if err != nil {
			return nil, err
		}
		g, err := apply(float64(c.G), g2)
		if err != nil {
			return nil, err
		}
		b, err := apply(float64(c.B), b2)
		if err != nil {
			return nil, err
		}
		return &Color{R: roundByte(r), G: roundByte(g), B: roundByte(b), A: c.A}, nil
	}
	switch r := rhs.(type) {
	case *Color:
		out, err := channels(float64(r.R), float64(r.G), float64(r.B))
		if err != nil {
			return nil, err
		}
		// Alpha follows stylus's RGBA.operate: + adds (clamped); - keeps the
		// left alpha when the right operand is opaque, else subtracts; * and /
		// keep the left alpha.
		switch {
		case op == "+":
			out.A = clamp01(c.A + r.A)
		case op == "-" && r.A != 1:
			out.A = clamp01(c.A - r.A)
		}
		return out, nil
	case *Number:
		return channels(r.Num, r.Num, r.Num)
	}
	return nil, fmt.Errorf("cannot apply %q to color and %s", op, rhs.TypeName())
}

// Truthy reports a value's boolean interpretation, following Stylus semantics:
// null, false, 0, and "" are falsy; everything else is truthy.
func Truthy(v Value) bool {
	switch x := v.(type) {
	case Null:
		return false
	case *Bool:
		return x.Val
	case *Number:
		return x.Num != 0
	case *Str:
		return x.Val != ""
	case nil:
		return false
	default:
		return true
	}
}
