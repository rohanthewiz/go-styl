package builtin

import (
	"fmt"
	"strings"

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
