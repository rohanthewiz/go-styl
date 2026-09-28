package builtin

import (
	"fmt"
	"math"
	"strconv"

	"github.com/rohanthewiz/go-styl/internal/value"
)

func init() {
	register("abs(n)", unaryMath("abs", math.Abs))
	register("ceil(n)", unaryMath("ceil", math.Ceil))
	register("floor(n)", unaryMath("floor", math.Floor))
	register("round(n)", unaryMath("round", math.Round))
	register("sqrt(n)", unaryMath("sqrt", math.Sqrt))
	register("sin(angle)", trig("sin", math.Sin))
	register("cos(angle)", trig("cos", math.Cos))
	register("tan(angle)", trig("tan", math.Tan))
	register("asin(n, unit = deg)", arcTrig("asin", math.Asin))
	register("acos(n, unit = deg)", arcTrig("acos", math.Acos))
	register("atan(n, unit = deg)", arcTrig("atan", math.Atan))
	register("radians-to-degrees(angle)", angleScale("radians-to-degrees", 180/math.Pi))
	register("degrees-to-radians(angle)", angleScale("degrees-to-radians", math.Pi/180))
	register("sum(nums)", sum)
	register("avg(nums)", avg)
	register("odd(n)", parity("odd", 1))
	register("even(n)", parity("even", 0))
	register("remove-unit(n)", removeUnit)
	register("percent-to-decimal(n)", percentToDecimal)
	register("base-convert(num, base, width = 2)", baseConvert)
	register("min(values...)", minMax("min", false))
	register("max(values...)", minMax("max", true))
	register("pow(base, exponent)", pow)
	register("percentage(n)", percentage)
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

// round9 rounds to 9 decimal places, as Stylus's trig functions do, so
// sin(180deg) is 0 rather than 1.2246467991473532e-16.
func round9(x float64) float64 {
	const m = 1e9
	return math.Round(x*m) / m
}

// trig implements sin/cos/tan as in Stylus's index.styl: a `deg` argument is
// converted to radians, any other unit is dropped, and the result is
// unitless, rounded to 9 places (sin(90deg) is 1).
func trig(fn string, f func(float64) float64) Func {
	return func(args []value.Value) (value.Value, error) {
		if err := wantArgs(fn, args, 1); err != nil {
			return nil, err
		}
		n, err := argNum(fn, args, 0)
		if err != nil {
			return nil, err
		}
		x := n.Num
		if n.Unit == "deg" {
			x = x * math.Pi / 180
		}
		return &value.Number{Num: round9(f(x))}, nil
	}
}

// angleFactors converts radians to each angle unit (Stylus's convert-angle).
var angleFactors = map[string]float64{
	"rad":  1,
	"deg":  180 / math.Pi,
	"turn": 0.5 / math.Pi,
	"grad": 200 / math.Pi,
}

// arcTrig implements asin/acos/atan(value, [unit]): the angle in `unit`
// (deg by default; rad, turn, grad also accepted), rounded to 9 places.
func arcTrig(fn string, f func(float64) float64) Func {
	return func(args []value.Value) (value.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return nil, fmt.Errorf("%s() expects 1 or 2 arguments, got %d", fn, len(args))
		}
		n, err := argNum(fn, args, 0)
		if err != nil {
			return nil, err
		}
		unit := "deg"
		if len(args) == 2 {
			unit = strVal(args[1])
		}
		factor, ok := angleFactors[unit]
		if !ok {
			return nil, fmt.Errorf("%s() unit must be deg, rad, turn or grad, got %q", fn, unit)
		}
		return &value.Number{Num: round9(f(n.Num) * factor), Unit: unit}, nil
	}
}

// angleScale multiplies by a fixed factor, keeping the unit (Stylus defines
// radians-to-degrees(a) as a * (180 / PI), which keeps a's unit).
func angleScale(fn string, factor float64) Func {
	return func(args []value.Value) (value.Value, error) {
		if err := wantArgs(fn, args, 1); err != nil {
			return nil, err
		}
		n, err := argNum(fn, args, 0)
		if err != nil {
			return nil, err
		}
		return &value.Number{Num: n.Num * factor, Unit: n.Unit}, nil
	}
}

// numbersOf returns the numbers in a list argument (or a lone number).
func numbersOf(fn string, v value.Value) ([]*value.Number, error) {
	items := asItems(v)
	out := make([]*value.Number, 0, len(items))
	for _, it := range items {
		n, ok := it.(*value.Number)
		if !ok {
			return nil, fmt.Errorf("%s() expects numbers, got %s", fn, it.TypeName())
		}
		out = append(out, n)
	}
	return out, nil
}

// sum adds a list of numbers (Stylus: sum = 0; sum += n for n in nums), so
// the result takes the first unit it meets.
func sum(args []value.Value) (value.Value, error) {
	if err := wantArgs("sum", args, 1); err != nil {
		return nil, err
	}
	nums, err := numbersOf("sum", args[0])
	if err != nil {
		return nil, err
	}
	total := &value.Number{}
	for _, n := range nums {
		if total, err = value.Arith("+", total, n); err != nil {
			return nil, err
		}
	}
	return total, nil
}

// avg is sum(nums) / length(nums).
func avg(args []value.Value) (value.Value, error) {
	if err := wantArgs("avg", args, 1); err != nil {
		return nil, err
	}
	nums, err := numbersOf("avg", args[0])
	if err != nil {
		return nil, err
	}
	if len(nums) == 0 {
		return nil, fmt.Errorf("avg() of an empty list")
	}
	total, err := sum(args)
	if err != nil {
		return nil, err
	}
	t := total.(*value.Number)
	return &value.Number{Num: t.Num / float64(len(nums)), Unit: t.Unit}, nil
}

// parity implements odd/even: n % 2 == rem.
func parity(fn string, rem float64) Func {
	return func(args []value.Value) (value.Value, error) {
		if err := wantArgs(fn, args, 1); err != nil {
			return nil, err
		}
		n, err := argNum(fn, args, 0)
		if err != nil {
			return nil, err
		}
		return &value.Bool{Val: math.Mod(n.Num, 2) == rem}, nil
	}
}

// removeUnit strips a number's unit; other values pass through.
func removeUnit(args []value.Value) (value.Value, error) {
	if err := wantArgs("remove-unit", args, 1); err != nil {
		return nil, err
	}
	if n, ok := args[0].(*value.Number); ok {
		return &value.Number{Num: n.Num}, nil
	}
	return args[0], nil
}

// percentToDecimal turns 50% into 0.5; other values pass through.
func percentToDecimal(args []value.Value) (value.Value, error) {
	if err := wantArgs("percent-to-decimal", args, 1); err != nil {
		return nil, err
	}
	if n, ok := args[0].(*value.Number); ok && n.Unit == "%" {
		return &value.Number{Num: n.Num / 100}, nil
	}
	return args[0], nil
}

// baseConvert(num, base, [width]) prints num in base, zero-padded to width
// (default 2), as an unquoted literal: base-convert(255, 16) is ff.
func baseConvert(args []value.Value) (value.Value, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("base-convert() expects 2 or 3 arguments, got %d", len(args))
	}
	n, err := argNum("base-convert", args, 0)
	if err != nil {
		return nil, err
	}
	b, err := argNum("base-convert", args, 1)
	if err != nil {
		return nil, err
	}
	base := int(b.Num)
	if base < 2 || base > 36 {
		return nil, fmt.Errorf("base-convert() base must be 2..36, got %d", base)
	}
	width := 2
	if len(args) == 3 {
		w, err := argNum("base-convert", args, 2)
		if err != nil {
			return nil, err
		}
		width = int(w.Num)
	}
	s := strconv.FormatInt(int64(n.Num), base)
	for len(s) < width {
		s = "0" + s
	}
	return &value.Str{Val: s}, nil
}
