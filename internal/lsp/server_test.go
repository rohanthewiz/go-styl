package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// client drives a Server over in-memory pipes, as an editor would over stdio.
type client struct {
	t      *testing.T
	in     *io.PipeWriter // client → server
	out    *conn          // server → client, read side
	nextID int
	notes  []message // notifications received while waiting for replies
	done   chan error
}

func newClient(t *testing.T) *client {
	t.Helper()
	cr, cw := io.Pipe() // client writes, server reads
	sr, sw := io.Pipe() // server writes, client reads
	c := &client{t: t, in: cw, out: &conn{r: bufio.NewReader(sr)}, done: make(chan error, 1)}
	go func() { c.done <- Serve(cr, sw, nil); sw.Close() }()
	t.Cleanup(func() { cw.Close() })
	c.call("initialize", map[string]any{"capabilities": map[string]any{}})
	c.notify("initialized", map[string]any{})
	return c
}

func (c *client) send(m map[string]any) {
	m["jsonrpc"] = "2.0"
	body, _ := json.Marshal(m)
	fmt.Fprintf(c.in, "Content-Length: %d\r\n\r\n%s", len(body), body)
}

func (c *client) notify(method string, params any) {
	c.send(map[string]any{"method": method, "params": params})
}

// call sends a request and returns its raw result, collecting any
// notifications that arrive first.
func (c *client) call(method string, params any) json.RawMessage {
	c.t.Helper()
	c.nextID++
	id := c.nextID
	c.send(map[string]any{"id": id, "method": method, "params": params})
	for {
		m := c.read()
		if m.Method != "" {
			c.notes = append(c.notes, *m)
			continue
		}
		if string(m.ID) != fmt.Sprint(id) {
			c.t.Fatalf("reply to %s: unexpected id %s", method, m.ID)
		}
		if m.Error != nil {
			c.t.Fatalf("%s: error %d %s", method, m.Error.Code, m.Error.Message)
		}
		raw, _ := json.Marshal(m.Result)
		return raw
	}
}

// read returns the next server message, failing the test after a timeout.
func (c *client) read() *message {
	c.t.Helper()
	type res struct {
		m   *message
		err error
	}
	ch := make(chan res, 1)
	go func() {
		// Results decode into json.RawMessage-friendly form via a second pass.
		m, err := c.out.read()
		ch <- res{m, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			c.t.Fatalf("read: %v", r.err)
		}
		return r.m
	case <-time.After(5 * time.Second):
		c.t.Fatal("timed out waiting for the server")
		return nil
	}
}

// diagnostics opens (or re-sends) a document and returns the diagnostics
// the server publishes for it.
func (c *client) open(uri, text string) []Diagnostic {
	c.t.Helper()
	c.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": "stylus", "version": 1, "text": text},
	})
	return c.waitDiags(uri)
}

func (c *client) change(uri, text string) []Diagnostic {
	c.t.Helper()
	c.notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": uri, "version": 2},
		"contentChanges": []map[string]any{{"text": text}},
	})
	return c.waitDiags(uri)
}

func (c *client) waitDiags(uri string) []Diagnostic {
	c.t.Helper()
	for {
		m := c.read()
		if m.Method != "textDocument/publishDiagnostics" {
			c.notes = append(c.notes, *m)
			continue
		}
		var p PublishDiagnosticsParams
		if err := json.Unmarshal(m.Params, &p); err != nil {
			c.t.Fatal(err)
		}
		if p.URI == uri {
			return p.Diagnostics
		}
	}
}

func pos(uri string, line, char int) map[string]any {
	return map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     map[string]any{"line": line, "character": char},
	}
}

func doc(uri string) map[string]any {
	return map[string]any{"textDocument": map[string]any{"uri": uri}}
}

// project writes a two-file project: main.styl imports _vars.styl.
func project(t *testing.T) (dir, mainURI string, mainSrc string) {
	t.Helper()
	dir = t.TempDir()
	vars := "brand = #0af\npad = 4px\nbutton(bg = brand)\n  background bg\n  padding pad * 2\n"
	if err := os.WriteFile(filepath.Join(dir, "_vars.styl"), []byte(vars), 0o644); err != nil {
		t.Fatal(err)
	}
	mainSrc = "@import '_vars'\n" + // 0
		"gap = pad * 3\n" + //          1
		".card\n" + //                  2
		"  margin gap\n" + //           3
		"  color #fff\n" + //           4
		"  button()\n" + //             5
		"  gap = 1px\n" + //            6
		"  padding gap\n" + //          7
		"#bad\n" + //                   8
		"  border 1px solid brand\n" // 9
	p := filepath.Join(dir, "main.styl")
	if err := os.WriteFile(p, []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, pathToURI(p), mainSrc
}

func TestServerFeatures(t *testing.T) {
	dir, uri, src := project(t)
	c := newClient(t)

	if diags := c.open(uri, src); len(diags) != 0 {
		t.Fatalf("clean file: got diagnostics %+v", diags)
	}

	t.Run("hover shows the computed value", func(t *testing.T) {
		var h Hover
		json.Unmarshal(c.call("textDocument/hover", pos(uri, 3, 10)), &h)
		if !strings.Contains(h.Contents.Value, "gap = pad * 3") || !strings.Contains(h.Contents.Value, "Computed: `12px`") {
			t.Errorf("hover: %q", h.Contents.Value)
		}
	})

	t.Run("definition follows the import", func(t *testing.T) {
		var loc Location
		json.Unmarshal(c.call("textDocument/definition", pos(uri, 5, 4)), &loc)
		if !strings.HasSuffix(loc.URI, "/_vars.styl") || loc.Range.Start.Line != 2 {
			t.Errorf("definition of button: %+v", loc)
		}
	})

	t.Run("a local shadows the global", func(t *testing.T) {
		var loc Location
		json.Unmarshal(c.call("textDocument/definition", pos(uri, 7, 11)), &loc)
		if loc.URI != uri || loc.Range.Start.Line != 6 {
			t.Errorf("definition of local gap: %+v", loc)
		}
		json.Unmarshal(c.call("textDocument/definition", pos(uri, 3, 10)), &loc)
		if loc.Range.Start.Line != 1 {
			t.Errorf("definition of root gap: %+v", loc)
		}
	})

	t.Run("completion", func(t *testing.T) {
		var list CompletionList
		json.Unmarshal(c.call("textDocument/completion", pos(uri, 3, 2)), &list)
		want := map[string]bool{"brand": false, "button": false, "darken": false, "gap": false}
		for _, it := range list.Items {
			if _, ok := want[it.Label]; ok {
				want[it.Label] = true
			}
			if it.Label == "brand" && it.Detail != "#0af" {
				t.Errorf("brand detail = %q, want its computed value", it.Detail)
			}
		}
		for name, ok := range want {
			if !ok {
				t.Errorf("completion is missing %q", name)
			}
		}
	})

	t.Run("colors skip ID selectors", func(t *testing.T) {
		var cols []ColorInformation
		json.Unmarshal(c.call("textDocument/documentColor", doc(uri)), &cols)
		if len(cols) != 1 || cols[0].Range.Start.Line != 4 || cols[0].Range.Start.Character != 8 || cols[0].Color.Red != 1 {
			t.Errorf("colors: %+v", cols)
		}
	})

	t.Run("symbols", func(t *testing.T) {
		var syms []DocumentSymbol
		json.Unmarshal(c.call("textDocument/documentSymbol", doc(uri)), &syms)
		var names []string
		for _, s := range syms {
			names = append(names, s.Name)
		}
		if got := strings.Join(names, ","); got != "gap,.card,#bad" {
			t.Errorf("symbols: %s", got)
		}
		if len(syms) > 1 && (len(syms[1].Children) != 1 || syms[1].Range.End.Line != 7) {
			t.Errorf(".card symbol: %+v", syms[1])
		}
	})

	t.Run("formatting", func(t *testing.T) {
		c.change(uri, ".a\n    color   red\n")
		var edits []TextEdit
		json.Unmarshal(c.call("textDocument/formatting", doc(uri)), &edits)
		if len(edits) != 1 || edits[0].NewText != ".a\n  color red\n" {
			t.Errorf("formatting edits: %+v", edits)
		}
	})

	t.Run("diagnostics", func(t *testing.T) {
		diags := c.change(uri, ".a\n  color red\n  +missing-mixin()\n")
		if len(diags) != 1 || diags[0].Range.Start.Line != 2 || !strings.Contains(diags[0].Message, "undefined mixin") {
			t.Errorf("undefined mixin: %+v", diags)
		}
		diags = c.change(uri, ".a\n  color (red\n")
		if len(diags) != 1 || diags[0].Range.Start.Line != 1 || diags[0].Severity != SeverityError {
			t.Errorf("parse error: %+v", diags)
		}
		// Mid-edit breakage keeps definitions from the last good parse.
		var h Hover
		c.change(uri, "w = 2px\n.a\n  width w\n")
		c.change(uri, "w = 2px\n.a\n  width w\n  color (\n")
		json.Unmarshal(c.call("textDocument/hover", pos(uri, 2, 8)), &h)
		if !strings.Contains(h.Contents.Value, "w = 2px") {
			t.Errorf("hover on broken text: %q", h.Contents.Value)
		}
	})

	t.Run("errors in an import land on the import line", func(t *testing.T) {
		bad := filepath.Join(dir, "_bad.styl")
		os.WriteFile(bad, []byte(".x\n  +nope()\n"), 0o644)
		diags := c.change(uri, "a = 1\n@import '_bad'\n")
		if len(diags) != 1 || diags[0].Range.Start.Line != 1 || !strings.Contains(diags[0].Message, "_bad.styl:2") {
			t.Errorf("import error: %+v", diags)
		}
	})

	t.Run("runaway loops are cut off", func(t *testing.T) {
		start := time.Now()
		diags := c.change(uri, "for i in 1..60000\n  for j in 1..60000\n    x = j\n")
		if len(diags) != 1 || diags[0].Severity != SeverityWarning || time.Since(start) > 4*time.Second {
			t.Errorf("runaway loop: %+v after %v", diags, time.Since(start))
		}
	})

	c.call("shutdown", nil)
	c.notify("exit", nil)
	if err := <-c.done; err != nil {
		t.Errorf("Serve: %v", err)
	}
}

// changeWatch sends a full-text change to uri and returns the diagnostics
// then published for watch (a document that imports uri).
func (c *client) changeWatch(uri, text, watch string) []Diagnostic {
	c.t.Helper()
	c.notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": uri, "version": 3},
		"contentChanges": []map[string]any{{"text": text}},
	})
	c.waitDiags(uri)
	return c.waitDiags(watch)
}

// TestUnsavedImportOverlay: an unsaved edit to an open imported file reaches
// the importer's diagnostics and hover values right away (N-045), and
// closing the buffer without saving reverts the importer to the disk text.
func TestUnsavedImportOverlay(t *testing.T) {
	dir, uri, src := project(t)
	varsURI := pathToURI(filepath.Join(dir, "_vars.styl"))
	varsSrc, _ := os.ReadFile(filepath.Join(dir, "_vars.styl"))
	c := newClient(t)
	c.open(uri, src)
	c.open(varsURI, string(varsSrc))
	c.waitDiags(uri) // main imports _vars, so it is re-analyzed too

	hoverGap := func() string {
		var h Hover
		json.Unmarshal(c.call("textDocument/hover", pos(uri, 3, 10)), &h)
		return h.Contents.Value
	}
	if v := hoverGap(); !strings.Contains(v, "Computed: `12px`") {
		t.Fatalf("hover before edit: %q", v)
	}

	// pad 4px → 5px, unsaved: gap = pad * 3 is now 15px.
	edited := strings.Replace(string(varsSrc), "pad = 4px", "pad = 5px", 1)
	if diags := c.changeWatch(varsURI, edited, uri); len(diags) != 0 {
		t.Fatalf("importer diagnostics after a clean edit: %+v", diags)
	}
	if v := hoverGap(); !strings.Contains(v, "Computed: `15px`") {
		t.Errorf("hover after unsaved edit: %q", v)
	}

	// Breaking the import shows on the importer's @import line.
	diags := c.changeWatch(varsURI, edited+".x\n  +nope()\n", uri)
	if len(diags) != 1 || diags[0].Range.Start.Line != 0 || !strings.Contains(diags[0].Message, "_vars.styl:7") {
		t.Errorf("importer diagnostics after a breaking edit: %+v", diags)
	}

	// Close without saving: the importer compiles the disk text again.
	c.notify("textDocument/didClose", doc(varsURI))
	c.waitDiags(varsURI)
	if diags := c.waitDiags(uri); len(diags) != 0 {
		t.Errorf("importer diagnostics after close: %+v", diags)
	}
	if v := hoverGap(); !strings.Contains(v, "Computed: `12px`") {
		t.Errorf("hover after close: %q", v)
	}
}

// TestOverlayFSUnsavedFile: a buffer that exists only in the editor can be
// imported by name, and reads are recorded as deps while Stat probes aren't.
func TestOverlayFSUnsavedFile(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "main.styl")
	newFile := filepath.Join(dir, "_new.styl")
	os.WriteFile(filepath.Join(dir, "_disk.styl"), []byte("d = 1px\n"), 0o644)
	src := "@import '_new'\n@import '_disk'\n.a\n  width n + d\n"
	an := analyze(main, src, map[string]string{newFile: "n = 2px\n", main: src}, false)
	if len(an.diags) != 0 {
		t.Fatalf("diags: %+v", an.diags)
	}
	if an.vars["n"] != "2px" {
		t.Errorf("n = %q", an.vars["n"])
	}
	if !an.deps[newFile] || !an.deps[filepath.Join(dir, "_disk.styl")] || len(an.deps) != 2 {
		t.Errorf("deps: %v", an.deps)
	}
}
