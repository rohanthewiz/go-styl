package stylcrit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	styl "github.com/rohanthewiz/go-styl"
	"github.com/rohanthewiz/go-styl/stylcrit"
)

var testFS = fstest.MapFS{
	"app.styl": &fstest.MapFile{Data: []byte(`@import "theme"

.card
  background primary

.unused
  color red

.menu--open
  display block
`)},
	"theme.styl": &fstest.MapFile{Data: []byte("primary = #0af\n")},
}

const page = `<html><head><title>x</title></head><body><div class="card">hi</div></body></html>`

func TestInlineInjectsPrunedCSS(t *testing.T) {
	e := stylcrit.New(stylcrit.Options{Path: "app.styl", FS: testFS})
	out, err := e.Inline(page)
	if err != nil {
		t.Fatalf("Inline: %v", err)
	}
	if !strings.Contains(out, "<style>.card{background:#0af}</style></head>") {
		t.Errorf("style block not injected before </head>:\n%s", out)
	}
	if strings.Contains(out, ".unused") || strings.Contains(out, ".menu--open") {
		t.Errorf("unused rules leaked into:\n%s", out)
	}
}

func TestSafelistKeepsJSToggledRules(t *testing.T) {
	e := stylcrit.New(stylcrit.Options{
		Path: "app.styl", FS: testFS,
		Safelist: styl.Used{Classes: []string{"menu--open"}},
	})
	css, err := e.CSS(page)
	if err != nil {
		t.Fatalf("CSS: %v", err)
	}
	if !strings.Contains(css, ".menu--open") {
		t.Errorf("safelisted rule pruned:\n%s", css)
	}
	if strings.Contains(css, ".unused") {
		t.Errorf("unrelated rule kept:\n%s", css)
	}
}

func TestInlineFallbacks(t *testing.T) {
	e := stylcrit.New(stylcrit.Options{Path: "app.styl", FS: testFS})

	noHead := `<body><div class="card">x</div></body>`
	out, err := e.Inline(noHead)
	if err != nil {
		t.Fatalf("Inline: %v", err)
	}
	if !strings.Contains(out, "</style></body>") {
		t.Errorf("no </head>: want injection before </body>, got:\n%s", out)
	}

	fragment := `<div class="card">x</div>`
	out, err = e.Inline(fragment)
	if err != nil {
		t.Fatalf("Inline: %v", err)
	}
	if !strings.HasPrefix(out, "<style>") {
		t.Errorf("fragment: want prepended style, got:\n%s", out)
	}

	// A page using nothing from the sheet comes back untouched.
	empty := `<html><head></head><body><p>plain</p></body></html>`
	out, err = e.Inline(empty)
	if err != nil {
		t.Fatalf("Inline: %v", err)
	}
	if out != empty {
		t.Errorf("page needing no CSS was modified:\n%s", out)
	}
}

func TestCacheHitsAndInvalidation(t *testing.T) {
	dir := t.TempDir()
	sheet := filepath.Join(dir, "app.styl")
	write := func(src string) {
		t.Helper()
		if err := os.WriteFile(sheet, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".card\n  color red\n")

	e := stylcrit.New(stylcrit.Options{Path: sheet})
	first, err := e.CSS(page)
	if err != nil {
		t.Fatalf("CSS: %v", err)
	}
	if !strings.Contains(first, "color:red") {
		t.Fatalf("unexpected css: %q", first)
	}
	// Same page again: cached (same content back).
	again, err := e.CSS(page)
	if err != nil || again != first {
		t.Fatalf("cache miss or drift: %q vs %q (err %v)", again, first, err)
	}

	// Change the sheet (and its size, so the stamp differs even on
	// filesystems with coarse mtimes): the engine must recompile.
	write(".card\n  color blue !important\n")
	fresh, err := e.CSS(page)
	if err != nil {
		t.Fatalf("CSS after change: %v", err)
	}
	if !strings.Contains(fresh, "blue") {
		t.Errorf("stale css served after source change: %q", fresh)
	}
}

func TestCompileErrorSurfaces(t *testing.T) {
	bad := fstest.MapFS{"app.styl": &fstest.MapFile{Data: []byte(".x\n  nope()\n")}}
	e := stylcrit.New(stylcrit.Options{Path: "app.styl", FS: bad})
	if _, err := e.CSS(page); err == nil {
		t.Error("want compile error, got nil")
	} else if !strings.Contains(err.Error(), "app.styl") {
		t.Errorf("error not positioned: %v", err)
	}
}
