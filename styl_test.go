package styl_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	styl "github.com/rohanthewiz/go-styl"
)

// TestGolden compiles every testdata/*.styl file and compares the result against
// the committed golden files: <name>.css (pretty) and, when present,
// <name>.min.css (compressed).
func TestGolden(t *testing.T) {
	matches, err := filepath.Glob("testdata/*.styl")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no testdata/*.styl fixtures found")
	}

	for _, src := range matches {
		name := strings.TrimSuffix(filepath.Base(src), ".styl")

		t.Run(name+"/pretty", func(t *testing.T) {
			got, err := styl.CompileFile(src, styl.Options{Pretty: true})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			assertGolden(t, "testdata/"+name+".css", got)
		})

		min := "testdata/" + name + ".min.css"
		if _, err := os.Stat(min); err == nil {
			t.Run(name+"/compressed", func(t *testing.T) {
				got, err := styl.CompileFile(src, styl.Options{Pretty: false})
				if err != nil {
					t.Fatalf("compile: %v", err)
				}
				assertGolden(t, min, got)
			})
		}
	}
}

func assertGolden(t *testing.T, path, got string) {
	t.Helper()
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	w := strings.TrimRight(string(want), "\n")
	g := strings.TrimRight(got, "\n")
	if g != w {
		t.Errorf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", path, g, w)
	}
}
