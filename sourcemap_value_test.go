package styl_test

import (
	"encoding/json"
	"strings"
	"testing"

	styl "github.com/rohanthewiz/go-styl"
)

// mapSeg is one decoded source-map segment, all fields 0-based.
type mapSeg struct{ genLine, genCol, srcLine, srcCol int }

// decodeSegments decodes a Source Map v3 "mappings" string into absolute
// segments. Generated columns reset per line; source line/column deltas run
// across the whole string (the same rules encodeMappings writes by).
func decodeSegments(mappings string) []mapSeg {
	var out []mapSeg
	srcLine, srcCol := 0, 0
	for gl, line := range strings.Split(mappings, ";") {
		if line == "" {
			continue
		}
		genCol := 0
		for _, seg := range strings.Split(line, ",") {
			d, i := vlqDecode(seg, 0)
			genCol += d
			_, i = vlqDecode(seg, i) // source index: always 0
			d, i = vlqDecode(seg, i)
			srcLine += d
			d, _ = vlqDecode(seg, i)
			srcCol += d
			out = append(out, mapSeg{gl, genCol, srcLine, srcCol})
		}
	}
	return out
}

// mappedSource compiles src with a map and returns the source text (to the end
// of its line) that the segment starting exactly at the first occurrence of
// gen in the output points to. ok is false when gen is absent or no segment
// starts there, i.e. that output text is not individually mapped.
func mappedSource(t *testing.T, src string, opts styl.Options, gen string) (string, bool) {
	t.Helper()
	css, mapJSON, err := styl.CompileMap(src, opts)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var doc struct{ Mappings string }
	if err := json.Unmarshal([]byte(mapJSON), &doc); err != nil {
		t.Fatalf("map JSON: %v", err)
	}
	at := strings.Index(css, gen)
	if at < 0 {
		t.Fatalf("%q not in output:\n%s", gen, css)
	}
	// Source maps count columns in characters, so measure in runes.
	before := css[:at]
	genLine := strings.Count(before, "\n")
	genCol := len([]rune(before[strings.LastIndex(before, "\n")+1:]))

	srcLines := strings.Split(src, "\n")
	for _, s := range decodeSegments(doc.Mappings) {
		if s.genLine == genLine && s.genCol == genCol {
			if s.srcLine >= len(srcLines) {
				t.Fatalf("segment at %q points past the source (line %d)", gen, s.srcLine+1)
			}
			line := []rune(srcLines[s.srcLine])
			if s.srcCol > len(line) {
				t.Fatalf("segment at %q points past the end of source line %d", gen, s.srcLine+1)
			}
			return string(line[s.srcCol:]), true
		}
	}
	return "", false
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
