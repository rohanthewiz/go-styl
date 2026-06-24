package builtin

import (
	"fmt"
	"regexp"

	"github.com/rohanthewiz/go-styl/internal/value"
)

func init() {
	register("typeof", typeOf)
	register("type", typeOf)
	register("unit", unit)
	register("match", match)
	register("light", light)
	register("dark", dark)
}

func typeOf(args []value.Value) (value.Value, error) {
	if err := wantArgs("typeof", args, 1); err != nil {
		return nil, err
	}
	return &value.Str{Val: args[0].TypeName(), Quote: 0}, nil
}

// unit(n) returns n's unit as a string; unit(n, u) returns n with unit u.
func unit(args []value.Value) (value.Value, error) {
	if len(args) == 1 {
		n, err := argNum("unit", args, 0)
		if err != nil {
			return nil, err
		}
		return &value.Str{Val: n.Unit, Quote: 0}, nil
	}
	if err := wantArgs("unit", args, 2); err != nil {
		return nil, err
	}
	n, err := argNum("unit", args, 0)
	if err != nil {
		return nil, err
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

// match(pattern, str) reports whether str matches the regular expression pattern.
func match(args []value.Value) (value.Value, error) {
	if err := wantArgs("match", args, 2); err != nil {
		return nil, err
	}
	pat := strVal(args[0])
	subject := strVal(args[1])
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, fmt.Errorf("match(): invalid pattern %q: %w", pat, err)
	}
	return &value.Bool{Val: re.MatchString(subject)}, nil
}

func light(args []value.Value) (value.Value, error) {
	c, err := getColorArg("light", args)
	if err != nil {
		return nil, err
	}
	_, _, l := c.HSL()
	return &value.Bool{Val: l >= 0.5}, nil
}

func dark(args []value.Value) (value.Value, error) {
	c, err := getColorArg("dark", args)
	if err != nil {
		return nil, err
	}
	_, _, l := c.HSL()
	return &value.Bool{Val: l < 0.5}, nil
}
