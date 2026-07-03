package styl_test

import (
	"strings"
	"testing"
	"testing/fstest"

	styl "github.com/rohanthewiz/go-styl"
)

// TestM13Globals covers Options.Globals: Go values seeded as root-scope
// variables before the stylesheet executes.
func TestM13Globals(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		globals map[string]any
		want    string
	}{
		{"color string", "a\n  color primary", map[string]any{"primary": "#0af"}, "a{color:#0af}"},
		{"unit string", "a\n  padding pad", map[string]any{"pad": "12px"}, "a{padding:12px}"},
		{"list string", "a\n  border b", map[string]any{"b": "1px solid red"}, "a{border:1px solid red}"},
		{"expression string", "a\n  color c", map[string]any{"c": "darken(#0af, 20%)"}, "a{color:#08c}"},
		{"int", "a\n  z-index z", map[string]any{"z": 42}, "a{z-index:42}"},
		{"float", "a\n  opacity o", map[string]any{"o": 0.5}, "a{opacity:.5}"},
		{"bool in condition", "if dark\n  a\n    color #fff", map[string]any{"dark": true}, "a{color:#fff}"},
		{"nil is null", "a\n  if x == null\n    ok 1", map[string]any{"x": nil}, "a{ok:1}"},
		{"global feeds arithmetic", "a\n  width base * 2", map[string]any{"base": "10px"}, "a{width:20px}"},
		{"conditional default yields", "primary ?= #333\na\n  color primary", map[string]any{"primary": "#0af"}, "a{color:#0af}"},
		{"plain assignment overrides", "primary = #333\na\n  color primary", map[string]any{"primary": "#0af"}, "a{color:#333}"},
		{"quoted string value", `a
  content s`, map[string]any{"s": `"hi"`}, `a{content:"hi"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := styl.Compile(c.src, styl.Options{Globals: c.globals})
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestM13GlobalErrors: a malformed or unsupported global fails the compile
// with a message naming the global, without a misleading file position.
func TestM13GlobalErrors(t *testing.T) {
	cases := []struct {
		name    string
		globals map[string]any
		wantSub string
	}{
		{"parse error", map[string]any{"x": "1px +"}, `global "x":`},
		{"eval error", map[string]any{"x": "1 + true"}, `global "x":`},
		{"unsupported type", map[string]any{"x": struct{}{}}, `global "x": unsupported Go type`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := styl.Compile("a\n  color red", styl.Options{Globals: c.globals})
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Errorf("error %q does not contain %q", err, c.wantSub)
			}
		})
	}
}

// TestM13CustomProperties covers Options.CustomProperties: listed root-level
// variables emit a :root block and direct references become var(--name),
// while compile-time computations use the concrete value.
func TestM13CustomProperties(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		props []string
		want  string
	}{
		{"root block and direct ref",
			"primary = #0af\na\n  color primary",
			[]string{"primary"},
			":root{--primary:#0af}a{color:var(--primary)}"},
		{"list order, undefined skipped",
			"b = 2px\na = 1px\nx\n  top a\n  bottom b",
			[]string{"b", "missing", "a"},
			":root{--b:2px;--a:1px}x{top:var(--a);bottom:var(--b)}"},
		{"arithmetic uses concrete value",
			"pad = 8px\na\n  margin pad\n  padding pad * 2",
			[]string{"pad"},
			":root{--pad:8px}a{margin:var(--pad);padding:16px}"},
		{"builtin uses concrete value",
			"c = #0af\na\n  color c\n  border-color darken(c, 20%)",
			[]string{"c"},
			":root{--c:#0af}a{color:var(--c);border-color:#08c}"},
		{"passthrough function keeps var",
			"x = 10px\na\n  transform translateX(x)",
			[]string{"x"},
			":root{--x:10px}a{transform:translateX(var(--x))}"},
		{"literal slash keeps var",
			"lh = 1.5\na\n  font 14px/lh sans-serif",
			[]string{"lh"},
			":root{--lh:1.5}a{font:14px/var(--lh) sans-serif}"},
		{"var inside value list",
			"c = #0af\na\n  border 1px solid c",
			[]string{"c"},
			":root{--c:#0af}a{border:1px solid var(--c)}"},
		{"interpolation uses concrete value",
			"g = 16px\na\n  width calc(100% - {g})",
			[]string{"g"},
			":root{--g:16px}a{width:calc(100% - 16px)}"},
		{"selector interpolation uses concrete value",
			"n = 2\n.col-{n}\n  x n",
			[]string{"n"},
			":root{--n:2}.col-2{x:var(--n)}"},
		{"media query uses concrete value",
			"bp = 600px\n@media (min-width: bp)\n  a\n    x 1",
			[]string{"bp"},
			":root{--bp:600px}@media (min-width:600px){a{x:1}}"},
		{"comparison uses concrete value",
			"c = #0af\nif c == #0af\n  a\n    ok 1",
			[]string{"c"},
			":root{--c:#0af}a{ok:1}"},
		{"truthiness uses concrete value",
			"flag = true\nif flag\n  a\n    ok flag",
			[]string{"flag"},
			":root{--flag:true}a{ok:var(--flag)}"},
		{"loop iterates concrete list",
			"sizes = 1px 2px\nfor s in sizes\n  .m-{s}\n    margin s",
			[]string{"sizes"},
			":root{--sizes:1px 2px}.m-1px{margin:1px}.m-2px{margin:2px}"},
		{"mixin body keeps var",
			"c = #0af\nbordered(x)\n  border 1px solid x\na\n  bordered(c)",
			[]string{"c"},
			":root{--c:#0af}a{border:1px solid var(--c)}"},
		{"function computes from concrete value",
			"pad = 8px\ndouble(n)\n  n * 2\na\n  width double(pad)",
			[]string{"pad"},
			":root{--pad:8px}a{width:16px}"},
		{"reassignment last value wins in root block",
			"c = #111\nc = #222\na\n  color c",
			[]string{"c"},
			":root{--c:#222}a{color:var(--c)}"},
		{"rule-local variable stays concrete",
			"a\n  pad = 4px\n  margin pad",
			[]string{"pad"},
			"a{margin:4px}"},
		{"unary uses concrete value",
			"g = 16px\na\n  margin -(g)",
			[]string{"g"},
			":root{--g:16px}a{margin:-16px}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := styl.Compile(c.src, styl.Options{CustomProperties: c.props})
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestM13GlobalsWithCustomProperties: the flagship theming combo — a Go-side
// theme value overrides the sheet default and is exposed for runtime
// re-theming.
func TestM13GlobalsWithCustomProperties(t *testing.T) {
	src := "primary ?= #333\na\n  color primary"
	got, err := styl.Compile(src, styl.Options{
		Globals:          map[string]any{"primary": "#0af"},
		CustomProperties: []string{"primary"},
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	want := ":root{--primary:#0af}a{color:var(--primary)}"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestM13GlobalsPretty: pretty output renders the :root block expanded.
func TestM13GlobalsPretty(t *testing.T) {
	got, err := styl.Compile("a\n  color c", styl.Options{
		Pretty:           true,
		Globals:          map[string]any{"c": "#0af"},
		CustomProperties: []string{"c"},
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	want := ":root {\n\t--c: #0af;\n}\n\na {\n\tcolor: var(--c);\n}"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestM13GlobalsInImports: globals live in the root scope, so imported
// sheets see them too.
func TestM13GlobalsInImports(t *testing.T) {
	fsys := fstest.MapFS{
		"main.styl": &fstest.MapFile{Data: []byte("@import \"part\"\n")},
		"part.styl": &fstest.MapFile{Data: []byte("a\n  color primary\n")},
	}
	got, err := styl.CompileFile("main.styl", styl.Options{
		FS:      fsys,
		Globals: map[string]any{"primary": "#0af"},
	})
	if err != nil {
		t.Fatalf("CompileFile: %v", err)
	}
	if got != "a{color:#0af}" {
		t.Errorf("got %q", got)
	}
}
