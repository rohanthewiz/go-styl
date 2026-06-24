package styl_test

import (
	"encoding/json"
	"strings"
	"testing"

	styl "github.com/rohanthewiz/go-styl"
)

// vlqDecode decodes the first base64-VLQ value of s starting at i.
func vlqDecode(s string, i int) (val, next int) {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	shift, result := 0, 0
	for {
		d := strings.IndexByte(chars, s[i])
		i++
		result |= (d & 0x1f) << shift
		if d&0x20 == 0 {
			break
		}
		shift += 5
	}
	v := result >> 1
	if result&1 == 1 {
		v = -v
	}
	return v, i
}

func TestSourceMapDocument(t *testing.T) {
	src := "body\n  color red\n  a\n    width 10px\n"
	css, mapJSON, err := styl.CompileMap(src, styl.Options{Pretty: true, Filename: "app.styl", OutFile: "app.css"})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !strings.HasPrefix(css, "body {") {
		t.Fatalf("unexpected css: %q", css)
	}

	var doc struct {
		Version        int      `json:"version"`
		File           string   `json:"file"`
		Sources        []string `json:"sources"`
		SourcesContent []string `json:"sourcesContent"`
		Names          []string `json:"names"`
		Mappings       string   `json:"mappings"`
	}
	if err := json.Unmarshal([]byte(mapJSON), &doc); err != nil {
		t.Fatalf("map is not valid JSON: %v\n%s", err, mapJSON)
	}

	if doc.Version != 3 {
		t.Errorf("version = %d, want 3", doc.Version)
	}
	if doc.File != "app.css" {
		t.Errorf("file = %q, want app.css", doc.File)
	}
	if len(doc.Sources) != 1 || doc.Sources[0] != "app.styl" {
		t.Errorf("sources = %v, want [app.styl]", doc.Sources)
	}
	if len(doc.SourcesContent) != 1 || doc.SourcesContent[0] != src {
		t.Errorf("sourcesContent not embedded verbatim")
	}
	if doc.Mappings == "" {
		t.Fatal("mappings is empty")
	}

	// First segment: generated (line 0, col 0) -> source (line 0, col 0), i.e. the
	// `body` selector maps to the first source line.
	genCol, i := vlqDecode(doc.Mappings, 0)
	srcIdx, i := vlqDecode(doc.Mappings, i)
	srcLine, i := vlqDecode(doc.Mappings, i)
	srcCol, _ := vlqDecode(doc.Mappings, i)
	if genCol != 0 || srcIdx != 0 || srcLine != 0 || srcCol != 0 {
		t.Errorf("first segment = (%d,%d,%d,%d), want (0,0,0,0)", genCol, srcIdx, srcLine, srcCol)
	}
}

// TestSourceMapCompressed verifies a map is still produced for compressed output
// (where everything is on generated line 0).
func TestSourceMapCompressed(t *testing.T) {
	_, mapJSON, err := styl.CompileMap("a\n  x 1\nb\n  y 2\n", styl.Options{Pretty: false})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var doc struct {
		Mappings string `json:"mappings"`
	}
	if err := json.Unmarshal([]byte(mapJSON), &doc); err != nil {
		t.Fatal(err)
	}
	// Single generated line ⇒ no ';' separators, but multiple comma segments.
	if strings.Contains(doc.Mappings, ";") {
		t.Errorf("compressed mappings should be one line, got %q", doc.Mappings)
	}
	if !strings.Contains(doc.Mappings, ",") {
		t.Errorf("expected multiple segments on one line, got %q", doc.Mappings)
	}
}
