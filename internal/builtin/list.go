package builtin

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/rohanthewiz/go-styl/internal/value"
)

func init() {
	register("length", length)
	register("push", push)
	register("append", push)
	register("unshift", unshift)
	register("prepend", unshift)
	register("index", index)
	register("last", last)
	register("first", first)
	register("join", join)
	register("pop", pop)
	register("shift", shift)
	register("range", rangeFn)
	register("list-separator", listSeparator)
	register("keys", pairPart("keys", 0))
	register("values", pairPart("values", 1))
	register("clone", clone)
}

// asItems returns the elements a value represents as a list: a List's items, an
// empty slice for null, or a single-element slice otherwise.
func asItems(v value.Value) []value.Value {
	switch x := v.(type) {
	case *value.List:
		return x.Items
	case value.Null:
		return nil
	default:
		return []value.Value{v}
	}
}

func length(args []value.Value) (value.Value, error) {
	if err := wantArgs("length", args, 1); err != nil {
		return nil, err
	}
	// A single string counts its characters, as in Stylus (length("abc") is
	// 3). Anything else counts list items; a lone value is a one-item list.
	if s, ok := args[0].(*value.Str); ok {
		return &value.Number{Num: float64(utf8.RuneCountInString(s.Val))}, nil
	}
	return &value.Number{Num: float64(len(asItems(args[0])))}, nil
}

func push(args []value.Value) (value.Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("push() expects at least 2 arguments, got %d", len(args))
	}
	items := append([]value.Value{}, asItems(args[0])...)
	items = append(items, args[1:]...)
	return &value.List{Items: items, Comma: listComma(args[0])}, nil
}

func unshift(args []value.Value) (value.Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("unshift() expects at least 2 arguments, got %d", len(args))
	}
	items := append([]value.Value{}, args[1:]...)
	items = append(items, asItems(args[0])...)
	return &value.List{Items: items, Comma: listComma(args[0])}, nil
}

func index(args []value.Value) (value.Value, error) {
	if err := wantArgs("index", args, 2); err != nil {
		return nil, err
	}
	target := args[1].CSS(true)
	for i, it := range asItems(args[0]) {
		if it.CSS(true) == target {
			return &value.Number{Num: float64(i)}, nil
		}
	}
	return value.Null{}, nil
}

func last(args []value.Value) (value.Value, error) {
	if err := wantArgs("last", args, 1); err != nil {
		return nil, err
	}
	items := asItems(args[0])
	if len(items) == 0 {
		return value.Null{}, nil
	}
	return items[len(items)-1], nil
}

func first(args []value.Value) (value.Value, error) {
	if err := wantArgs("first", args, 1); err != nil {
		return nil, err
	}
	items := asItems(args[0])
	if len(items) == 0 {
		return value.Null{}, nil
	}
	return items[0], nil
}

// join(sep, list) joins a list's items into a string with the given separator.
func join(args []value.Value) (value.Value, error) {
	if err := wantArgs("join", args, 2); err != nil {
		return nil, err
	}
	sep, ok := args[0].(*value.Str)
	sepStr := ""
	if ok {
		sepStr = sep.Val
	} else {
		sepStr = args[0].CSS(true)
	}
	parts := make([]string, 0)
	for _, it := range asItems(args[1]) {
		parts = append(parts, it.CSS(true))
	}
	return &value.Str{Val: strings.Join(parts, sepStr), Quote: 0}, nil
}

func listComma(v value.Value) bool {
	if l, ok := v.(*value.List); ok {
		return l.Comma
	}
	return false
}

// pop returns a list's last item (null for an empty list). Stylus's pop also
// removes it from the list variable in place; go-styl values are immutable,
// so the list is unchanged, like push() returning a new list.
func pop(args []value.Value) (value.Value, error) {
	if err := wantArgs("pop", args, 1); err != nil {
		return nil, err
	}
	items := asItems(args[0])
	if len(items) == 0 {
		return value.Null{}, nil
	}
	return items[len(items)-1], nil
}

// shift returns a list's first item (null for an empty list); see pop for
// the in-place difference from Stylus.
func shift(args []value.Value) (value.Value, error) {
	if err := wantArgs("shift", args, 1); err != nil {
		return nil, err
	}
	items := asItems(args[0])
	if len(items) == 0 {
		return value.Null{}, nil
	}
	return items[0], nil
}

// maxRange caps range() so a tiny step can't allocate without bound.
const maxRange = 100000

// rangeFn implements range(start, stop, [step = 1]): start, start+step, …
// up to and including stop, in start's unit.
func rangeFn(args []value.Value) (value.Value, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("range() expects 2 or 3 arguments, got %d", len(args))
	}
	start, err := argNum("range", args, 0)
	if err != nil {
		return nil, err
	}
	stop, err := argNum("range", args, 1)
	if err != nil {
		return nil, err
	}
	step := 1.0
	if len(args) == 3 {
		n, err := argNum("range", args, 2)
		if err != nil {
			return nil, err
		}
		if n.Num <= 0 {
			return nil, fmt.Errorf("range() step must be positive, got %v", n.Num)
		}
		step = n.Num
	}
	var items []value.Value
	for x := start.Num; x <= stop.Num; x += step {
		if len(items) == maxRange {
			return nil, fmt.Errorf("range() would produce more than %d items", maxRange)
		}
		items = append(items, &value.Number{Num: x, Unit: start.Unit})
	}
	return &value.List{Items: items}, nil
}

// listSeparator returns ',' for a comma list and ' ' otherwise, as a quoted
// string (Stylus returns a String node).
func listSeparator(args []value.Value) (value.Value, error) {
	if err := wantArgs("list-separator", args, 1); err != nil {
		return nil, err
	}
	sep := " "
	if listComma(args[0]) {
		sep = ","
	}
	return &value.Str{Val: sep, Quote: '\''}, nil
}

// pairPart implements keys/values over a list of pairs:
// keys((a 1) (b 2)) is `a b`, values(...) is `1 2`. Stylus also accepts an
// object (hash) here; go-styl has no hash type.
func pairPart(fn string, i int) Func {
	return func(args []value.Value) (value.Value, error) {
		if err := wantArgs(fn, args, 1); err != nil {
			return nil, err
		}
		var out []value.Value
		for _, pair := range asItems(args[0]) {
			items := asItems(pair)
			if len(items) <= i {
				return nil, fmt.Errorf("%s() expects a list of pairs, got %s", fn, pair.CSS(true))
			}
			out = append(out, items[i])
		}
		return &value.List{Items: out}, nil
	}
}

// clone returns its argument. Stylus deep-copies so a later push() can't
// change the original; go-styl values are never mutated, so a copy is not
// needed.
func clone(args []value.Value) (value.Value, error) {
	if err := wantArgs("clone", args, 1); err != nil {
		return nil, err
	}
	return args[0], nil
}
