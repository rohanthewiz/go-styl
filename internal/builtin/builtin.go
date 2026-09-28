// Package builtin holds the registry of built-in Stylus functions, organized by
// category (color.go, math.go, list.go, string.go, type.go). Each category file
// registers its functions via init(); this file holds the registry and the
// argument helpers shared across categories.
package builtin

import (
	"fmt"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/value"
)

// Func is a built-in function implementation.
type Func func(args []value.Value) (value.Value, error)

// Registry maps function names to their implementations. Categories populate it
// through register() in their init() functions.
var Registry = map[string]Func{}

// signatures maps function names to their parameter lists as a user would
// write the call (`darken(color, amount)`), for editor tooling: signature
// help and hover. Optional parameters show their default (`step = 1`) or,
// when the default isn't a simple value, a trailing `?`; a rest parameter
// ends in `...`. A function with more than one calling form lists each,
// separated by " | ".
var signatures = map[string]string{}

// register adds a built-in under the name its signature starts with.
//
// The signature, not a bare name, is the key so that a built-in can't be
// registered without one: the parameter list lives on the same line as the
// registration and changes with it, instead of in a table elsewhere that
// drifts out of date.
func register(sig string, f Func) {
	name, _, ok := strings.Cut(sig, "(")
	if !ok {
		panic("builtin: register signature without a parameter list: " + sig)
	}
	Registry[name] = f
	signatures[name] = sig
}

// Signatures returns name's calling forms (usually one; rgba has two), or
// nil if name isn't a registered built-in.
func Signatures(name string) []string {
	sig, ok := signatures[name]
	if !ok {
		return nil
	}
	return strings.Split(sig, " | ")
}

// Lookup returns the built-in for name, if any.
func Lookup(name string) (Func, bool) {
	f, ok := Registry[name]
	return f, ok
}

// --- argument helpers ---

// wantArgs verifies an exact argument count.
func wantArgs(fn string, args []value.Value, n int) error {
	if len(args) != n {
		return fmt.Errorf("%s() expects %d argument(s), got %d", fn, n, len(args))
	}
	return nil
}

// argNum extracts arg i as a Number.
func argNum(fn string, args []value.Value, i int) (*value.Number, error) {
	n, ok := args[i].(*value.Number)
	if !ok {
		return nil, fmt.Errorf("%s() argument %d must be a number, got %s", fn, i+1, args[i].TypeName())
	}
	return n, nil
}

// argColor extracts arg i as a color, accepting Color values and color keywords
// (e.g. the ident `blue`).
func argColor(fn string, args []value.Value, i int) (*value.Color, error) {
	c, ok := toColor(args[i])
	if !ok {
		return nil, fmt.Errorf("%s() argument %d must be a color, got %s", fn, i+1, args[i].TypeName())
	}
	return c, nil
}

// toColor coerces a value to a Color: Color values pass through and color
// keywords (idents) are looked up in the CSS named-color table.
func toColor(v value.Value) (*value.Color, bool) {
	switch x := v.(type) {
	case *value.Color:
		return x, true
	case *value.Ident:
		return value.LookupNamedColor(x.Name)
	}
	return nil, false
}

// fraction interprets a number as a fraction: a `%` value is divided by 100,
// a bare value is taken as-is (so both `20%` and `0.2` mean 0.2).
func fraction(n *value.Number) float64 {
	if n.Unit == "%" {
		return n.Num / 100
	}
	return n.Num
}

func clampByte(x float64) uint8 {
	if x < 0 {
		x = 0
	}
	if x > 255 {
		x = 255
	}
	return uint8(x + 0.5)
}

func clampAlpha(a float64) float64 {
	if a < 0 {
		return 0
	}
	if a > 1 {
		return 1
	}
	return a
}
