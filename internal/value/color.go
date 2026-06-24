package value

import "math"

// HSL returns the color's hue (degrees [0,360)), saturation, and lightness
// (both [0,1]). Alpha is carried separately on the Color.
func (c *Color) HSL() (h, s, l float64) {
	r := float64(c.R) / 255
	g := float64(c.G) / 255
	b := float64(c.B) / 255

	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l = (max + min) / 2

	if max == min {
		return 0, 0, l // achromatic
	}

	d := max - min
	if l > 0.5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}

	switch max {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default: // b
		h = (r-g)/d + 4
	}
	h *= 60
	return h, s, l
}

// NewColorHSL builds a Color from hue (degrees), saturation/lightness ([0,1]),
// and alpha ([0,1]).
func NewColorHSL(h, s, l, a float64) *Color {
	h = math.Mod(math.Mod(h, 360)+360, 360) / 360 // normalize to [0,1)
	s = clamp01(s)
	l = clamp01(l)

	var r, g, b float64
	if s == 0 {
		r, g, b = l, l, l // achromatic
	} else {
		var q float64
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = l + s - l*s
		}
		p := 2*l - q
		r = hue2rgb(p, q, h+1.0/3.0)
		g = hue2rgb(p, q, h)
		b = hue2rgb(p, q, h-1.0/3.0)
	}
	return &Color{
		R: roundByte(r * 255),
		G: roundByte(g * 255),
		B: roundByte(b * 255),
		A: clamp01(a),
	}
}

func hue2rgb(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 1.0/2.0:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	default:
		return p
	}
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func roundByte(x float64) uint8 {
	if x < 0 {
		x = 0
	}
	if x > 255 {
		x = 255
	}
	return uint8(math.Round(x))
}

// LookupNamedColor resolves a CSS color keyword (e.g. "rebeccapurple") to a Color.
func LookupNamedColor(name string) (*Color, bool) {
	hex, ok := namedColors[name]
	if !ok {
		return nil, false
	}
	c, err := ParseColor(hex)
	if err != nil {
		return nil, false
	}
	return c, true
}
