package parser

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFormatCorpus formats every .styl file in the repo's fixtures and checks
// that the full re-indent (not just the whitespace fallback) keeps the AST,
// and that formatting is idempotent.
func TestFormatCorpus(t *testing.T) {
	var files []string
	for _, pat := range []string{"../../testdata/*.styl", "../../testdata/*/*.styl", "../../examples/*/*.styl", "../../examples/*.styl", "../../difftest/corpus/*.styl", "../../difftest/corpus/*/*.styl"} {
		m, _ := filepath.Glob(pat)
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Fatal("no fixtures found")
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(data)
		orig, err := Parse(src)
		if err != nil {
			continue // fixtures of deliberate errors
		}
		re := reindent(tidy(src))
		sheet, err := Parse(re)
		if err != nil || !sameAST(orig, sheet) {
			t.Errorf("%s: reindent changed the AST (err=%v)", f, err)
			continue
		}
		out, err := Format(src)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		again, err := Format(out)
		if err != nil || again != out {
			t.Errorf("%s: not idempotent (err=%v)", f, err)
		}
	}
}
