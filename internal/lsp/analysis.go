package lsp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/diag"
	"github.com/rohanthewiz/go-styl/internal/eval"
	"github.com/rohanthewiz/go-styl/internal/parser"
)

// analysis is everything the server knows about one open document, rebuilt
// on each change:
//
//	text ──parser.Parse──▶ sheet ──collectDefs (+ @import files)──▶ defs
//	                         │
//	                         └──eval.ExtractManifest (sandboxed)──▶ vars, diags
//
// The evaluation is the real compile (minus rendering CSS), so diagnostics
// are exactly the errors `styl` would report and hover values are exactly
// what the CSS would contain.
type analysis struct {
	sheet *ast.Stylesheet   // nil when the text doesn't parse
	defs  []def             // definitions in the document, then in its imports
	vars  map[string]string // computed root-scope variable values
	diags []Diagnostic
}

// defKind distinguishes the two kinds of name a Stylus sheet defines.
type defKind int

const (
	defVar  defKind = iota // assignment, parameter, or for-loop variable
	defFunc                // function/mixin definition
)

// def is one definition site.
//
// Scope. Stylus scopes variables lexically: function/mixin bodies and
// selector blocks open a scope, if/for bodies don't. A def records the line
// span of the scope it lives in (FromLine–ToLine, 0 for the file's root),
// so go-to-definition can skip a same-named local that isn't visible from
// the cursor.
type def struct {
	Name     string
	Kind     defKind
	File     string // absolute path; "" for an untitled document
	Line     int    // 1-based line of the definition
	Sig      string // source text of the defining line, for hover/completion
	FromLine int    // scope span (1-based, inclusive); 0,0 means root scope
	ToLine   int
}

// root reports whether the def is at its file's top level.
func (d def) root() bool { return d.FromLine == 0 }

// visibleAt reports whether a use at line (in the def's own file) can see d.
func (d def) visibleAt(line int) bool {
	return d.root() || (line >= d.FromLine && line <= d.ToLine)
}

// evalTimeout bounds each analysis compile. A sheet being typed can be
// momentarily pathological (a loop over 1..1e9); the sandbox turns that into
// a warning instead of a hung server.
const evalTimeout = time.Second

// analyze builds the analysis of a document. path is its OS path ("" for an
// untitled buffer); overlay returns the text of other open documents so
// definitions come from unsaved editor state.
func analyze(path, text string, overlay func(path string) (string, bool)) *analysis {
	a := &analysis{vars: map[string]string{}}
	lines := strings.Split(text, "\n")

	sheet, err := parser.Parse(text)
	if err != nil {
		a.diags = append(a.diags, errDiag(err, path, "", lines, nil))
		return a
	}
	a.sheet = sheet
	a.defs = collectDefs(sheet.Statements, path, lines)
	a.defs = append(a.defs, importedDefs(sheet, path, overlay)...)

	// Evaluate in a sandbox over an os.DirFS at the filesystem root: the
	// sandbox's step/time/value budgets need an fs.FS (it never touches the
	// OS disk directly), and a root DirFS gives imports the same reach a
	// normal compile has.
	root, fsPath := fsLocation(path)
	var warns []string
	opts := eval.Options{
		Pretty:   true,
		FS:       os.DirFS(root),
		Filename: fsPath,
		BaseDir:  filepathDirSlash(fsPath),
		Warn:     func(msg string) { warns = append(warns, msg) },
		Sandbox: &eval.Sandbox{
			Deadline:       time.Now().Add(evalTimeout),
			MaxSteps:       2_000_000,
			MaxValueBytes:  1 << 20,
			MaxOutputBytes: 32 << 20,
		},
	}
	m, err := eval.ExtractManifest(sheet, opts)
	if err != nil {
		a.diags = append(a.diags, errDiag(err, path, root, lines, sheet))
	} else {
		for _, v := range m.Vars {
			a.vars[v.Name] = v.Value
		}
	}
	for _, w := range warns {
		// warn() carries no position; the top of the file is the one place
		// every client shows.
		a.diags = append(a.diags, Diagnostic{
			Range:    lineRange(lines, 0),
			Severity: SeverityWarning,
			Source:   "styl",
			Message:  "warn(): " + w,
		})
	}
	return a
}

// fsLocation splits an OS path into a DirFS root (the volume root) and the
// slash-separated fs path of the file under it. An untitled document is
// placed in the working directory, so relative imports still resolve
// somewhere sensible.
func fsLocation(path string) (root, fsPath string) {
	if path == "" {
		wd, err := os.Getwd()
		if err != nil {
			wd = "/"
		}
		path = filepath.Join(wd, "untitled.styl")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	root = filepath.VolumeName(abs) + string(filepath.Separator)
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		rel = abs
	}
	return root, filepath.ToSlash(rel)
}

func filepathDirSlash(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return "."
}

// errDiag turns a compile error into a diagnostic on the document.
//
// An error inside an imported file is still reported on this document (the
// client only shows diagnostics for files it asked about, and this
// document's compile is what failed), at its first @import line, with the
// imported file's position in the message.
func errDiag(err error, path, fsRoot string, lines []string, sheet *ast.Stylesheet) Diagnostic {
	sev := SeverityError
	if errors.Is(err, eval.ErrLimit) {
		// A budget ran out: likely mid-edit, not a bug in the sheet.
		sev = SeverityWarning
	}
	msg, line, col := err.Error(), 0, 0
	var de *diag.Error
	if errors.As(err, &de) {
		msg, line, col = de.Msg, de.Line, de.Col
		file := de.File
		if file != "" && fsRoot != "" {
			file = filepath.Join(fsRoot, filepath.FromSlash(file))
		}
		if file != "" && !samePath(file, path) {
			loc := file
			if de.Line > 0 {
				loc = fmt.Sprintf("%s:%d", file, de.Line)
			}
			msg = fmt.Sprintf("in %s: %s", loc, de.Msg)
			line, col = firstImportLine(sheet), 0
		}
	}
	r := lineRange(lines, max(line-1, 0))
	if col > 1 && line > 0 && line <= len(lines) && !strings.Contains(lines[line-1], "\t") {
		// Columns are rune-based but tab-expanded, so they're only trusted
		// on tab-free lines; otherwise the whole line is marked.
		lr := []rune(lines[line-1])
		if col-1 < len(lr) {
			r.Start.Character = utf16Col(lines[line-1], col-1)
		}
	}
	return Diagnostic{Range: r, Severity: sev, Source: "styl", Message: msg}
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return a == b
	}
	aa, _ := filepath.Abs(a)
	bb, _ := filepath.Abs(b)
	return aa == bb
}

// firstImportLine is the line of the first non-literal @import/@require at
// the top of the sheet, or 1.
func firstImportLine(sheet *ast.Stylesheet) int {
	if sheet != nil {
		for _, s := range sheet.Statements {
			if imp, ok := s.(*ast.Import); ok && !imp.Literal {
				return imp.Line
			}
		}
	}
	return 1
}

// lineRange spans the content of line i (0-based), from its first
// non-space character to its end.
func lineRange(lines []string, i int) Range {
	if i >= len(lines) {
		i = max(len(lines)-1, 0)
	}
	text := ""
	if i < len(lines) {
		text = strings.TrimRight(lines[i], "\r")
	}
	start := len([]rune(text)) - len([]rune(strings.TrimLeft(text, " \t")))
	return Range{
		Start: Position{Line: i, Character: utf16Col(text, start)},
		End:   Position{Line: i, Character: utf16Col(text, len([]rune(text)))},
	}
}

// --- definition index ---

// collectDefs walks statements and records every definition with its
// scope span. file is the path recorded on each def; lines is the file's
// text, for signatures.
func collectDefs(stmts []ast.Stmt, file string, lines []string) []def {
	var out []def
	var walk func(stmts []ast.Stmt, from, to int)
	sig := func(line int) string {
		if line < 1 || line > len(lines) {
			return ""
		}
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(lines[line-1]), "{"))
	}
	// scope opens a nested scope spanning a block's statements.
	scope := func(line int, body []ast.Stmt) (int, int) {
		return line, max(line, lastLine(body))
	}
	walk = func(stmts []ast.Stmt, from, to int) {
		for _, s := range stmts {
			switch n := s.(type) {
			case *ast.Assignment:
				out = append(out, def{Name: n.Name, Kind: defVar, File: file, Line: n.Line, Sig: sig(n.Line), FromLine: from, ToLine: to})
			case *ast.FuncDef:
				out = append(out, def{Name: n.Name, Kind: defFunc, File: file, Line: n.Line, Sig: sig(n.Line), FromLine: from, ToLine: to})
				f, t := scope(n.Line, n.Body)
				for _, p := range n.Params {
					out = append(out, def{Name: p.Name, Kind: defVar, File: file, Line: n.Line, Sig: "parameter of " + sig(n.Line), FromLine: f, ToLine: t})
				}
				walk(n.Body, f, t)
			case *ast.RuleSet:
				f, t := scope(n.Line, n.Body)
				walk(n.Body, f, t)
			case *ast.AtRule:
				f, t := scope(n.Line, n.Body)
				walk(n.Body, f, t)
			case *ast.MixinCall:
				f, t := scope(n.Line, n.Block)
				walk(n.Block, f, t)
			case *ast.If:
				for _, b := range n.Branches {
					walk(b.Body, from, to)
				}
				walk(n.Else, from, to)
			case *ast.For:
				// Loop variables are visible in the loop body only; other
				// assignments in the body belong to the enclosing scope.
				f, t := scope(n.Line, n.Body)
				for _, name := range []string{n.Value, n.Index} {
					if name != "" {
						out = append(out, def{Name: name, Kind: defVar, File: file, Line: n.Line, Sig: sig(n.Line), FromLine: f, ToLine: t})
					}
				}
				walk(n.Body, from, to)
			}
		}
	}
	walk(stmts, 0, 0)
	return out
}

// lastLine is the last source line any statement in stmts (recursively)
// starts on: the AST records where statements begin, not where blocks end,
// so this is where a block's scope is taken to end.
func lastLine(stmts []ast.Stmt) int {
	last := 0
	for _, s := range stmts {
		l, _ := ast.Pos(s)
		last = max(last, l)
		for _, body := range bodies(s) {
			last = max(last, lastLine(body))
		}
	}
	return last
}

// bodies returns a statement's nested statement lists.
func bodies(s ast.Stmt) [][]ast.Stmt {
	switch n := s.(type) {
	case *ast.RuleSet:
		return [][]ast.Stmt{n.Body}
	case *ast.FuncDef:
		return [][]ast.Stmt{n.Body}
	case *ast.AtRule:
		return [][]ast.Stmt{n.Body}
	case *ast.MixinCall:
		return [][]ast.Stmt{n.Block}
	case *ast.For:
		return [][]ast.Stmt{n.Body}
	case *ast.If:
		out := [][]ast.Stmt{n.Else}
		for _, b := range n.Branches {
			out = append(out, b.Body)
		}
		return out
	}
	return nil
}

// maxImportDepth bounds how deep definition collection follows imports; a
// compile has its own cycle handling, the index just needs to terminate.
const maxImportDepth = 16

// importedDefs collects the root-level definitions of every .styl file the
// sheet imports, transitively. Only root definitions are visible to the
// importer, so nested ones are dropped.
func importedDefs(sheet *ast.Stylesheet, path string, overlay func(string) (string, bool)) []def {
	if path == "" {
		return nil
	}
	seen := map[string]bool{}
	if abs, err := filepath.Abs(path); err == nil {
		seen[abs] = true
	}
	var out []def
	var visit func(stmts []ast.Stmt, dir string, depth int)
	visit = func(stmts []ast.Stmt, dir string, depth int) {
		if depth > maxImportDepth {
			return
		}
		for _, imp := range imports(stmts) {
			files, err := eval.ResolveImport(dir, imp.Path, nil)
			if err != nil {
				continue
			}
			for _, f := range files {
				if seen[f] {
					continue
				}
				seen[f] = true
				text, ok := overlay(f)
				if !ok {
					data, err := os.ReadFile(f)
					if err != nil {
						continue
					}
					text = string(data)
				}
				sub, err := parser.Parse(text)
				if err != nil {
					continue
				}
				for _, d := range collectDefs(sub.Statements, f, strings.Split(text, "\n")) {
					if d.root() {
						out = append(out, d)
					}
				}
				visit(sub.Statements, filepath.Dir(f), depth+1)
			}
		}
	}
	visit(sheet.Statements, filepath.Dir(path), 0)
	return out
}

// imports returns the non-literal imports among stmts, including those
// nested in blocks (an @import inside a mixin still inlines a file).
func imports(stmts []ast.Stmt) []*ast.Import {
	var out []*ast.Import
	for _, s := range stmts {
		if imp, ok := s.(*ast.Import); ok && !imp.Literal {
			out = append(out, imp)
		}
		for _, b := range bodies(s) {
			out = append(out, imports(b)...)
		}
	}
	return out
}

// lookup finds the definition a use of name at line (1-based) in file
// refers to, preferring, in order:
//
//  1. a visible definition in the same file at or before the line — the
//     latest one, and among equals the innermost scope (a local shadows a
//     global);
//  2. any visible definition in the same file (a function called above
//     where it's defined, which a mixin body may legitimately do);
//  3. a definition from an imported file.
func (a *analysis) lookup(name, file string, line int, kind defKind) (def, bool) {
	var best def
	found := false
	better := func(d def) bool {
		if !found {
			return true
		}
		if d.Line != best.Line {
			return d.Line > best.Line
		}
		return !d.root() && (best.root() || d.ToLine-d.FromLine < best.ToLine-best.FromLine)
	}
	for _, d := range a.defs {
		if d.Name == name && d.Kind == kind && d.File == file && d.Line <= line && d.visibleAt(line) && better(d) {
			best, found = d, true
		}
	}
	if found {
		return best, true
	}
	for _, d := range a.defs {
		if d.Name == name && d.Kind == kind && d.File == file && d.visibleAt(line) {
			return d, true
		}
	}
	for _, d := range a.defs {
		if d.Name == name && d.Kind == kind && d.File != file {
			return d, true
		}
	}
	return def{}, false
}
