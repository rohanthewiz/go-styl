// Package builtin holds the registry of built-in Stylus functions. M1 ships a
// minimal set (rgba, unit) to exercise the function-call path end to end; the
// full color/math/list/string library arrives in M3.
package builtin

import (
	"fmt"

	"github.com/rohanthewiz/go-styl/internal/value"
)

// Func is a built-in function implementation.
type Func func(args []value.Value) (value.Value, error)

// Registry maps function names to their implementations.
var Registry = map[string]Func{
	"rgba": rgba,
	"rgb":  rgb,
	"unit": unit,
}

// Lookup returns the built-in for name, if any.
func Lookup(name string) (Func, bool) {
	f, ok := Registry[name]
	return f, ok
}

func rgba(args []value.Value) (value.Value, error) {
	if len(args) != 4 {
		return nil, fmt.Errorf("rgba() expects 4 arguments, got %d", len(args))
	}
	r, err := channel(args[0], "rgba")
	if err != nil {
		return nil, err
	}
	g, err := channel(args[1], "rgba")
	if err != nil {
		return nil, err
	}
	b, err := channel(args[2], "rgba")
	if err != nil {
		return nil, err
	}
	a, ok := args[3].(*value.Number)
	if !ok {
		return nil, fmt.Errorf("rgba() alpha must be a number, got %s", args[3].TypeName())
	}
	return &value.Color{R: r, G: g, B: b, A: clampAlpha(a.Num)}, nil
}

func rgb(args []value.Value) (value.Value, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("rgb() expects 3 arguments, got %d", len(args))
	}
	r, err := channel(args[0], "rgb")
	if err != nil {
		return nil, err
	}
	g, err := channel(args[1], "rgb")
	if err != nil {
		return nil, err
	}
	b, err := channel(args[2], "rgb")
	if err != nil {
		return nil, err
	}
	return &value.Color{R: r, G: g, B: b, A: 1}, nil
}

func unit(args []value.Value) (value.Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("unit() expects 2 arguments, got %d", len(args))
	}
	n, ok := args[0].(*value.Number)
	if !ok {
		return nil, fmt.Errorf("unit() first argument must be a number, got %s", args[0].TypeName())
	}
	var u string
	switch x := args[1].(type) {
	case *value.Str:
		u = x.Val
	case *value.Ident:
		u = x.Name
	default:
		return nil, fmt.Errorf("unit() second argument must be a string, got %s", args[1].TypeName())
	}
	return &value.Number{Num: n.Num, Unit: u}, nil
}

func channel(v value.Value, fn string) (uint8, error) {
	n, ok := v.(*value.Number)
	if !ok {
		return 0, fmt.Errorf("%s() channel must be a number, got %s", fn, v.TypeName())
	}
	x := n.Num
	if x < 0 {
		x = 0
	}
	if x > 255 {
		x = 255
	}
	return uint8(x + 0.5), nil
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
