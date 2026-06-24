package builtin

import (
	"fmt"

	"github.com/rohanthewiz/go-styl/internal/value"
)

func init() {
	register("rgb", rgb)
	register("rgba", rgba)
	register("hsl", hsl)
	register("hsla", hsla)
	register("red", channelGetter("red", func(c *value.Color) float64 { return float64(c.R) }))
	register("green", channelGetter("green", func(c *value.Color) float64 { return float64(c.G) }))
	register("blue", channelGetter("blue", func(c *value.Color) float64 { return float64(c.B) }))
	register("alpha", alpha)
	register("hue", hue)
	register("saturation", saturation)
	register("lightness", lightness)
	register("lighten", lighten)
	register("darken", darken)
	register("saturate", saturate)
	register("desaturate", desaturate)
	register("mix", mix)
	register("tint", tint)
	register("shade", shade)
	register("complement", complement)
	register("invert", invert)
}

func rgb(args []value.Value) (value.Value, error) {
	if err := wantArgs("rgb", args, 3); err != nil {
		return nil, err
	}
	r, g, b, err := rgbChannels("rgb", args)
	if err != nil {
		return nil, err
	}
	return &value.Color{R: r, G: g, B: b, A: 1}, nil
}

func rgba(args []value.Value) (value.Value, error) {
	// rgba(color, alpha) sets the alpha of an existing color.
	if len(args) == 2 {
		c, err := argColor("rgba", args, 0)
		if err != nil {
			return nil, err
		}
		a, err := argNum("rgba", args, 1)
		if err != nil {
			return nil, err
		}
		return &value.Color{R: c.R, G: c.G, B: c.B, A: clampAlpha(fraction(a))}, nil
	}
	if err := wantArgs("rgba", args, 4); err != nil {
		return nil, err
	}
	r, g, b, err := rgbChannels("rgba", args)
	if err != nil {
		return nil, err
	}
	a, err := argNum("rgba", args, 3)
	if err != nil {
		return nil, err
	}
	return &value.Color{R: r, G: g, B: b, A: clampAlpha(fraction(a))}, nil
}

func rgbChannels(fn string, args []value.Value) (r, g, b uint8, err error) {
	rn, err := argNum(fn, args, 0)
	if err != nil {
		return
	}
	gn, err := argNum(fn, args, 1)
	if err != nil {
		return
	}
	bn, err := argNum(fn, args, 2)
	if err != nil {
		return
	}
	return clampByte(rn.Num), clampByte(gn.Num), clampByte(bn.Num), nil
}

func hsl(args []value.Value) (value.Value, error) {
	if err := wantArgs("hsl", args, 3); err != nil {
		return nil, err
	}
	return makeHSL("hsl", args, 1)
}

func hsla(args []value.Value) (value.Value, error) {
	if err := wantArgs("hsla", args, 4); err != nil {
		return nil, err
	}
	return makeHSL("hsla", args, -1)
}

// makeHSL builds a color from h,s,l arguments; if alphaIdx >= 0 it is the fixed
// alpha, otherwise the alpha is read from args[3].
func makeHSL(fn string, args []value.Value, fixedAlpha float64) (value.Value, error) {
	h, err := argNum(fn, args, 0)
	if err != nil {
		return nil, err
	}
	s, err := argNum(fn, args, 1)
	if err != nil {
		return nil, err
	}
	l, err := argNum(fn, args, 2)
	if err != nil {
		return nil, err
	}
	a := fixedAlpha
	if fixedAlpha < 0 {
		an, err := argNum(fn, args, 3)
		if err != nil {
			return nil, err
		}
		a = fraction(an)
	}
	return value.NewColorHSL(h.Num, fraction(s), fraction(l), clampAlpha(a)), nil
}

// channelGetter builds a getter for an rgb channel (returns a unitless number).
func channelGetter(fn string, get func(*value.Color) float64) Func {
	return func(args []value.Value) (value.Value, error) {
		if err := wantArgs(fn, args, 1); err != nil {
			return nil, err
		}
		c, err := argColor(fn, args, 0)
		if err != nil {
			return nil, err
		}
		return &value.Number{Num: get(c)}, nil
	}
}

// alpha(color) returns the alpha; alpha(color, n) sets it.
func alpha(args []value.Value) (value.Value, error) {
	if len(args) == 2 {
		c, err := argColor("alpha", args, 0)
		if err != nil {
			return nil, err
		}
		a, err := argNum("alpha", args, 1)
		if err != nil {
			return nil, err
		}
		return &value.Color{R: c.R, G: c.G, B: c.B, A: clampAlpha(fraction(a))}, nil
	}
	if err := wantArgs("alpha", args, 1); err != nil {
		return nil, err
	}
	c, err := argColor("alpha", args, 0)
	if err != nil {
		return nil, err
	}
	return &value.Number{Num: c.A}, nil
}

func hue(args []value.Value) (value.Value, error) {
	c, err := getColorArg("hue", args)
	if err != nil {
		return nil, err
	}
	h, _, _ := c.HSL()
	return &value.Number{Num: h, Unit: "deg"}, nil
}

func saturation(args []value.Value) (value.Value, error) {
	c, err := getColorArg("saturation", args)
	if err != nil {
		return nil, err
	}
	_, s, _ := c.HSL()
	return &value.Number{Num: s * 100, Unit: "%"}, nil
}

func lightness(args []value.Value) (value.Value, error) {
	c, err := getColorArg("lightness", args)
	if err != nil {
		return nil, err
	}
	_, _, l := c.HSL()
	return &value.Number{Num: l * 100, Unit: "%"}, nil
}

func getColorArg(fn string, args []value.Value) (*value.Color, error) {
	if err := wantArgs(fn, args, 1); err != nil {
		return nil, err
	}
	return argColor(fn, args, 0)
}

// adjustHSL applies delta to one HSL component and returns the new color.
func adjustHSL(fn string, args []value.Value, apply func(h, s, l, amt float64) (float64, float64, float64)) (value.Value, error) {
	if err := wantArgs(fn, args, 2); err != nil {
		return nil, err
	}
	c, err := argColor(fn, args, 0)
	if err != nil {
		return nil, err
	}
	amt, err := argNum(fn, args, 1)
	if err != nil {
		return nil, err
	}
	h, s, l := c.HSL()
	h, s, l = apply(h, s, l, fraction(amt))
	return value.NewColorHSL(h, s, l, c.A), nil
}

func lighten(args []value.Value) (value.Value, error) {
	return adjustHSL("lighten", args, func(h, s, l, amt float64) (float64, float64, float64) {
		return h, s, l + amt
	})
}

func darken(args []value.Value) (value.Value, error) {
	return adjustHSL("darken", args, func(h, s, l, amt float64) (float64, float64, float64) {
		return h, s, l - amt
	})
}

func saturate(args []value.Value) (value.Value, error) {
	return adjustHSL("saturate", args, func(h, s, l, amt float64) (float64, float64, float64) {
		return h, s + amt, l
	})
}

func desaturate(args []value.Value) (value.Value, error) {
	return adjustHSL("desaturate", args, func(h, s, l, amt float64) (float64, float64, float64) {
		return h, s - amt, l
	})
}

// mix(c1, c2, [weight=50%]) blends two colors; weight is the proportion of c1.
func mix(args []value.Value) (value.Value, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("mix() expects 2 or 3 arguments, got %d", len(args))
	}
	c1, err := argColor("mix", args, 0)
	if err != nil {
		return nil, err
	}
	c2, err := argColor("mix", args, 1)
	if err != nil {
		return nil, err
	}
	w := 0.5
	if len(args) == 3 {
		wn, err := argNum("mix", args, 2)
		if err != nil {
			return nil, err
		}
		w = fraction(wn)
	}
	return blend(c1, c2, w), nil
}

func tint(args []value.Value) (value.Value, error) {
	return mixWith("tint", args, &value.Color{R: 255, G: 255, B: 255, A: 1})
}

func shade(args []value.Value) (value.Value, error) {
	return mixWith("shade", args, &value.Color{R: 0, G: 0, B: 0, A: 1})
}

// mixWith blends a color toward base by the given amount (proportion of base).
func mixWith(fn string, args []value.Value, base *value.Color) (value.Value, error) {
	if err := wantArgs(fn, args, 2); err != nil {
		return nil, err
	}
	c, err := argColor(fn, args, 0)
	if err != nil {
		return nil, err
	}
	amt, err := argNum(fn, args, 1)
	if err != nil {
		return nil, err
	}
	return blend(base, c, fraction(amt)), nil
}

// blend linearly mixes two colors; w is the weight of c1.
func blend(c1, c2 *value.Color, w float64) *value.Color {
	w = clampAlpha(w)
	mixCh := func(a, b uint8) uint8 {
		return clampByte(float64(a)*w + float64(b)*(1-w))
	}
	return &value.Color{
		R: mixCh(c1.R, c2.R),
		G: mixCh(c1.G, c2.G),
		B: mixCh(c1.B, c2.B),
		A: c1.A*w + c2.A*(1-w),
	}
}

func complement(args []value.Value) (value.Value, error) {
	c, err := getColorArg("complement", args)
	if err != nil {
		return nil, err
	}
	h, s, l := c.HSL()
	return value.NewColorHSL(h+180, s, l, c.A), nil
}

func invert(args []value.Value) (value.Value, error) {
	c, err := getColorArg("invert", args)
	if err != nil {
		return nil, err
	}
	return &value.Color{R: 255 - c.R, G: 255 - c.G, B: 255 - c.B, A: c.A}, nil
}
