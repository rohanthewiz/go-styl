package styl

import (
	"strings"
	"testing"
	"testing/fstest"
)

// N-023: @require (import once per compile) and glob import paths.

var requireFS = fstest.MapFS{
	"s/_styl/_b.styl": &fstest.MapFile{Data: []byte(".b\n  order 2\n")},
	"s/_styl/_a.styl": &fstest.MapFile{Data: []byte(".a\n  order 1\n")},
	"s/_styl/x.css":   &fstest.MapFile{Data: []byte(".css{}")},
	"s/_vars.styl":    &fstest.MapFile{Data: []byte("v = 3px\n.v\n  order 3\n")},
	"s/_uses.styl":    &fstest.MapFile{Data: []byte("@require '_vars'\n")},
}

func compileRequire(t *testing.T, src string) string {
	t.Helper()
	got, err := Compile(src, Options{FS: requireFS, BaseDir: "s"})
	if err != nil {
		t.Fatalf("compile %q: %v", src, err)
	}
	return strings.TrimRight(got, "\n")
}

func TestRequire(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			"require twice imports once",
			"@require '_vars'\n@require '_vars'\n.x\n  width v",
			".v{order:3}.x{width:3px}",
		},
		{
			"nested require sees outer require",
			"@require '_vars'\n@import '_uses'",
			".v{order:3}",
		},
		{
			"import is not import-once",
			"@import '_vars'\n@import '_vars'",
			".v{order:3}.v{order:3}",
		},
		{
			"require after import still imports once more",
			"@import '_vars'\n@require '_vars'\n@require '_vars'",
			".v{order:3}.v{order:3}",
		},
		{
			"glob imports .styl matches in sorted order",
			"@import '_styl/*'",
			".a{order:1}.b{order:2}",
		},
		{
			"require glob skips already-required files",
			"@require '_styl/_b'\n@require '_styl/*'",
			".b{order:2}.a{order:1}",
		},
		{
			"literal require is deduped",
			"@require 'x.css'\n@require 'x.css'",
			`@import "x.css";`,
		},
		{
			"brace syntax with semicolon",
			"@require '_vars';\n.x { width: v; }",
			".v{order:3}.x{width:3px}",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := compileRequire(t, c.src); got != c.want {
				t.Errorf("got  %q\nwant %q", got, c.want)
			}
		})
	}
}

func TestRequireGlobNoMatch(t *testing.T) {
	_, err := Compile("@import 'nope/*'", Options{FS: requireFS, BaseDir: "s"})
	if err == nil || !strings.Contains(err.Error(), "file not found") {
		t.Fatalf("want file-not-found error, got %v", err)
	}
}

// Globbing on the OS filesystem, with deps recorded for each matched file.
func TestRequireGlobOS(t *testing.T) {
	res, err := BuildFile("testdata/require/main.styl", Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := ".a{width:1px}.b{width:2px}.main{width:1px}"
	if got := strings.TrimRight(res.CSS, "\n"); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if len(res.Deps) != 3 { // main.styl + _a + _b; the repeat require adds none
		t.Errorf("want 3 deps, got %v", res.Deps)
	}
}
