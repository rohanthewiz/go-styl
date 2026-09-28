package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"unicode"

	"github.com/rohanthewiz/go-styl/internal/eval"
	"github.com/rohanthewiz/go-styl/internal/parser"
)

// Version is reported to clients in serverInfo.
const Version = "0.1.0"

// document is one open text document.
type document struct {
	uri     string
	path    string // OS path, "" for non-file URIs
	version int
	text    string
	lines   []string
	an      *analysis
	// lastGood is the most recent analysis whose text parsed. Mid-edit text
	// is usually broken, and completion/hover/definition keep working off
	// the last parse that succeeded rather than going blank.
	lastGood *analysis
}

// Server is a Stylus language server. Requests are handled one at a time on
// the read loop: every handler is fast (a full re-analysis is one compile,
// bounded by evalTimeout), so ordering is simple and no document state
// needs locking.
type Server struct {
	c        *conn
	docs     map[string]*document
	shutdown bool
	log      *log.Logger
}

// Serve runs a language server over r/w (stdin/stdout for an editor) until
// the client sends exit or closes the stream. Diagnostic logging goes to
// logw (stderr; nil discards it), since stdout is the protocol channel.
func Serve(r io.Reader, w io.Writer, logw io.Writer) error {
	if logw == nil {
		logw = io.Discard
	}
	s := &Server{
		c:    newConn(r, w),
		docs: map[string]*document{},
		log:  log.New(logw, "styl-lsp: ", log.LstdFlags),
	}
	for {
		m, err := s.c.read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			var re *rpcError
			if errors.As(err, &re) {
				// Malformed JSON: report it and keep serving.
				_ = s.c.reply(json.RawMessage("null"), nil, re)
				continue
			}
			return err
		}
		if m.Method == "exit" {
			if !s.shutdown {
				return errors.New("exit before shutdown")
			}
			return nil
		}
		s.handle(m)
	}
}

// handle dispatches one message. Requests (with an ID) always get a reply;
// unknown notifications are ignored, as the protocol requires.
func (s *Server) handle(m *message) {
	isRequest := len(m.ID) > 0
	result, rerr := s.dispatch(m)
	if !isRequest {
		if rerr != nil {
			s.log.Printf("%s: %s", m.Method, rerr.Message)
		}
		return
	}
	if err := s.c.reply(m.ID, result, rerr); err != nil {
		s.log.Printf("reply %s: %v", m.Method, err)
	}
}

func (s *Server) dispatch(m *message) (any, *rpcError) {
	if s.shutdown && m.Method != "exit" && len(m.ID) > 0 {
		return nil, &rpcError{Code: codeInvalidRequest, Message: "server is shutting down"}
	}
	switch m.Method {
	case "initialize":
		return s.initialize(), nil
	case "initialized", "$/cancelRequest", "$/setTrace", "workspace/didChangeConfiguration":
		return nil, nil
	case "shutdown":
		s.shutdown = true
		return nil, nil

	case "textDocument/didOpen":
		var p DidOpenParams
		if err := unmarshal(m.Params, &p); err != nil {
			return nil, err
		}
		d := &document{uri: p.TextDocument.URI, path: uriToPath(p.TextDocument.URI), version: p.TextDocument.Version}
		s.docs[d.uri] = d
		s.update(d, p.TextDocument.Text)
		return nil, nil
	case "textDocument/didChange":
		var p DidChangeParams
		if err := unmarshal(m.Params, &p); err != nil {
			return nil, err
		}
		d := s.docs[p.TextDocument.URI]
		if d == nil || len(p.ContentChanges) == 0 {
			return nil, nil
		}
		d.version = p.TextDocument.Version
		s.update(d, p.ContentChanges[len(p.ContentChanges)-1].Text)
		return nil, nil
	case "textDocument/didSave":
		// Open documents already compile against each other's editor text
		// (overlayFS), so a save changes nothing they see. It is still the
		// cue that files may have changed on disk behind the editor's back
		// (a branch switch, a generator), so everything is re-analyzed.
		for _, d := range s.docs {
			s.refresh(d)
		}
		return nil, nil
	case "textDocument/didClose":
		var p DidCloseParams
		if err := unmarshal(m.Params, &p); err != nil {
			return nil, err
		}
		closed := s.docs[p.TextDocument.URI]
		delete(s.docs, p.TextDocument.URI)
		// Clear the closed file's diagnostics from the problems view.
		_ = s.c.notify("textDocument/publishDiagnostics", PublishDiagnosticsParams{URI: p.TextDocument.URI, Diagnostics: []Diagnostic{}})
		// Closing a buffer with unsaved edits reverts importers to the text
		// on disk.
		if closed != nil {
			s.refreshDependents(closed)
		}
		return nil, nil

	case "textDocument/hover":
		return withPos(s, m, s.hover)
	case "textDocument/completion":
		return withPos(s, m, s.completion)
	case "textDocument/definition":
		return withPos(s, m, s.definition)
	case "textDocument/documentSymbol":
		return withDoc(s, m, func(d *document) (any, *rpcError) { return s.symbols(d), nil })
	case "textDocument/formatting":
		return withDoc(s, m, s.format)
	case "textDocument/documentColor":
		return withDoc(s, m, func(d *document) (any, *rpcError) { return documentColors(d), nil })
	case "textDocument/colorPresentation":
		var p ColorPresentationParams
		if err := unmarshal(m.Params, &p); err != nil {
			return nil, err
		}
		return colorPresentations(p.Color), nil
	}
	if len(m.ID) > 0 {
		return nil, &rpcError{Code: codeMethodNotFound, Message: "method not supported: " + m.Method}
	}
	return nil, nil
}

func unmarshal(raw json.RawMessage, v any) *rpcError {
	if err := json.Unmarshal(raw, v); err != nil {
		return &rpcError{Code: codeInvalidParams, Message: err.Error()}
	}
	return nil
}

// withDoc decodes a request's textDocument and runs f on the open document.
// A request for a document the client never opened gets a null result.
func withDoc(s *Server, m *message, f func(*document) (any, *rpcError)) (any, *rpcError) {
	var p DocumentParams
	if err := unmarshal(m.Params, &p); err != nil {
		return nil, err
	}
	d := s.docs[p.TextDocument.URI]
	if d == nil {
		return nil, nil
	}
	return f(d)
}

// withPos is withDoc for position requests (hover, completion, definition).
func withPos(s *Server, m *message, f func(*document, Position) (any, *rpcError)) (any, *rpcError) {
	var p TextDocumentPositionParams
	if err := unmarshal(m.Params, &p); err != nil {
		return nil, err
	}
	d := s.docs[p.TextDocument.URI]
	if d == nil {
		return nil, nil
	}
	return f(d, p.Position)
}

func (s *Server) initialize() any {
	return map[string]any{
		"capabilities": map[string]any{
			// Full-document sync: Stylus files are small, and a compile
			// needs the whole text anyway, so incremental edits would buy
			// nothing but patching code.
			"textDocumentSync": map[string]any{
				"openClose": true,
				"change":    1,
				"save":      map[string]any{"includeText": false},
			},
			"hoverProvider":              true,
			"completionProvider":         map[string]any{"triggerCharacters": []string{"$"}},
			"definitionProvider":         true,
			"documentSymbolProvider":     true,
			"documentFormattingProvider": true,
			"colorProvider":              true,
		},
		"serverInfo": map[string]any{"name": "styl-lsp", "version": Version},
	}
}

// update replaces a document's text, re-analyzes it and every open document
// whose last compile read it, and publishes their diagnostics.
func (s *Server) update(d *document, text string) {
	d.text = text
	d.lines = strings.Split(text, "\n")
	s.refresh(d)
	s.refreshDependents(d)
}

// refreshDependents re-analyzes the other open documents that import d
// (directly or through other files; an analysis's deps are transitive, so
// one pass is enough and nothing cascades).
func (s *Server) refreshDependents(d *document) {
	if d.path == "" {
		return
	}
	key := absPath(d.path)
	for _, o := range s.docs {
		// A document mid-edit that doesn't parse has no deps of its own; its
		// last good analysis says what it imports.
		if o != d && (o.an != nil && o.an.deps[key] || o.lastGood != nil && o.lastGood.deps[key]) {
			s.refresh(o)
		}
	}
}

// openFiles maps the absolute path of every open file-backed document to
// its editor text: the overlay an analysis compiles against.
func (s *Server) openFiles() map[string]string {
	out := make(map[string]string, len(s.docs))
	for _, d := range s.docs {
		if d.path != "" {
			out[absPath(d.path)] = d.text
		}
	}
	return out
}

// refresh re-analyzes a document's current text and publishes its
// diagnostics.
func (s *Server) refresh(d *document) {
	d.an = analyze(d.path, d.text, s.openFiles())
	if d.an.sheet != nil {
		d.lastGood = d.an
	}
	diags := d.an.diags
	if diags == nil {
		diags = []Diagnostic{} // an empty array clears; null is invalid
	}
	if err := s.c.notify("textDocument/publishDiagnostics", PublishDiagnosticsParams{URI: d.uri, Version: d.version, Diagnostics: diags}); err != nil {
		s.log.Printf("publishDiagnostics: %v", err)
	}
}

// overlay returns the editor text of an open document by OS path, so
// definitions in an imported file reflect unsaved edits.
func (s *Server) overlay(path string) (string, bool) {
	for _, d := range s.docs {
		if d.path != "" && samePath(d.path, path) {
			return d.text, true
		}
	}
	return "", false
}

// index is the analysis to answer queries from: the current one when it
// parsed, else the last one that did.
func (d *document) index() *analysis {
	if d.an != nil && d.an.sheet != nil {
		return d.an
	}
	if d.lastGood != nil {
		return d.lastGood
	}
	return d.an
}

// --- words under the cursor ---

// isWordRune reports whether c can be part of a Stylus identifier:
// variable and mixin names use letters, digits, '-', '_' and '$'.
func isWordRune(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '-' || c == '_' || c == '$'
}

// wordAt returns the identifier touching the position, its rune span on the
// line, and whether it's used as a call (`name(` or `+name`).
func (d *document) wordAt(pos Position) (word string, start, end int, call bool) {
	if pos.Line < 0 || pos.Line >= len(d.lines) {
		return "", 0, 0, false
	}
	line := []rune(d.lines[pos.Line])
	c := runeCol(d.lines[pos.Line], pos.Character)
	start, end = c, c
	for start > 0 && isWordRune(line[start-1]) {
		start--
	}
	for end < len(line) && isWordRune(line[end]) {
		end++
	}
	// A leading '-' is more often a minus sign than part of a name.
	for start < end && line[start] == '-' {
		start++
	}
	if start >= end {
		return "", c, c, false
	}
	call = (end < len(line) && line[end] == '(') || (start > 0 && line[start-1] == '+')
	return string(line[start:end]), start, end, call
}

// resolve finds the definition of the word at pos, trying the kind its
// syntax suggests first (a call wants a function) and then the other.
func (d *document) resolve(pos Position) (def, string, bool) {
	word, _, _, call := d.wordAt(pos)
	if word == "" {
		return def{}, "", false
	}
	an := d.index()
	if an == nil {
		return def{}, word, false
	}
	first, second := defVar, defFunc
	if call {
		first, second = defFunc, defVar
	}
	if df, ok := an.lookup(word, d.path, pos.Line+1, first); ok {
		return df, word, true
	}
	df, ok := an.lookup(word, d.path, pos.Line+1, second)
	return df, word, ok
}

// --- hover ---

func (s *Server) hover(d *document, pos Position) (any, *rpcError) {
	word, start, end, _ := d.wordAt(pos)
	if word == "" {
		return nil, nil
	}
	var b strings.Builder
	if df, _, ok := d.resolve(pos); ok {
		fmt.Fprintf(&b, "```stylus\n%s\n```", df.Sig)
		// The computed value is only meaningful for root-scope variables:
		// a local's value depends on the call it runs in.
		if df.Kind == defVar && df.root() {
			if v, ok := d.index().vars[word]; ok {
				fmt.Fprintf(&b, "\n\nComputed: `%s`", v)
			}
		}
		if df.File != d.path && df.File != "" {
			fmt.Fprintf(&b, "\n\nDefined in `%s:%d`", df.File, df.Line)
		}
	} else if isBuiltin(word) {
		fmt.Fprintf(&b, "```stylus\n%s()\n```\n\nBuilt-in function", word)
	} else {
		return nil, nil
	}
	line := d.lines[pos.Line]
	return Hover{
		Contents: MarkupContent{Kind: "markdown", Value: b.String()},
		Range: &Range{
			Start: Position{Line: pos.Line, Character: utf16Col(line, start)},
			End:   Position{Line: pos.Line, Character: utf16Col(line, end)},
		},
	}, nil
}

var builtinNames = eval.BuiltinNames()

func isBuiltin(name string) bool {
	i := sort.SearchStrings(builtinNames, name)
	return i < len(builtinNames) && builtinNames[i] == name
}

// --- completion ---

// keywords are Stylus's statement keywords and value literals.
var keywords = []string{"if", "else", "unless", "for", "in", "return", "true", "false", "null", "!important"}

// completion offers every name in reach: the document's and its imports'
// variables and functions (a root variable's detail is its computed value),
// the built-ins, and the keywords. Clients filter by the typed prefix
// themselves, so the list isn't pre-filtered.
func (s *Server) completion(d *document, pos Position) (any, *rpcError) {
	var items []CompletionItem
	seen := map[string]bool{}
	add := func(it CompletionItem) {
		key := fmt.Sprintf("%d/%s", it.Kind, it.Label)
		if !seen[key] {
			seen[key] = true
			items = append(items, it)
		}
	}
	if an := d.index(); an != nil {
		line := pos.Line + 1
		for _, df := range an.defs {
			if df.File == d.path && !df.visibleAt(line) {
				continue
			}
			switch df.Kind {
			case defVar:
				detail := df.Sig
				if v, ok := an.vars[df.Name]; ok && df.root() {
					detail = v
				}
				add(CompletionItem{Label: df.Name, Kind: kindVariable, Detail: detail})
			case defFunc:
				add(CompletionItem{Label: df.Name, Kind: kindFunction, Detail: df.Sig})
			}
		}
	}
	for _, name := range builtinNames {
		add(CompletionItem{Label: name, Kind: kindFunction, Detail: "built-in"})
	}
	for _, k := range keywords {
		add(CompletionItem{Label: k, Kind: kindKeyword})
	}
	return CompletionList{Items: items}, nil
}

// --- definition ---

func (s *Server) definition(d *document, pos Position) (any, *rpcError) {
	df, _, ok := d.resolve(pos)
	if !ok {
		return nil, nil
	}
	uri := d.uri
	lines := d.lines
	if df.File != d.path {
		uri = pathToURI(df.File)
		text, ok := s.overlay(df.File)
		if !ok {
			text = readFile(df.File)
		}
		lines = strings.Split(text, "\n")
	}
	return Location{URI: uri, Range: nameRange(lines, df.Line-1, df.Name)}, nil
}

// nameRange locates name on line i (0-based): the AST records a
// statement's line, and the name is found on it textually (its column
// fields are tab-expanded, so not usable as rune offsets).
func nameRange(lines []string, i int, name string) Range {
	r := lineRange(lines, i)
	if i < 0 || i >= len(lines) {
		return r
	}
	line := lines[i]
	runes := []rune(line)
	nr := []rune(name)
	for k := 0; k+len(nr) <= len(runes); k++ {
		if string(runes[k:k+len(nr)]) != name {
			continue
		}
		if (k > 0 && isWordRune(runes[k-1])) || (k+len(nr) < len(runes) && isWordRune(runes[k+len(nr)])) {
			continue // part of a longer word
		}
		return Range{
			Start: Position{Line: i, Character: utf16Col(line, k)},
			End:   Position{Line: i, Character: utf16Col(line, k+len(nr))},
		}
	}
	return r
}

// --- formatting ---

// format answers textDocument/formatting with styl fmt's output as a single
// whole-document edit (or none when already formatted). A sheet that doesn't
// parse is an error: formatting it would mean guessing at its structure.
func (s *Server) format(d *document) (any, *rpcError) {
	out, err := parser.Format(d.text)
	if err != nil {
		return nil, &rpcError{Code: codeRequestFailed, Message: err.Error()}
	}
	if out == d.text {
		return []TextEdit{}, nil
	}
	last := len(d.lines) - 1
	return []TextEdit{{
		Range: Range{
			Start: Position{},
			End:   Position{Line: last, Character: utf16Col(d.lines[last], len([]rune(d.lines[last])))},
		},
		NewText: out,
	}}, nil
}
