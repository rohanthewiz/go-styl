package builtin

import (
	"fmt"
	"path"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/value"
)

func init() {
	register("unquote(str)", unquote)
	register("quote(str)", quote)
	register("s(fmt, args...)", sprintf)
	register("uppercase(str)", caseFn("uppercase", strings.ToUpper))
	register("lowercase(str)", caseFn("lowercase", strings.ToLower))
	register("substr(str, start, length?)", substr)
	register("replace(pattern, replacement, str)", replace)
	register("split(delim, str)", split)
	register("basename(path, ext?)", basename)
	register("dirname(path)", dirname)
	register("extname(path)", extname)
	register("pathjoin(parts...)", pathjoin)
	register("convert(str)", convert)
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

// sprintf implements Stylus's s() (and the string `%` operator, which calls
// it): placeholders are filled from the remaining arguments in order. %s
// takes an argument's CSS form, strings keeping their quotes; %d takes a
// number's bare value, unit dropped (`%d` of 3px is 3). A placeholder with no
// argument left becomes empty (Stylus fills it with null). `%%` is not an
// escape in Stylus and stays as is.
func sprintf(args []value.Value) (value.Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("s() expects at least 1 argument")
	}
	format := strVal(args[0])
	rest := args[1:]
	var b strings.Builder
	ai := 0
	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) && (format[i+1] == 's' || format[i+1] == 'd') {
			if ai < len(rest) {
				arg := rest[ai]
				ai++
				if format[i+1] == 'd' {
					n, ok := arg.(*value.Number)
					if !ok {
						return nil, fmt.Errorf("%%d requires a unit, got %s", arg.TypeName())
					}
					arg = &value.Number{Num: n.Num}
				}
				b.WriteString(arg.CSS(true))
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

// The path functions work on slash-separated paths (URLs and asset paths
// in a stylesheet) and return single-quoted strings, as Stylus does.
func pathStr(s string) value.Value { return &value.Str{Val: s, Quote: '\''} }

// basename(path, [ext]) is the last path element, without ext when it ends
// the name: basename("a/b/c.png", ".png") is 'c'.
func basename(args []value.Value) (value.Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("basename() expects 1 or 2 arguments, got %d", len(args))
	}
	base := path.Base(strVal(args[0]))
	if len(args) == 2 {
		base = strings.TrimSuffix(base, strVal(args[1]))
	}
	return pathStr(base), nil
}

// dirname(path) is everything before the last element: 'a/b'.
func dirname(args []value.Value) (value.Value, error) {
	if err := wantArgs("dirname", args, 1); err != nil {
		return nil, err
	}
	return pathStr(path.Dir(strVal(args[0]))), nil
}

// extname(path) is the extension, dot included: '.png'.
func extname(args []value.Value) (value.Value, error) {
	if err := wantArgs("extname", args, 1); err != nil {
		return nil, err
	}
	return pathStr(path.Ext(strVal(args[0]))), nil
}

// pathjoin(parts...) joins and cleans path parts: 'a/b/c.png'.
func pathjoin(args []value.Value) (value.Value, error) {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = strVal(a)
	}
	return pathStr(path.Join(parts...)), nil
}

// convert(str) turns a string into the value it spells: convert("10px") is
// the number 10px, convert("#fff") a color, anything else an identifier.
// (Stylus parses the string as an expression; go-styl handles the single
// values that are its use in practice.)
func convert(args []value.Value) (value.Value, error) {
	if err := wantArgs("convert", args, 1); err != nil {
		return nil, err
	}
	s := strings.TrimSpace(strVal(args[0]))
	if n, err := value.ParseNumber(s); err == nil {
		return n, nil
	}
	if strings.HasPrefix(s, "#") {
		if c, err := value.ParseColor(s); err == nil {
			return c, nil
		}
	}
	return &value.Ident{Name: s}, nil
}
