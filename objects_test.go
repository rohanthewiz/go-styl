package styl_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rohanthewiz/go-styl"
)

// TestObjects covers Stylus objects (hashes): literals, member and subscript
// reads, `in`, and the object-aware built-ins. Expected values are reference
// stylus 0.64's output for the same sheet, except where a comment says
// otherwise.
func TestObjects(t *testing.T) {
	const vars = "obj = {\n  foo: bar,\n  'baz-q': 1px 2px\n  n: 3\n}\n" +
		"inl = { a: 1, b: #f00 }\nempty = {}\nk = n\n"
	cases := []struct {
		name string
		expr string
		want string
	}{
		{"member", `obj.foo`, "bar"},
		{"quoted key subscript", `obj['baz-q']`, "1px 2px"},
		{"missing key is null", `obj.nope`, ""},
		{"variable subscript", `obj[k]`, "3"},
		{"length", `length(obj)`, "3"},
		{"keys", `keys(obj)`, "'foo' 'baz-q' 'n'"},
		{"values", `values(obj)`, "bar 1px 2px 3"},
		{"single-line literal", `inl.b`, "#f00"},
		{"empty", `length(empty)`, "0"},
		{"in", `'foo' in obj`, "true"},
		{"not in", `'zz' in obj`, "false"},
		{"in list", `(2 in (1 2 3)) (5 in (1 2 3))`, "true false"},
		{"inline literal member", `({a: 1, b: 2}).b`, "2"},
		{"merge", `keys(merge({a: 1, b: 2}, {b: 3, c: 4}))`, "'a' 'b' 'c'"},
		{"extend alias", `keys(extend({a: 1}, {b: 2}))`, "'a' 'b'"},
		{"clone", `length(clone(obj))`, "3"},
		{"contrast", `contrast(#000, #fff).ratio`, "21"},
		{"contrast translucent bottom", `contrast(#777, rgba(#fff, .5)).ratio`, "2.8"},
		{"contrast translucent top", `contrast(rgba(#000, .5), #fff).ratio`, "3.9"},
		{"contrast filter", `contrast(1.5)`, "contrast(1.5)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := compileMin(t, vars+".a\n  k "+c.expr+"\n")
			if want := ".a{k:" + c.want + "}"; got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// TestObjectAssignment: member and subscript assignment change the object
// in place, so every name holding it (b2 = obj) sees the change, as in
// Stylus. `for k, v in obj` walks keys in insertion order.
func TestObjectAssignment(t *testing.T) {
	src := `obj = { foo: bar, n: 3 }
obj.foo = qux
obj.new = 9
obj['k2'] = 8
obj.n ?= 100
nested = { a: { b: 5 } }
nested.a.b = 6
b2 = obj
b2.alias = yes
.a
  w obj.foo obj.new obj.k2 obj.n
  x nested['a']['b'] obj.alias
  for k, v in obj
    y-{k} v
`
	want := ".a{w:qux 9 8 3;x:6 yes;y-foo:qux;y-n:3;y-new:9;y-k2:8;y-alias:yes}"
	if got := compileMin(t, src); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// TestObjectDeepMerge: merge(…, true) merges nested objects instead of
// replacing them, and merge changes its first argument.
func TestObjectDeepMerge(t *testing.T) {
	src := `a = {x: 1, y: {p: 1, q: 2}}
merge(a, {y: {q: 5}})
d = {y: {p: 1, q: 2}}
merge(d, {y: {q: 5}}, true)
.m
  w a.y.q length(a.y)
  x d.y.p d.y.q
`
	if got, want := compileMin(t, src), ".m{w:5 1;x:1 5}"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestObjectBraceSyntax: object literals in a brace-syntax file are values,
// not blocks, including a multi-line literal with a trailing comment and
// one returned from a function.
func TestObjectBraceSyntax(t *testing.T) {
	src := `theme = {
  bg: #fff, // page background
  sizes: { sm: 10px, lg: 20px }
}
.a {
  padding: theme.sizes.lg;
  theme.bg = #000;
}
.b { color: theme.bg; }
f(o) {
  return {k: o.sm + 1}
}
.c { w: f(theme.sizes).k; }
`
	want := ".a{padding:20px}.b{color:#000}.c{w:11px}"
	if got := compileMin(t, src); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// TestObjectLineNumbers: folding a multi-line literal onto one line keeps
// later lines' error positions.
func TestObjectLineNumbers(t *testing.T) {
	src := "o = {\n  a: 1,\n  b: 2\n}\n.a\n  w 1px +\n"
	_, err := styl.Compile(src, styl.Options{})
	if err == nil || !strings.Contains(err.Error(), ":6:") {
		t.Fatalf("want an error on line 6, got %v", err)
	}
}

// TestObjectErrors: an object as a property value, and member access on
// something that isn't an object, are errors (Stylus prints a JSON-like
// blob, and has no `a.b` outside objects).
func TestObjectErrors(t *testing.T) {
	for _, src := range []string{
		"o = {a: 1}\n.a\n  w o\n",
		"x = 1px\n.a\n  w x.b\n",
		"x = 1 2\nx.b = 1\n",
	} {
		if _, err := styl.Compile(src, styl.Options{}); err == nil {
			t.Errorf("%q: want an error", src)
		}
	}
}

// TestInKeywordInCSS: CSS color-interpolation syntax uses the word `in`; in
// a property value it stays text when its right side is a bare word.
func TestInKeywordInCSS(t *testing.T) {
	src := ".a\n  background linear-gradient(to right in oklch, #f00, #00f)\n  color color-mix(in srgb, #f00 40%, #00f)\n"
	want := ".a{background:linear-gradient(to right in oklch,#f00,#00f);color:color-mix(in srgb,#f00 40%,#00f)}"
	if got := compileMin(t, src); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// TestJSON covers json() in both modes, reading through Options.FS: an
// object (with nested keys, parsed strings, leave-strings and optional) and
// one variable per leaf with an optional name prefix. The file is a build
// dependency.
func TestJSON(t *testing.T) {
	fsys := fstest.MapFS{
		"data.json": {Data: []byte(`{"color": "#f00", "size": "10px", "name": "hello world", "n": 3, "flag": true, "nested": {"a": "1em", "b": {"c": 2}}}`)},
		"main.styl": {Data: []byte(`json('data.json')
.j
  w color size name n flag
  x nested-a nested-b-c
json('data.json', false, 'pre-')
.k
  w pre-color
cfg = json('data.json', { hash: true })
.l
  w cfg.color cfg.nested.b.c cfg.name
cfg2 = json('data.json', { hash: true, leave-strings: true })
.l2
  w cfg2.color
opt = json('missing.json', { hash: true, optional: true })
.l3
  w type(opt)
`)},
	}
	res, err := styl.BuildFile("main.styl", styl.Options{FS: fsys})
	if err != nil {
		t.Fatalf("BuildFile: %v", err)
	}
	want := ".j{w:#f00 10px hello world 3 true;x:1em 2}.k{w:#f00}.l{w:#f00 2 hello world}.l2{w:'#f00'}.l3{w:null}"
	if got := strings.TrimRight(res.CSS, ""); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
	found := false
	for _, d := range res.Deps {
		if strings.HasSuffix(d, "data.json") {
			found = true
		}
	}
	if !found {
		t.Errorf("Deps %v should include data.json", res.Deps)
	}
}
