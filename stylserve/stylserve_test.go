package stylserve

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func osRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("app.styl", "@import \"theme\"\nbody\n  color fg\n")
	write("theme.styl", "fg = #333\n")
	write("sub/page.styl", ".p\n  margin 0\n")
	write("broken.styl", ".x\n  nope()\n")
	return dir
}

func TestEngineCompiles(t *testing.T) {
	eng := New(Options{Dir: osRoot(t)})

	a, err := eng.Asset("app.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(a.Body), "color:#333") {
		t.Errorf("body = %q, want imported color", a.Body)
	}
	if a.ContentType != "text/css; charset=utf-8" || a.ETag == "" {
		t.Errorf("asset meta = %q %q", a.ContentType, a.ETag)
	}

	if _, err := eng.Asset("sub/page.css"); err != nil {
		t.Errorf("nested path: %v", err)
	}
}

func TestEngineCaches(t *testing.T) {
	eng := New(Options{Dir: osRoot(t)})
	a1, err := eng.Asset("app.css")
	if err != nil {
		t.Fatal(err)
	}
	a2, err := eng.Asset("app.css")
	if err != nil {
		t.Fatal(err)
	}
	if a1 != a2 {
		t.Error("expected the same cached asset")
	}
}

// Touching an imported dependency invalidates the compiled entry.
func TestEngineInvalidatesOnImportChange(t *testing.T) {
	dir := osRoot(t)
	eng := New(Options{Dir: dir})

	a1, err := eng.Asset("app.css")
	if err != nil {
		t.Fatal(err)
	}
	theme := filepath.Join(dir, "theme.styl")
	if err := os.WriteFile(theme, []byte("fg = #b00\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Ensure the mtime moves even on coarse-grained filesystems.
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(theme, future, future); err != nil {
		t.Fatal(err)
	}

	a2, err := eng.Asset("app.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(a2.Body), "color:#b00") {
		t.Errorf("body = %q, want recompiled color #b00", a2.Body)
	}
	if a1.ETag == a2.ETag {
		t.Error("ETag should change when output changes")
	}
}

func TestEngineNotFoundAndTraversal(t *testing.T) {
	eng := New(Options{Dir: osRoot(t)})
	for _, p := range []string{
		"missing.css",   // no source
		"app.styl",      // only .css requests are served
		"app.txt",       // wrong extension
		"../etc/pw.css", // traversal
		"app.css.map",   // maps disabled
	} {
		if _, err := eng.Asset(p); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Asset(%q) err = %v, want fs.ErrNotExist", p, err)
		}
	}
}

func TestEngineCompileErrorPositioned(t *testing.T) {
	eng := New(Options{Dir: osRoot(t)})
	_, err := eng.Asset("broken.css")
	if err == nil || !strings.Contains(err.Error(), "broken.styl:2:3") {
		t.Errorf("err = %v, want positioned compile error", err)
	}
}

func TestEngineFSMode(t *testing.T) {
	fsys := fstest.MapFS{
		"app.styl":   &fstest.MapFile{Data: []byte("@import \"theme\"\nbody\n  color fg\n")},
		"theme.styl": &fstest.MapFile{Data: []byte("fg = #444\n")},
	}
	eng := New(Options{FS: fsys, Pretty: true})
	a, err := eng.Asset("app.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(a.Body), "color: #444") {
		t.Errorf("body = %q", a.Body)
	}
}

func TestEngineSourceMaps(t *testing.T) {
	eng := New(Options{Dir: osRoot(t), SourceMaps: true})

	css, err := eng.Asset("app.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css.Body), "/*# sourceMappingURL=app.css.map */") {
		t.Errorf("css missing sourceMappingURL comment:\n%s", css.Body)
	}

	m, err := eng.Asset("app.css.map")
	if err != nil {
		t.Fatal(err)
	}
	if m.ContentType != "application/json; charset=utf-8" ||
		!strings.Contains(string(m.Body), `"version":3`) {
		t.Errorf("map = %q %q", m.ContentType, m.Body)
	}
}
