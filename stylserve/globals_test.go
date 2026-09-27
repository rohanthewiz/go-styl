package stylserve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// themeRoot writes a stylesheet with overridable defaults (`?=`), the shape
// per-request globals are meant for.
func themeRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := "brand ?= #333\npad ?= 4px\n.btn\n  color brand\n  padding pad\n"
	if err := os.WriteFile(filepath.Join(dir, "app.styl"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestAssetWithLayersGlobals(t *testing.T) {
	eng := New(Options{Dir: themeRoot(t), Globals: map[string]any{"pad": "8px"}})

	base, err := eng.Asset("app.css")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(base.Body); !strings.Contains(got, "color:#333") || !strings.Contains(got, "padding:8px") {
		t.Errorf("base = %q", got)
	}

	red, err := eng.AssetWith("app.css", map[string]any{"brand": "#f00"})
	if err != nil {
		t.Fatal(err)
	}
	// The per-call value wins; the engine-wide pad still applies.
	if got := string(red.Body); !strings.Contains(got, "color:red") && !strings.Contains(got, "color:#f00") || !strings.Contains(got, "padding:8px") {
		t.Errorf("variant = %q", got)
	}
	if red.ETag == base.ETag {
		t.Error("variant shares the base ETag")
	}

	// Same set again: served from cache (same *Asset).
	again, _ := eng.AssetWith("app.css", map[string]any{"brand": "#f00"})
	if again != red {
		t.Error("identical globals recompiled instead of hitting the cache")
	}
	// An empty set is the base build.
	if b, _ := eng.AssetWith("app.css", map[string]any{}); b != base {
		t.Error("empty globals did not return the base asset")
	}
}

func TestGlobalsKeyTypeAware(t *testing.T) {
	if globalsKey(map[string]any{"n": "10"}) == globalsKey(map[string]any{"n": 10}) {
		t.Error(`"10" and 10 share a key`)
	}
	a := globalsKey(map[string]any{"a": 1, "b": "x"})
	b := globalsKey(map[string]any{"b": "x", "a": 1})
	if a != b {
		t.Error("key depends on map order")
	}
}

func TestVariantsCapped(t *testing.T) {
	eng := New(Options{Dir: themeRoot(t), MaxVariants: 3})
	base, _ := eng.Asset("app.css")
	for i := 0; i < 10; i++ {
		if _, err := eng.AssetWith("app.css", map[string]any{"pad": i}); err != nil {
			t.Fatal(err)
		}
		if n := len(eng.variants); n > 3 {
			t.Fatalf("variants grew to %d, cap 3", n)
		}
	}
	// The base build survives variant eviction.
	if b, _ := eng.Asset("app.css"); b != base {
		t.Error("base asset evicted with the variants")
	}
}

func TestVariantInvalidatedBySourceChange(t *testing.T) {
	dir := themeRoot(t)
	eng := New(Options{Dir: dir})
	v1, err := eng.AssetWith("app.css", map[string]any{"brand": "#f00"})
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "app.styl")
	if err := os.WriteFile(p, []byte("brand ?= #333\n.btn\n  background brand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(p, later, later); err != nil {
		t.Fatal(err)
	}
	v2, err := eng.AssetWith("app.css", map[string]any{"brand": "#f00"})
	if err != nil {
		t.Fatal(err)
	}
	if v2 == v1 || !strings.Contains(string(v2.Body), "background:") {
		t.Errorf("variant not rebuilt after source change: %q", v2.Body)
	}
}
