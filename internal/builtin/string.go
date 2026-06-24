package builtin

import (
	"fmt"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/value"
)

func init() {
	register("unquote", unquote)
	register("quote", quote)
	register("s", sprintf)
	register("uppercase", caseFn("uppercase", strings.ToUpper))
	register("lowercase", caseFn("lowercase", strings.ToLower))
	register("substr", substr)
	register("replace", replace)
	register("split", split)
}

// strVal returns the textual content of a value (a Str's raw text, or any value's
// CSS form).
func strVal(v value.Value) string {
	if s, ok := v.(*value.Str); ok {
		return s.Val
	}
	return v.CSS(true)
}

func unquote(args []value.Value) (value.Value, error) {
	if err := wantArgs("unquote", args, 1); err != nil {
		return nil, err
	}
	return &value.Str{Val: strVal(args[0]), Quote: 0}, nil
}

func quote(args []value.Value) (value.Value, error) {
	if err := wantArgs("quote", args, 1); err != nil {
		return nil, err
	}
	return &value.Str{Val: strVal(args[0]), Quote: '"'}, nil
}

// sprintf implements Stylus's s(): %s placeholders are filled from the remaining
// arguments in order.
func sprintf(args []value.Value) (value.Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("s() expects at least 1 argument")
	}
	format := strVal(args[0])
	rest := args[1:]
	var b strings.Builder
	ai := 0
	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) && format[i+1] == 's' {
			if ai < len(rest) {
				b.WriteString(rest[ai].CSS(true))
				ai++
			}
			i++
			continue
		}
		b.WriteByte(format[i])
	}
	return &value.Str{Val: b.String(), Quote: 0}, nil
}

func caseFn(fn string, f func(string) string) Func {
	return func(args []value.Value) (value.Value, error) {
		if err := wantArgs(fn, args, 1); err != nil {
			return nil, err
		}
		s, ok := args[0].(*value.Str)
		if !ok {
			return nil, fmt.Errorf("%s() argument must be a string, got %s", fn, args[0].TypeName())
		}
		return &value.Str{Val: f(s.Val), Quote: s.Quote}, nil
	}
}

// substr(str, start, [length]) returns a substring (runes), Stylus-style.
func substr(args []value.Value) (value.Value, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("substr() expects 2 or 3 arguments, got %d", len(args))
	}
	runes := []rune(strVal(args[0]))
	startN, err := argNum("substr", args, 1)
	if err != nil {
		return nil, err
	}
	start := int(startN.Num)
	if start < 0 {
		start += len(runes)
	}
	if start < 0 {
		start = 0
	}
	if start > len(runes) {
		start = len(runes)
	}
	end := len(runes)
	if len(args) == 3 {
		lenN, err := argNum("substr", args, 2)
		if err != nil {
			return nil, err
		}
		end = start + int(lenN.Num)
		if end > len(runes) {
			end = len(runes)
		}
		if end < start {
			end = start
		}
	}
	return &value.Str{Val: string(runes[start:end]), Quote: quoteOf(args[0])}, nil
}

func replace(args []value.Value) (value.Value, error) {
	if err := wantArgs("replace", args, 3); err != nil {
		return nil, err
	}
	s := strVal(args[2])
	old := strVal(args[0])
	with := strVal(args[1])
	return &value.Str{Val: strings.ReplaceAll(s, old, with), Quote: quoteOf(args[2])}, nil
}

// split(delim, str) splits a string into a comma list of strings.
func split(args []value.Value) (value.Value, error) {
	if err := wantArgs("split", args, 2); err != nil {
		return nil, err
	}
	delim := strVal(args[0])
	parts := strings.Split(strVal(args[1]), delim)
	items := make([]value.Value, len(parts))
	for i, p := range parts {
		items[i] = &value.Str{Val: p, Quote: 0}
	}
	return &value.List{Items: items, Comma: true}, nil
}

func quoteOf(v value.Value) rune {
	if s, ok := v.(*value.Str); ok {
		return s.Quote
	}
	return 0
}
