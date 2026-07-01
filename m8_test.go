package styl

import (
	"strings"
	"testing"
	"testing/fstest"
)

// M8: Options.FS — compile and resolve @import through an fs.FS (embed.FS etc).

var m8FS = fstest.MapFS{
	"styles/app.styl": &fstest.MapFile{Data: []byte(
		"@import \"theme\"\n\nbody\n  color fg\n  width gutter * 2\n")},
	"styles/theme.styl": &fstest.MapFile{Data: []byte(
		"fg = #333\ngutter = 8px\n")},
	"styles/deep/nested.styl": &fstest.MapFile{Data: []byte(
		"@import \"/shared/base\"\n.n\n  padding pad\n")},
	"shared/base.styl": &fstest.MapFile{Data: []byte(
		"pad = 4px\n")},
	"styles/broken.styl": &fstest.MapFile{Data: []byte(
		".x\n  nope()\n")},
}

func TestM8CompileFileFS(t *testing.T) {
	css, err := CompileFile("styles/app.styl", Options{FS: m8FS, Pretty: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"color: #333", "width: 16px"} {
		if !strings.Contains(css, want) {
			t.Errorf("output missing %q:\n%s", want, css)
		}
	}
}

// A leading '/' import resolves from the FS root.
func TestM8RootRelativeImport(t *testing.T) {
	css, err := CompileFile("styles/deep/nested.styl", Options{FS: m8FS, Pretty: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(css, "padding: 4px") {
		t.Errorf("output missing root-relative import result:\n%s", css)
	}
}

// Compile (source given directly) also resolves imports through the FS, using
// BaseDir as the fs base.
func TestM8CompileWithFSBaseDir(t *testing.T) {
	css, err := Compile("@import \"theme\"\n.a\n  color fg\n",
		Options{FS: m8FS, BaseDir: "styles", Pretty: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(css, "color: #333") {
		t.Errorf("output missing imported variable:\n%s", css)
	}
}

// IncludePaths work as fs paths too.
func TestM8FSIncludePaths(t *testing.T) {
	css, err := Compile("@import \"base\"\n.a\n  padding pad\n",
		Options{FS: m8FS, IncludePaths: []string{"shared"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(css, "padding:4px") {
		t.Errorf("output missing include-path import result:\n%s", css)
	}
}

// Errors in FS mode still carry positions and the fs path.
func TestM8FSErrorPosition(t *testing.T) {
	_, err := CompileFile("styles/broken.styl", Options{FS: m8FS})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.HasPrefix(err.Error(), "styles/broken.styl:2:3: ") {
		t.Errorf("error = %q, want styles/broken.styl:2:3 prefix", err.Error())
	}
}

func TestM8FSMissingImport(t *testing.T) {
	_, err := Compile("@import \"missing\"\n", Options{FS: m8FS, BaseDir: "styles"})
	if err == nil || !strings.Contains(err.Error(), `"missing"`) {
		t.Errorf("error = %v, want mention of missing import", err)
	}
}
