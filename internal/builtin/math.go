package builtin

import (
	"fmt"
	"math"

	"github.com/rohanthewiz/go-styl/internal/value"
)

func init() {
	register("abs", unaryMath("abs", math.Abs))
	register("ceil", unaryMath("ceil", math.Ceil))
	register("floor", unaryMath("floor", math.Floor))
	register("round", unaryMath("round", math.Round))
	register("sqrt", unaryMath("sqrt", math.Sqrt))
	register("sin", unaryMath("sin", math.Sin))
	register("cos", unaryMath("cos", math.Cos))
	register("tan", unaryMath("tan", math.Tan))
	register("min", minMax("min", false))
	register("max", minMax("max", true))
	register("pow", pow)
	register("percentage", percentage)
}

// unaryMath applies a float function to a single numeric argument, preserving
// the argument's unit.
func unaryMath(fn string, f func(float64) float64) Func {
	return func(args []value.Value) (value.Value, error) {
		if err := wantArgs(fn, args, 1); err != nil {
			return nil, err
		}
		n, err := argNum(fn, args, 0)
		if err != nil {
			return nil, err
		}
		return &value.Number{Num: f(n.Num), Unit: n.Unit}, nil
	}
}

// minMax returns the smallest/largest of its numeric arguments (keeping its unit).
func minMax(fn string, wantMax bool) Func {
	return func(args []value.Value) (value.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("%s() expects at least 1 argument", fn)
		}
		// A single list argument is treated as the set of operands.
		if len(args) == 1 {
			if l, ok := args[0].(*value.List); ok {
				args = l.Items
			}
		}
		best, err := argNum(fn, args, 0)
		if err != nil {
			return nil, err
		}
		for i := 1; i < len(args); i++ {
			n, err := argNum(fn, args, i)
			if err != nil {
				return nil, err
			}
			if (wantMax && n.Num > best.Num) || (!wantMax && n.Num < best.Num) {
				best = n
			}
		}
		return &value.Number{Num: best.Num, Unit: best.Unit}, nil
	}
}

func pow(args []value.Value) (value.Value, error) {
	if err := wantArgs("pow", args, 2); err != nil {
		return nil, err
	}
	base, err := argNum("pow", args, 0)
	if err != nil {
		return nil, err
	}
	exp, err := argNum("pow", args, 1)
	if err != nil {
		return nil, err
	}
	return &value.Number{Num: math.Pow(base.Num, exp.Num), Unit: base.Unit}, nil
}

// percentage(0.5) => 50%
func percentage(args []value.Value) (value.Value, error) {
	if err := wantArgs("percentage", args, 1); err != nil {
		return nil, err
	}
	n, err := argNum("percentage", args, 0)
	if err != nil {
		return nil, err
	}
	return &value.Number{Num: n.Num * 100, Unit: "%"}, nil
}
