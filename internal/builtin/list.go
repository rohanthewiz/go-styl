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
	register("merge", merge)
	register("extend", merge)
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
	// An object counts its keys.
	if h, ok := args[0].(*value.Hash); ok {
		return &value.Number{Num: float64(h.Len())}, nil
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
	// Items join by their text: a string contributes its contents without
	// quotes, as in Stylus's join (which builds the result by string
	// concatenation), so join('|', 'a' 'b') is a|b.
	parts := make([]string, 0)
	for _, it := range asItems(args[1]) {
		if str, isStr := it.(*value.Str); isStr {
			parts = append(parts, str.Val)
			continue
		}
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

// pairPart implements keys/values over an object or a list of pairs:
// keys((a 1) (b 2)) is `a b`, values(...) is `1 2`. An object's keys come
// back as quoted strings ('a' 'b'), as in Stylus, in insertion order.
func pairPart(fn string, i int) Func {
	return func(args []value.Value) (value.Value, error) {
		if err := wantArgs(fn, args, 1); err != nil {
			return nil, err
		}
		var out []value.Value
		if h, ok := args[0].(*value.Hash); ok {
			for _, k := range h.Keys() {
				if i == 0 {
					out = append(out, &value.Str{Val: k, Quote: '\''})
				} else {
					v, _ := h.Get(k)
					out = append(out, v)
				}
			}
			return &value.List{Items: out}, nil
		}
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

// clone returns a copy of its argument. Objects are the only mutable values
// (see value.Hash), so they are deep-copied; anything else is returned as-is,
// since go-styl never mutates it in place (push() on a variable rebinds the
// variable to a new list).
func clone(args []value.Value) (value.Value, error) {
	if err := wantArgs("clone", args, 1); err != nil {
		return nil, err
	}
	if h, ok := args[0].(*value.Hash); ok {
		return h.Clone(), nil
	}
	return args[0], nil
}

// merge(dest, src…, [deep]) (alias extend) copies each src object's keys into
// dest, later sources winning, and returns dest. dest is changed in place, so
// every variable holding it sees the new keys, as in Stylus. A trailing true
// merges deeply: where both sides hold an object under a key, the source
// object is merged into dest's instead of replacing it.
//
//	merge({a: 1, b: {x: 1}}, {b: {y: 2}})        → {a: 1, b: {y: 2}}
//	merge({a: 1, b: {x: 1}}, {b: {y: 2}}, true)  → {a: 1, b: {x: 1, y: 2}}
func merge(args []value.Value) (value.Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("merge() expects at least 1 argument")
	}
	dest, ok := args[0].(*value.Hash)
	if !ok {
		return nil, fmt.Errorf("merge() argument 1 must be an object, got %s", args[0].TypeName())
	}
	srcs := args[1:]
	deep := false
	if n := len(srcs); n > 0 {
		if b, isBool := srcs[n-1].(*value.Bool); isBool {
			deep = b.Val
			srcs = srcs[:n-1]
		}
	}
	for i, a := range srcs {
		src, ok := a.(*value.Hash)
		if !ok {
			return nil, fmt.Errorf("merge() argument %d must be an object, got %s", i+2, a.TypeName())
		}
		mergeInto(dest, src, deep)
	}
	return dest, nil
}

// mergeInto copies src's keys into dest (see merge).
func mergeInto(dest, src *value.Hash, deep bool) {
	for _, k := range src.Keys() {
		sv, _ := src.Get(k)
		if deep {
			dv, _ := dest.Get(k)
			dh, dok := dv.(*value.Hash)
			sh, sok := sv.(*value.Hash)
			if dok && sok {
				mergeInto(dh, sh, true)
				continue
			}
		}
		dest.Set(k, sv)
	}
}
