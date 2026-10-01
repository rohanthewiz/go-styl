package styl_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	styl "github.com/rohanthewiz/go-styl"
)

// mapSeg is one decoded source-map segment, all fields 0-based.
type mapSeg struct{ genLine, genCol, src, srcLine, srcCol int }

// decodeSegments decodes a Source Map v3 "mappings" string into absolute
// segments. Generated columns reset per line; source index/line/column deltas
// run across the whole string (the same rules encodeMappings writes by).
func decodeSegments(mappings string) []mapSeg {
	var out []mapSeg
	src, srcLine, srcCol := 0, 0, 0
	for gl, line := range strings.Split(mappings, ";") {
		if line == "" {
			continue
		}
		genCol := 0
		for _, seg := range strings.Split(line, ",") {
			d, i := vlqDecode(seg, 0)
			genCol += d
			d, i = vlqDecode(seg, i)
			src += d
			d, i = vlqDecode(seg, i)
			srcLine += d
			d, _ = vlqDecode(seg, i)
			srcCol += d
			out = append(out, mapSeg{gl, genCol, src, srcLine, srcCol})
		}
	}
	return out
}

// mapDoc is the part of a Source Map v3 document the tests read.
type mapDoc struct {
	Sources        []string  `json:"sources"`
	SourcesContent []*string `json:"sourcesContent"`
	Mappings       string    `json:"mappings"`
}

// segmentAt finds the segment starting exactly at the first occurrence of gen
// in css and returns the "sources" name it points into and the source text
// from its position to the end of that line, read from sourcesContent. ok is
// false when gen is absent or no segment starts there, i.e. that output text
// is not individually mapped.
func segmentAt(t *testing.T, css, mapJSON, gen string) (source, text string, ok bool) {
	t.Helper()
	var doc mapDoc
	if err := json.Unmarshal([]byte(mapJSON), &doc); err != nil {
		t.Fatalf("map JSON: %v", err)
	}
	if len(doc.SourcesContent) != len(doc.Sources) {
		t.Fatalf("sourcesContent has %d entries for %d sources", len(doc.SourcesContent), len(doc.Sources))
	}
	at := strings.Index(css, gen)
	if at < 0 {
		t.Fatalf("%q not in output:\n%s", gen, css)
	}
	// Source maps count columns in characters, so measure in runes.
	before := css[:at]
	genLine := strings.Count(before, "\n")
	genCol := len([]rune(before[strings.LastIndex(before, "\n")+1:]))

	for _, s := range decodeSegments(doc.Mappings) {
		if s.genLine != genLine || s.genCol != genCol {
			continue
		}
		if s.src >= len(doc.Sources) || doc.SourcesContent[s.src] == nil {
			t.Fatalf("segment at %q points at source %d with no content (sources %v)", gen, s.src, doc.Sources)
		}
		srcLines := strings.Split(*doc.SourcesContent[s.src], "\n")
		if s.srcLine >= len(srcLines) {
			t.Fatalf("segment at %q points past the end of %s (line %d)", gen, doc.Sources[s.src], s.srcLine+1)
		}
		line := []rune(srcLines[s.srcLine])
		if s.srcCol > len(line) {
			t.Fatalf("segment at %q points past the end of %s line %d", gen, doc.Sources[s.src], s.srcLine+1)
		}
		return doc.Sources[s.src], string(line[s.srcCol:]), true
	}
	return "", "", false
}

// mappedSource compiles src with a map and returns the source text the segment
// at gen points to (see segmentAt).
func mappedSource(t *testing.T, src string, opts styl.Options, gen string) (string, bool) {
	t.Helper()
	css, mapJSON, err := styl.CompileMap(src, opts)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	_, text, ok := segmentAt(t, css, mapJSON, gen)
	return text, ok
}

// TestSourceMapValues checks that a declaration's value gets its own segment,
// pointing at the value expression, and that property and value columns are
// the source's own in the layouts whose output indentation differs from the
// source's: tab indentation, brace syntax (re-indented before parsing), a
// one-line brace block (split over several internal lines), and `;`-separated
// statements.
func TestSourceMapValues(t *testing.T) {
	pretty := styl.Options{Pretty: true}
	compressed := styl.Options{}
	cases := []struct {
		name string
		src  string
		opts styl.Options
		gen  string // text in the output where a segment should start
		want string // source text that segment should point at (a prefix)
	}{
		{"value maps to its expression", "c = red\nbody\n  color c\n", pretty,
			"red", "c"},
		{"property still maps", "c = red\nbody\n  color c\n", pretty,
			"color", "color c"},
		{"computed value", "body\n  width 10px * 2\n", pretty,
			"20px", "10px * 2"},
		{"important shares the value segment", "a\n  margin: 0 auto !important\n", pretty,
			"0 auto", "0 auto !important"},
		{"tab indentation", "a\n\tb\n\t\twidth 1px\n", pretty,
			"1px", "1px"},
		{"tab indentation, property", "a\n\tb\n\t\twidth 1px\n", pretty,
			"width", "width 1px"},
		{"brace syntax", "body {\n    color: red;\n}\n", pretty,
			"red", "red;"},
		{"one-line brace block", "body {\n  .a { width: 1px; height: 2px }\n}\n", pretty,
			"2px", "2px }"},
		{"one-line brace block, property", "body {\n  .a { width: 1px; height: 2px }\n}\n", pretty,
			"height", "height: 2px }"},
		{"comment before a braced rule", "/* x */ .b { top: 0 }\n", pretty,
			".b", ".b { top: 0 }"},
		{"semicolons in indented syntax", "a\n  color: red; top 0\n", pretty,
			"0;", "0"},
		{"mixin body value", "m()\n  pad 1px\nbody\n  m()\n", pretty,
			"1px", "1px"},
		{"compressed", "a\n  x 1\nb\n  y 2\n", compressed,
			"2}", "2"},
		{"string value past non-ASCII", "a\n  font: 'Aé' 1px, b\n", compressed,
			"'Aé'", "'Aé' 1px, b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := mappedSource(t, tc.src, tc.opts, tc.gen)
			if !ok {
				t.Fatalf("no segment starts at %q", tc.gen)
			}
			if !strings.HasPrefix(got, tc.want) {
				t.Errorf("%q maps to %q, want text starting %q", tc.gen, got, tc.want)
			}
		})
	}
}

// TestSourceMapNoValueSegment pins that a value without a source expression
// (add-property()) gets no segment of its own: the property's covers it.
func TestSourceMapNoValueSegment(t *testing.T) {
	src := "a\n  foo add-property(bar, 1)\n"
	if _, ok := mappedSource(t, src, styl.Options{Pretty: true}, "bar"); !ok {
		t.Fatal("add-property()'s property should be mapped (to its rule)")
	}
	if got, ok := mappedSource(t, src, styl.Options{Pretty: true}, "1;"); ok {
		t.Errorf("add-property()'s value should not have its own segment, got one at %q", got)
	}
}

// TestBraceOneLinerErrorPosition pins that a statement in a one-line brace
// block reports its own source line and column, not the internal line the
// brace rewrite moved it to.
func TestBraceOneLinerErrorPosition(t *testing.T) {
	_, err := styl.Compile(".a {\n  .b { color: red; @extend }\n}\n", styl.Options{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), ":2:20:") {
		t.Errorf("error = %v, want position 2:20", err)
	}
}

// mapCase is one expectation on a multi-file map: the segment at gen points
// into source, at text starting with want.
type mapCase struct{ gen, source, want string }

func checkMap(t *testing.T, res styl.Result, wantSources []string, cases []mapCase) {
	t.Helper()
	var doc mapDoc
	if err := json.Unmarshal([]byte(res.Map), &doc); err != nil {
		t.Fatalf("map JSON: %v", err)
	}
	if strings.Join(doc.Sources, " ") != strings.Join(wantSources, " ") {
		t.Errorf("sources = %q, want %q", doc.Sources, wantSources)
	}
	for _, c := range cases {
		source, text, ok := segmentAt(t, res.CSS, res.Map, c.gen)
		if !ok {
			t.Errorf("no segment starts at %q", c.gen)
			continue
		}
		if source != c.source || !strings.HasPrefix(text, c.want) {
			t.Errorf("%q maps to %s %q, want %s %q", c.gen, source, text, c.source, c.want)
		}
	}
}

// TestSourceMapImports: a rule from an @import'ed file maps into that file,
// which gets its own "sources" entry and sourcesContent. Absolute entry path,
// so import names are absolute too.
func TestSourceMapImports(t *testing.T) {
	dir := filepath.ToSlash(t.TempDir())
	write := func(name, src string) {
		p := filepath.Join(filepath.FromSlash(dir), filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// _lib's rule sits on line 4, past the end of the 3-line entry: before
	// multi-file maps it was mapped onto the entry, past its end.
	write("_lib.styl", "\n\n\n.lib\n  color blue\n")
	write("partials/_btn.styl", "btn()\n  padding 2px\n")
	write("app.styl", "@import '_lib'\n@import 'partials/_btn'\nbody\n  top 0\n  btn()\n")

	res, err := styl.BuildFile(dir+"/app.styl", styl.Options{SourceMap: true, Pretty: true})
	if err != nil {
		t.Fatal(err)
	}
	checkMap(t, res, []string{dir + "/app.styl", dir + "/_lib.styl", dir + "/partials/_btn.styl"}, []mapCase{
		{".lib", dir + "/_lib.styl", ".lib"},
		{"blue", dir + "/_lib.styl", "blue"},
		{"body", dir + "/app.styl", "body"},
		{"top", dir + "/app.styl", "top 0"},
		// A mixin's declarations map to where the mixin is defined.
		{"padding", dir + "/partials/_btn.styl", "padding 2px"},
		{"2px", dir + "/partials/_btn.styl", "2px"},
	})
}

// TestSourceMapImportsRelative: a relative entry path gives import names
// relative to the same base. _vars.styl has no rules of its own; it is listed
// because the button() mixin defined there emits the `.btn` declarations.
func TestSourceMapImportsRelative(t *testing.T) {
	res, err := styl.BuildFile("testdata/imports/main.styl", styl.Options{SourceMap: true, Pretty: true})
	if err != nil {
		t.Fatal(err)
	}
	checkMap(t, res, []string{"testdata/imports/main.styl", "testdata/imports/_vars.styl"}, []mapCase{
		{".btn", "testdata/imports/main.styl", ".btn"},
		{"background", "testdata/imports/_vars.styl", "background c"},
		{"white", "testdata/imports/main.styl", "white"},
	})
}

// TestSourceMapImportsFS covers fs.FS paths: names are the fs paths, read
// relative to the entry's own directory, and sourcesContent comes from the FS.
// A partial with no output (variables only) is not listed, and a file
// imported twice is listed once.
func TestSourceMapImportsFS(t *testing.T) {
	fsys := fstest.MapFS{
		"styles/app.styl":         {Data: []byte("@import 'vars'\n@import 'partials/_a'\n@import 'partials/_a'\nbody\n  color c\n")},
		"styles/vars.styl":        {Data: []byte("c = red\n")},
		"styles/partials/_a.styl": {Data: []byte(".a\n  x 1\n")},
	}
	res, err := styl.BuildFile("styles/app.styl", styl.Options{FS: fsys, SourceMap: true, Pretty: true})
	if err != nil {
		t.Fatal(err)
	}
	checkMap(t, res, []string{"styles/app.styl", "styles/partials/_a.styl"}, []mapCase{
		{".a", "styles/partials/_a.styl", ".a"},
		{"red", "styles/app.styl", "c"},
	})

	// A bare-string compile sits in BaseDir, and its imports are named
	// relative to it, next to the default "input.styl".
	res, err = styl.Build("@import 'partials/_a'\n", styl.Options{FS: fsys, BaseDir: "styles", SourceMap: true})
	if err != nil {
		t.Fatal(err)
	}
	checkMap(t, res, []string{"input.styl", "partials/_a.styl"}, []mapCase{
		{".a", "partials/_a.styl", ".a"},
	})
}

// TestSourceMapMapFile: with Options.MapFile set, every "sources" name, the
// entry's included, is relative to the map's directory, which is where the v3
// spec resolves them from. Covers a map beside the entry, a map in a sibling
// output directory, relative and absolute OS paths, and fs paths.
func TestSourceMapMapFile(t *testing.T) {
	build := func(t *testing.T, entry string, opts styl.Options) styl.Result {
		t.Helper()
		opts.SourceMap, opts.Pretty = true, true
		res, err := styl.BuildFile(entry, opts)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	t.Run("relative entry, map in output dir", func(t *testing.T) {
		// The CLI case: styl -o out/main.css -sourcemap testdata/imports/main.styl.
		res := build(t, "testdata/imports/main.styl", styl.Options{MapFile: "out/main.css.map"})
		checkMap(t, res, []string{"../testdata/imports/main.styl", "../testdata/imports/_vars.styl"}, []mapCase{
			{".btn", "../testdata/imports/main.styl", ".btn"},
			{"background", "../testdata/imports/_vars.styl", "background c"},
		})
	})

	t.Run("map beside entry", func(t *testing.T) {
		res := build(t, "testdata/imports/main.styl", styl.Options{MapFile: "testdata/imports/main.css.map"})
		checkMap(t, res, []string{"main.styl", "_vars.styl"}, []mapCase{
			{"background", "_vars.styl", "background c"},
		})
	})

	t.Run("absolute paths", func(t *testing.T) {
		// Absolute entry and map give relative names, so the map stays
		// valid when the tree moves.
		dir := t.TempDir()
		for name, src := range map[string]string{
			"styles/app.styl":           "@import 'partials/_btn'\nbody\n  btn()\n",
			"styles/partials/_btn.styl": "btn()\n  padding 2px\n",
		} {
			p := filepath.Join(dir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		res := build(t, filepath.Join(dir, "styles", "app.styl"), styl.Options{MapFile: filepath.Join(dir, "dist", "app.css.map")})
		checkMap(t, res, []string{"../styles/app.styl", "../styles/partials/_btn.styl"}, []mapCase{
			{"body", "../styles/app.styl", "body"},
			{"padding", "../styles/partials/_btn.styl", "padding 2px"},
		})
	})

	fsys := fstest.MapFS{
		"styles/app.styl":         {Data: []byte("@import 'partials/_a'\nbody\n  x 1\n")},
		"styles/partials/_a.styl": {Data: []byte(".a\n  x 1\n")},
	}

	t.Run("fs paths", func(t *testing.T) {
		res := build(t, "styles/app.styl", styl.Options{FS: fsys, MapFile: "dist/app.css.map"})
		checkMap(t, res, []string{"../styles/app.styl", "../styles/partials/_a.styl"}, []mapCase{
			{".a", "../styles/partials/_a.styl", ".a"},
		})
		res = build(t, "styles/app.styl", styl.Options{FS: fsys, MapFile: "styles/app.css.map"})
		checkMap(t, res, []string{"app.styl", "partials/_a.styl"}, nil)
	})

	t.Run("bare string", func(t *testing.T) {
		// No Filename: the entry is "input.styl" in BaseDir, as without
		// MapFile, then named from the map like the rest.
		res, err := styl.Build("@import 'partials/_a'\n", styl.Options{FS: fsys, BaseDir: "styles", SourceMap: true, MapFile: "dist/x.css.map"})
		if err != nil {
			t.Fatal(err)
		}
		checkMap(t, res, []string{"../styles/input.styl", "../styles/partials/_a.styl"}, nil)
	})
}
