package builtin

import (
	"fmt"
	"math"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/value"
)

func init() {
	register("rgb(red, green, blue)", rgb)
	register("rgba(red, green, blue, alpha) | rgba(color, alpha)", rgba)
	register("hsl(hue, saturation, lightness)", hsl)
	register("hsla(hue, saturation, lightness, alpha)", hsla)
	register("red(color)", channelGetter("red", func(c *value.Color) float64 { return float64(c.R) }))
	register("green(color)", channelGetter("green", func(c *value.Color) float64 { return float64(c.G) }))
	register("blue(color)", channelGetter("blue", func(c *value.Color) float64 { return float64(c.B) }))
	register("alpha(color, value?)", alpha)
	register("hue(color)", hue)
	register("saturation(color)", saturation)
	register("lightness(color)", lightness)
	register("lighten(color, amount)", lighten)
	register("darken(color, amount)", darken)
	register("spin(color, degrees)", spin)
	register("saturate(color, amount)", saturate)
	register("desaturate(color, amount)", desaturate)
	register("mix(color1, color2, weight = 50%)", mix)
	register("tint(color, amount)", tint)
	register("shade(color, amount)", shade)
	register("complement(color)", complement)
	register("invert(color)", invert)
	register("fade-out(color, amount)", fade("fade-out", -1))
	register("fade-in(color, amount)", fade("fade-in", 1))
	register("grayscale(color)", grayscale)
	register("luminosity(color)", luminosity)
	register("blend(top, bottom = white)", blendOver)
	register("contrast(top, bottom = white)", contrast)
	register("transparentify(top, bottom = white, alpha?)", transparentify)
	register("component(color, name)", component)
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

// adjustHSL applies a change to one HSL component and returns the new color.
func adjustHSL(fn string, args []value.Value, apply func(h, s, l float64, amt *value.Number) (float64, float64, float64)) (value.Value, error) {
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
	h, s, l = apply(h, s, l, amt)
	return value.NewColorHSL(h, s, l, c.A), nil
}

// hslDelta is the change reference Stylus's adjust() applies to an HSL
// component (cur in [0,1]) for a signed amount: a `%` amount is relative —
// increasing lightness scales into its remaining headroom, everything else
// scales the component itself — while a unitless amount adds absolute
// percentage points.
func hslDelta(component string, cur float64, amt *value.Number, sign float64) float64 {
	v := sign * amt.Num / 100
	if amt.Unit != "%" {
		return v
	}
	if component == "lightness" && v > 0 {
		return (1 - cur) * v
	}
	return cur * v
}

func lighten(args []value.Value) (value.Value, error) {
	return adjustHSL("lighten", args, func(h, s, l float64, amt *value.Number) (float64, float64, float64) {
		return h, s, l + hslDelta("lightness", l, amt, +1)
	})
}

func darken(args []value.Value) (value.Value, error) {
	return adjustHSL("darken", args, func(h, s, l float64, amt *value.Number) (float64, float64, float64) {
		return h, s, l + hslDelta("lightness", l, amt, -1)
	})
}

// spin rotates the hue by amount degrees.
func spin(args []value.Value) (value.Value, error) {
	return adjustHSL("spin", args, func(h, s, l float64, amt *value.Number) (float64, float64, float64) {
		return h + amt.Num, s, l
	})
}

// saturate(color, amount) raises saturation. With anything but a color
// first it is the CSS filter function and passes through (filter:
// saturate(2)), as in Stylus's index.styl.
func saturate(args []value.Value) (value.Value, error) {
	if len(args) == 0 || !isColor(args[0]) {
		return literalCall("saturate", args), nil
	}
	return adjustHSL("saturate", args, func(h, s, l float64, amt *value.Number) (float64, float64, float64) {
		return h, s + hslDelta("saturation", s, amt, +1), l
	})
}

func desaturate(args []value.Value) (value.Value, error) {
	return adjustHSL("desaturate", args, func(h, s, l float64, amt *value.Number) (float64, float64, float64) {
		return h, s + hslDelta("saturation", s, amt, -1), l
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

// blend linearly mixes two colors; w is the weight of c1. Channels are
// floored, not rounded, matching reference Stylus's mix()/tint()/shade()
// (mix(#fff, #000) is #7f7f7f there, while #fff * 0.5 rounds to #808080).
func blend(c1, c2 *value.Color, w float64) *value.Color {
	w = clampAlpha(w)
	mixCh := func(a, b uint8) uint8 {
		return uint8(math.Floor(float64(a)*w + float64(b)*(1-w)))
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

// invert(color) is the RGB negative. With anything but a color it is the
// CSS filter function and passes through (filter: invert(1)).
func invert(args []value.Value) (value.Value, error) {
	if len(args) != 1 || !isColor(args[0]) {
		return literalCall("invert", args), nil
	}
	c, err := getColorArg("invert", args)
	if err != nil {
		return nil, err
	}
	return &value.Color{R: 255 - c.R, G: 255 - c.G, B: 255 - c.B, A: c.A}, nil
}

// fade implements fade-in/fade-out(color, amount): alpha moves by amount (a
// fraction: 20% or 0.2), clamped to [0, 1]. Stylus defines these as
// color ± rgba(black, percent-to-decimal(amount)).
func fade(fn string, sign float64) Func {
	return func(args []value.Value) (value.Value, error) {
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
		return &value.Color{R: c.R, G: c.G, B: c.B, A: clampAlpha(c.A + sign*fraction(amt))}, nil
	}
}

// grayscale(color) is desaturate(color, 100%). Anything else is the CSS
// filter function and passes through: filter: grayscale(100%).
func grayscale(args []value.Value) (value.Value, error) {
	if len(args) == 1 {
		if c, ok := toColor(args[0]); ok {
			h, _, l := c.HSL()
			return value.NewColorHSL(h, 0, l, c.A), nil
		}
	}
	return literalCall("grayscale", args), nil
}

// isColor reports whether v is a color or a color keyword.
func isColor(v value.Value) bool {
	_, ok := toColor(v)
	return ok
}

// literalCall renders a call as plain CSS text, for a builtin that shares
// its name with a CSS function and got non-Stylus arguments.
func literalCall(name string, args []value.Value) value.Value {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = a.CSS(true)
	}
	return &value.Ident{Name: name + "(" + strings.Join(parts, ", ") + ")"}
}

// relLuminance is WCAG 2.0 relative luminance
// (https://www.w3.org/TR/WCAG20/#relativeluminancedef), with Stylus's
// 0.03928 threshold.
func relLuminance(c *value.Color) float64 {
	ch := func(v uint8) float64 {
		x := float64(v) / 255
		if x < 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c.R) + 0.7152*ch(c.G) + 0.0722*ch(c.B)
}

// luminosity(color) is the color's relative luminance, 0 (black) to 1
// (white): luminosity(red) is 0.2126.
func luminosity(args []value.Value) (value.Value, error) {
	c, err := getColorArg("luminosity", args)
	if err != nil {
		return nil, err
	}
	return &value.Number{Num: relLuminance(c)}, nil
}

// blendOver(top, [bottom = white]) composites top over bottom by top's alpha
// (Stylus's blend()): blend(rgba(#fff, .5), #000) is #808080.
func blendOver(args []value.Value) (value.Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("blend() expects 1 or 2 arguments, got %d", len(args))
	}
	top, err := argColor("blend", args, 0)
	if err != nil {
		return nil, err
	}
	bottom := &value.Color{R: 255, G: 255, B: 255, A: 1}
	if len(args) == 2 {
		if bottom, err = argColor("blend", args, 1); err != nil {
			return nil, err
		}
	}
	a := top.A
	mixCh := func(t, b uint8) uint8 { return clampByte(float64(t)*a + float64(b)*(1-a)) }
	return &value.Color{
		R: mixCh(top.R, bottom.R),
		G: mixCh(top.G, bottom.G),
		B: mixCh(top.B, bottom.B),
		A: clampAlpha(a + bottom.A - a*bottom.A),
	}, nil
}

// contrast(top, [bottom = white]) is the WCAG contrast ratio of text colored
// top over a bottom background, as an object (Stylus's contrast):
//
//	contrast(#000, #fff) → {ratio: 21, error: 0, min: 21, max: 21}
//
// A translucent top is first blended over bottom. With a translucent bottom
// the real background is unknown (it depends on what lies under it), so the
// result is a range: max is the better of the ratios over black and over
// white backings, min is the ratio over the backing that makes bottom look
// most like top, ratio is their midpoint and error half their spread. Ratios
// round to one decimal place; ratio and error to two.
//
// With anything but a color it is the CSS filter function and passes through
// (filter: contrast(1.5)).
func contrast(args []value.Value) (value.Value, error) {
	if len(args) < 1 || len(args) > 2 || !isColor(args[0]) {
		return literalCall("contrast", args), nil
	}
	top, err := argColor("contrast", args, 0)
	if err != nil {
		return nil, err
	}
	bottom := &value.Color{R: 255, G: 255, B: 255, A: 1}
	if len(args) == 2 {
		if bottom, err = argColor("contrast", args, 1); err != nil {
			return nil, err
		}
	}
	// over composites a over b with a's alpha, rounding channels the way
	// Stylus's RGBA constructor does (blendOver's math).
	over := func(a, b *value.Color) *value.Color {
		v, _ := blendOver([]value.Value{a, b})
		return v.(*value.Color)
	}
	ratioOf := func(t, b *value.Color) float64 {
		if t.A < 1 {
			t = over(t, b)
		}
		l1 := relLuminance(b) + 0.05
		l2 := relLuminance(t) + 0.05
		r := l1 / l2
		if l2 > l1 {
			r = 1 / r
		}
		return math.Round(r*10) / 10
	}

	out := value.NewHash()
	num := func(f float64) value.Value { return &value.Number{Num: f} }
	if bottom.A >= 1 {
		r := ratioOf(top, bottom)
		out.Set("ratio", num(r))
		out.Set("error", num(0))
		out.Set("min", num(r))
		out.Set("max", num(r))
		return out, nil
	}
	black := &value.Color{A: 1}
	white := &value.Color{R: 255, G: 255, B: 255, A: 1}
	onBlack := ratioOf(top, over(bottom, black))
	onWhite := ratioOf(top, over(bottom, white))
	maxR := math.Max(onBlack, onWhite)
	// closest is the opaque backing under which bottom comes out nearest to
	// top: solve top = bottom·a + backing·(1-a) per channel, clamped.
	ch := func(t, b uint8) uint8 {
		x := (float64(t) - float64(b)*bottom.A) / (1 - bottom.A)
		return clampByte(math.Min(math.Max(0, x), 255))
	}
	closest := &value.Color{R: ch(top.R, bottom.R), G: ch(top.G, bottom.G), B: ch(top.B, bottom.B), A: 1}
	minR := ratioOf(top, over(bottom, closest))
	out.Set("ratio", num(math.Round((minR+maxR)*50)/100))
	out.Set("error", num(math.Round((maxR-minR)*50)/100))
	out.Set("min", num(minR))
	out.Set("max", num(maxR))
	return out, nil
}

// transparentify(top, [bottom = white], [alpha]) finds the most transparent
// color that looks like top when laid over bottom (Stylus's
// transparentify): transparentify(#808080) is rgba(0,0,0,0.5). An explicit
// alpha (0.5 or 50%) fixes the opacity instead. A lone second argument that
// is a number, not a color, is the alpha.
func transparentify(args []value.Value) (value.Value, error) {
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("transparentify() expects 1 to 3 arguments, got %d", len(args))
	}
	top, err := argColor("transparentify", args, 0)
	if err != nil {
		return nil, err
	}
	bottom := &value.Color{R: 255, G: 255, B: 255, A: 1}
	var alphaArg value.Value
	switch {
	case len(args) == 3:
		if bottom, err = argColor("transparentify", args, 1); err != nil {
			return nil, err
		}
		alphaArg = args[2]
	case len(args) == 2:
		if c, ok := toColor(args[1]); ok {
			bottom = c
		} else {
			alphaArg = args[1]
		}
	}

	t := [3]float64{float64(top.R), float64(top.G), float64(top.B)}
	b := [3]float64{float64(bottom.R), float64(bottom.G), float64(bottom.B)}
	// The alpha each channel needs, toward 255 when the top channel is
	// above the bottom and toward 0 otherwise; the largest wins. A channel
	// that doesn't differ needs none (0/0 counts as 0).
	best := math.Inf(-1)
	for i := range t {
		d := t[i] - b[i]
		target := 0.0
		if d > 0 {
			target = 255
		}
		a := d / (target - b[i])
		if math.IsNaN(a) {
			a = 0
		}
		best = math.Max(best, a)
	}
	if alphaArg != nil {
		n, ok := alphaArg.(*value.Number)
		if !ok {
			return nil, fmt.Errorf("transparentify() alpha must be a number, got %s", alphaArg.TypeName())
		}
		best = fraction(n)
	}
	best = math.Max(math.Min(best, 1), 0)
	ch := func(i int) uint8 {
		if best == 0 {
			return clampByte(b[i])
		}
		return clampByte(b[i] + (t[i]-b[i])/best)
	}
	return &value.Color{R: ch(0), G: ch(1), B: ch(2), A: math.Round(best*100) / 100}, nil
}

// component(color, name) returns one component: red, green, blue (0-255),
// alpha (0-1), hue (deg), saturation or lightness (%).
func component(args []value.Value) (value.Value, error) {
	if err := wantArgs("component", args, 2); err != nil {
		return nil, err
	}
	c, err := argColor("component", args, 0)
	if err != nil {
		return nil, err
	}
	h, sat, l := c.HSL()
	switch name := strVal(args[1]); name {
	case "red":
		return &value.Number{Num: float64(c.R)}, nil
	case "green":
		return &value.Number{Num: float64(c.G)}, nil
	case "blue":
		return &value.Number{Num: float64(c.B)}, nil
	case "alpha":
		return &value.Number{Num: c.A}, nil
	case "hue":
		return &value.Number{Num: h, Unit: "deg"}, nil
	case "saturation":
		return &value.Number{Num: sat * 100, Unit: "%"}, nil
	case "lightness":
		return &value.Number{Num: l * 100, Unit: "%"}, nil
	default:
		return nil, fmt.Errorf("invalid color component %q", name)
	}
}
