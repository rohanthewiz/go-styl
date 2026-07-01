package styl

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzCompile asserts the compiler never panics on arbitrary input: any source
// must either compile or return an error. Run with:
//
//	go test -fuzz=FuzzCompile -fuzztime=60s
func FuzzCompile(f *testing.F) {
	// Seed with every example and testdata stylesheet plus targeted snippets
	// covering both syntaxes and the trickier constructs.
	for _, dir := range []string{"examples", filepath.Join("examples", "imports"), "testdata"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) != ".styl" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err == nil {
				f.Add(string(data))
			}
		}
	}
	for _, s := range []string{
		"a\n  b: c\n",
		"a { b: c }",
		"x = {y}\n",
		"a\n  width calc(100% - {g})\n",
		"@media (min-width: bp)\n  a\n    b c\n",
		"if x\n  y z\nelse\n  p q\n",
		"for i in 1 2 3\n  w i\n",
		"f(a, b = 1, rest...)\n  return a + b\n",
		"a\n  margin 10px -5px\n",
		"$ph\n  c d\n.e\n  @extend $ph\n",
		"a\n  b \"unterminated\n",
		"a\n  b: 1 +\n",
		"{{{}}}",
		"a{b:c;;}}}",
		"\t \ta\n\t\t\tb c\n",
	} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, src string) {
		// Both output modes and the merge pass; errors are fine, panics are not.
		_, _ = Compile(src, Options{Pretty: true})
		_, _ = Compile(src, Options{MergeDuplicates: true})
		_, _, _ = CompileMap(src, Options{Filename: "fuzz.styl"})
	})
}
