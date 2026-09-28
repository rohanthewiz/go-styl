package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/parser"
)

// --- use sites ---
//
// Expressions in the AST carry no positions (only statements do), so a use
// site is found in two steps:
//
//  1. From the AST: the set of names each statement's expressions mention
//     (identifiers, call names, and words inside `{…}` interpolation in
//     strings, selectors and property names), attached to the source lines
//     the statement occupies.
//  2. From the text: every word occurrence on those lines whose name is in
//     that line's set, skipping definition sites, declaration property names
//     and words glued to a `.`, `#` or `@` (a class, an ID or a property
//     lookup, never a variable).
//
//	line 3:  "  padding pad * 2"        names(line 3) = {pad}
//	          ^^^^^^^ ^^^
//	          property  use ✓  (padding is not in the set; it's also a property)
//
// Comments and the non-interpolated parts of strings are blanked first
// (codeMask), so a word inside them never counts. The result is a
// superset-safe list: a use is only reported once lookup resolves it to a
// definition.

// use is one occurrence of a name in code position.
type use struct {
	Name       string
	Line       int // 1-based
	Start, End int // rune offsets on the line
	Call       bool
	// Prop marks a declaration's property name. It is a use only when it
	// names a mixin (a transparent mixin call: `size 10px` runs size()).
	Prop bool
}

// resolveUse finds the definition a use refers to in an index, trying the
// kind its syntax suggests first (see document.resolve). A property name
// only resolves to a function, and not from inside that function's own
// body, where Stylus emits the property instead of recursing.
func resolveUse(an *analysis, file string, u use) (def, bool) {
	if u.Prop {
		d, ok := an.lookup(u.Name, file, u.Line, defFunc)
		if ok && d.File == file && u.Line > d.Line && u.Line <= d.BodyTo {
			return def{}, false
		}
		return d, ok
	}
	first, second := defVar, defFunc
	if u.Call {
		first, second = defFunc, defVar
	}
	if d, ok := an.lookup(u.Name, file, u.Line, first); ok {
		return d, true
	}
	return an.lookup(u.Name, file, u.Line, second)
}

// wordRE matches a Stylus identifier (see isWordRune).
var wordRE = regexp.MustCompile(`[\p{L}\p{N}_$-]+`)

// interpRE matches a `{…}` interpolation group (no nesting, which Stylus
// interpolation doesn't use).
var interpRE = regexp.MustCompile(`\{[^{}]*\}`)

// interpWords adds the words inside every `{…}` group of raw text.
func interpWords(raw string, add func(string)) {
	for _, g := range interpRE.FindAllString(raw, -1) {
		for _, w := range wordRE.FindAllString(g, -1) {
			add(w)
		}
	}
}

// exprNames adds every name an expression mentions.
func exprNames(e ast.Expr, add func(string)) {
	switch n := e.(type) {
	case *ast.Ident:
		// A raw url(...) token is an Ident; its contents may reference
		// variables (evalURL), so parse and recurse.
		if len(n.Name) > 4 && strings.EqualFold(n.Name[:4], "url(") && strings.HasSuffix(n.Name, ")") {
			if inner, err := parser.ParseExpr(strings.TrimSpace(n.Name[4:len(n.Name)-1]), 0); err == nil {
				exprNames(inner, add)
			}
			return
		}
		add(n.Name)
	case *ast.Call:
		add(n.Name)
		for _, a := range n.Args {
			exprNames(a, add)
		}
	case *ast.StringLit:
		interpWords(n.Value, add)
	case *ast.Unary:
		exprNames(n.X, add)
	case *ast.Binary:
		exprNames(n.L, add)
		exprNames(n.R, add)
	case *ast.List:
		for _, it := range n.Items {
			exprNames(it, add)
		}
	case *ast.Index:
		exprNames(n.X, add)
		exprNames(n.Index, add)
	case *ast.Member:
		exprNames(n.X, add)
	case *ast.Object:
		for _, p := range n.Pairs {
			exprNames(p.Value, add)
		}
	}
}

// stmtNames adds the names a statement itself mentions (not its body's).
// props receives a declaration's property name, which is a use at the
// start of its statement only as a transparent mixin call (use.Prop).
func stmtNames(s ast.Stmt, add func(string), props func(string)) {
	switch n := s.(type) {
	case *ast.RuleSet:
		for _, sel := range n.Selectors {
			interpWords(sel, add)
		}
	case *ast.Declaration:
		interpWords(n.Property, add)
		if !strings.Contains(n.Property, "{") {
			props(n.Property)
		}
		exprNames(n.Value, add)
	case *ast.Assignment:
		exprNames(n.Value, add)
	case *ast.MemberAssign:
		exprNames(n.Target, add)
		exprNames(n.Value, add)
	case *ast.FuncDef:
		for _, p := range n.Params {
			if p.Default != nil {
				exprNames(p.Default, add)
			}
		}
	case *ast.MixinCall:
		add(n.Name)
		for _, a := range n.Args {
			exprNames(a, add)
		}
	case *ast.If:
		for _, b := range n.Branches {
			exprNames(b.Cond, add)
		}
	case *ast.For:
		exprNames(n.Iterable, add)
	case *ast.Return:
		if n.Value != nil {
			exprNames(n.Value, add)
		}
	case *ast.ExprStmt:
		exprNames(n.X, add)
	case *ast.AtRule:
		// Params are raw text that may use variables (`@media (min-width:
		// tablet)`), so every word is a candidate; lookup filters out the
		// ones that name nothing.
		for _, w := range wordRE.FindAllString(n.Params, -1) {
			add(w)
		}
	}
}

// collectUses finds every use site in a parsed file. lines is the file's
// raw text split into lines.
//
// Each statement owns the lines from its own start to the line before the
// next statement's start (that covers comma-continued values). An `else`
// line isn't a statement start, so it is given to its If explicitly.
func collectUses(stmts []ast.Stmt, lines []string) []use {
	masked := strings.Split(codeMask(strings.Join(lines, "\n")), "\n")

	type lineInfo struct {
		names map[string]bool
		props map[string]bool
	}
	info := map[int]*lineInfo{}
	get := func(l int) *lineInfo {
		li := info[l]
		if li == nil {
			li = &lineInfo{names: map[string]bool{}, props: map[string]bool{}}
			info[l] = li
		}
		return li
	}

	// Pass 1: statement start lines, and each statement's own names.
	type owned struct {
		line  int
		names map[string]bool
		props map[string]bool
		extra []int // lines owned outside the start-to-next-start run
	}
	var all []owned
	var walk func(stmts []ast.Stmt)
	walk = func(stmts []ast.Stmt) {
		for _, s := range stmts {
			line, _ := ast.Pos(s)
			o := owned{line: line, names: map[string]bool{}, props: map[string]bool{}}
			stmtNames(s, func(n string) { o.names[n] = true }, func(p string) { o.props[p] = true })
			if n, ok := s.(*ast.If); ok {
				o.extra = elseLines(n, masked)
			}
			all = append(all, o)
			for _, b := range bodies(s) {
				walk(b)
			}
		}
	}
	walk(stmts)

	starts := map[int]bool{}
	for _, o := range all {
		starts[o.line] = true
	}
	for _, o := range all {
		if o.line < 1 {
			continue
		}
		owned := []int{o.line}
		for l := o.line + 1; l <= len(masked) && !starts[l]; l++ {
			owned = append(owned, l)
		}
		owned = append(owned, o.extra...)
		for _, l := range owned {
			li := get(l)
			for n := range o.names {
				li.names[n] = true
			}
			for p := range o.props {
				li.props[p] = true
			}
		}
	}

	// Definition sites, which are not uses.
	defSite := map[string]bool{}
	for _, d := range collectDefs(stmts, "", lines) {
		r := nameRange(lines, d.Line-1, d.Name)
		defSite[fmt.Sprintf("%d:%d", d.Line, runeCol(lines[min(d.Line-1, len(lines)-1)], r.Start.Character))] = true
	}

	// Pass 2: word occurrences on each line.
	var out []use
	for l := 1; l <= len(masked); l++ {
		li := info[l]
		if li == nil || len(li.names)+len(li.props) == 0 {
			continue
		}
		runes := []rune(masked[l-1])
		for i := 0; i < len(runes); {
			if !isWordRune(runes[i]) {
				i++
				continue
			}
			j := i
			for j < len(runes) && isWordRune(runes[j]) {
				j++
			}
			start := i
			// A leading '-' is a minus sign more often than part of a name
			// (wordAt makes the same call).
			for start < j && runes[start] == '-' {
				start++
			}
			word := string(runes[start:j])
			i = j
			if start == j {
				continue
			}
			before := strings.TrimRight(string(runes[:start]), " \t")
			segStart := before == "" || strings.HasSuffix(before, "{") || strings.HasSuffix(before, ";")
			prop := segStart && li.props[word]
			if !li.names[word] && !prop {
				continue
			}
			if start > 0 && strings.ContainsRune(".#@", runes[start-1]) {
				continue
			}
			if defSite[fmt.Sprintf("%d:%d", l, start)] {
				continue
			}
			call := (j < len(runes) && runes[j] == '(') || (start > 0 && runes[start-1] == '+')
			// `padding pad`: padding is the property (a use only if a
			// mixin of that name exists), pad a variable.
			out = append(out, use{Name: word, Line: l, Start: start, End: j, Call: call, Prop: prop})
		}
	}
	return out
}

// elseLines returns the lines of an If's `else` / `else if` headers: the
// lines in the If's extent, at its own indentation (or after a closing
// brace), that begin with `else`.
func elseLines(n *ast.If, masked []string) []int {
	if n.Line < 1 || n.Line > len(masked) {
		return nil
	}
	indent := leadingWS(masked[n.Line-1])
	last := lastLine(bodies(n)[0])
	for _, b := range bodies(n)[1:] {
		last = max(last, lastLine(b))
	}
	var out []int
	for l := n.Line + 1; l <= min(last, len(masked)); l++ {
		t := masked[l-1]
		rest := strings.TrimLeft(strings.TrimPrefix(strings.TrimLeft(t, " \t"), "}"), " \t")
		if strings.HasPrefix(rest, "else") && (leadingWS(t) == indent || strings.HasPrefix(strings.TrimLeft(t, " \t"), "}")) {
			out = append(out, l)
		}
	}
	return out
}

func leadingWS(s string) string { return s[:len(s)-len(strings.TrimLeft(s, " \t"))] }

// codeMask blanks comments, and the parts of quoted strings outside `{…}`
// interpolation, with spaces, keeping every rune at its column (newlines
// are kept). What remains is code a use can appear in.
func codeMask(src string) string {
	r := []rune(src)
	var quote rune
	depth := 0 // `{` depth inside the current string
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case quote != 0:
			switch {
			case c == '\n':
				quote = 0
			case depth > 0:
				if c == '}' {
					depth--
				}
			case c == '{':
				depth++
			case c == '\\' && i+1 < len(r) && r[i+1] != '\n':
				r[i], r[i+1] = ' ', ' '
				i++
			case c == quote:
				quote = 0
			default:
				r[i] = ' '
			}
		case c == '"' || c == '\'':
			quote, depth = c, 0
		case c == '/' && i+1 < len(r) && r[i+1] == '*':
			for ; i < len(r) && !(r[i] == '*' && i+1 < len(r) && r[i+1] == '/'); i++ {
				if r[i] != '\n' {
					r[i] = ' '
				}
			}
			if i+1 < len(r) {
				r[i], r[i+1] = ' ', ' '
				i++
			}
		case c == '/' && i+1 < len(r) && r[i+1] == '/' && (i == 0 || r[i-1] == ' ' || r[i-1] == '\t' || r[i-1] == '\n'):
			// Same rule as the parser: `//` after whitespace starts a
			// comment, so `url(http://x)` survives.
			for ; i < len(r) && r[i] != '\n'; i++ {
				r[i] = ' '
			}
		}
	}
	return string(r)
}

// --- symbol identity ---

// symKey identifies the variable or function a definition belongs to.
// Every root-scope definition of a name is the same symbol (reassigning a
// root variable, in any file of the compile, rebinds the one variable);
// a local is identified by its scope, so a parameter and a reassignment in
// the body are one symbol, and a same-named local elsewhere is another.
func symKey(d def) string {
	if d.root() {
		return fmt.Sprintf("root/%d/%s", d.Kind, d.Name)
	}
	return fmt.Sprintf("%s/%d-%d/%d/%s", d.File, d.FromLine, d.ToLine, d.Kind, d.Name)
}

// --- files in a compile ---

// fileIndex is one file's definitions and uses, parsed from its current
// text (the editor's, if open).
type fileIndex struct {
	path  string
	uri   string
	lines []string
	defs  []def
	uses  []use
}

// indexText parses a file and indexes it; nil when it doesn't parse.
func indexText(path, uri, text string) *fileIndex {
	sheet, err := parser.Parse(text)
	if err != nil {
		return nil
	}
	lines := strings.Split(text, "\n")
	return &fileIndex{
		path: path, uri: uri, lines: lines,
		defs: collectDefs(sheet.Statements, path, lines),
		uses: collectUses(sheet.Statements, lines),
	}
}

// importClosure lists the .styl files a sheet imports, transitively, in
// first-import order (the same resolution importedDefs uses).
func importClosure(sheet *ast.Stylesheet, path string, overlay func(string) (string, bool)) []string {
	if path == "" || sheet == nil {
		return nil
	}
	seen := map[string]bool{absPath(path): true}
	var out []string
	var visit func(stmts []ast.Stmt, dir string, depth int)
	visit = func(stmts []ast.Stmt, dir string, depth int) {
		if depth > maxImportDepth {
			return
		}
		for _, f := range resolvedImports(stmts, dir) {
			if seen[f] {
				continue
			}
			seen[f] = true
			out = append(out, f)
			text, ok := overlay(f)
			if !ok {
				data, err := os.ReadFile(f)
				if err != nil {
					continue
				}
				text = string(data)
			}
			if sub, err := parser.Parse(text); err == nil {
				visit(sub.Statements, filepath.Dir(f), depth+1)
			}
		}
	}
	visit(sheet.Statements, filepath.Dir(path), 0)
	return out
}

// --- references and rename ---

// loc is a found occurrence.
type loc struct {
	uri   string
	rng   Range
	isDef bool
}

// references finds every occurrence of the symbol def t is part of.
//
// Imported files share the importer's root scope (a partial may use a
// variable another partial defined without importing it), so a use can
// only be resolved in the context of a compile. Every open document is
// taken as the root of one:
//
//	root X ─ imports ─▶ G1, G2, …      (X's import closure)
//
// and when t's file is part of that compile, each file G in it is scanned,
// its uses resolved against G's own definitions first and then the root
// definitions of every other file in the compile, exactly as lookup does
// for go-to-definition. Results from different roots are merged.
func (s *Server) references(from *document, t def) []loc {
	key := symKey(t)
	open := func(p string) (string, bool) { return s.overlay(p) }
	cache := map[string]*fileIndex{}
	index := func(p, text string, uri string) *fileIndex {
		k := absPath(p)
		if p == "" {
			k = "untitled:" + uri
		}
		if fi, ok := cache[k]; ok {
			return fi
		}
		fi := indexText(p, uri, text)
		cache[k] = fi
		return fi
	}

	seen := map[string]bool{}
	var out []loc
	add := func(l loc) {
		k := fmt.Sprintf("%s:%d:%d", l.uri, l.rng.Start.Line, l.rng.Start.Character)
		if !seen[k] {
			seen[k] = true
			out = append(out, l)
		}
	}

	roots := []*document{from}
	for _, d := range s.docs {
		if d != from && d.path != "" {
			roots = append(roots, d)
		}
	}
	for _, root := range roots {
		rootFI := index(root.path, root.text, root.uri)
		if rootFI == nil {
			continue
		}
		sheet, _ := parser.Parse(root.text)
		files := []*fileIndex{rootFI}
		for _, p := range importClosure(sheet, root.path, open) {
			text, ok := s.overlay(p)
			if !ok {
				text = readFile(p)
			}
			if fi := index(p, text, pathToURI(p)); fi != nil {
				files = append(files, fi)
			}
		}
		inCompile := false
		for _, fi := range files {
			if samePath(fi.path, t.File) || fi.path == t.File {
				inCompile = true
			}
		}
		if !inCompile {
			continue
		}
		for _, g := range files {
			// G's own definitions, then every other file's root ones.
			an := &analysis{defs: append([]def(nil), g.defs...)}
			for _, o := range files {
				if o == g {
					continue
				}
				for _, d := range o.defs {
					if d.root() {
						an.defs = append(an.defs, d)
					}
				}
			}
			for _, d := range g.defs {
				if symKey(d) == key {
					add(loc{uri: g.uri, rng: nameRange(g.lines, d.Line-1, d.Name), isDef: true})
				}
			}
			for _, u := range g.uses {
				if d, ok := resolveUse(an, g.path, u); ok && symKey(d) == key {
					line := g.lines[u.Line-1]
					add(loc{uri: g.uri, rng: Range{
						Start: Position{Line: u.Line - 1, Character: utf16Col(line, u.Start)},
						End:   Position{Line: u.Line - 1, Character: utf16Col(line, u.End)},
					}})
				}
			}
		}
	}
	return out
}

func (s *Server) referencesReq(d *document, pos Position, includeDecl bool) (any, *rpcError) {
	t, _, ok := d.resolve(pos)
	if !ok {
		return nil, nil
	}
	out := []Location{}
	for _, l := range s.references(d, t) {
		if l.isDef && !includeDecl {
			continue
		}
		out = append(out, Location{URI: l.uri, Range: l.rng})
	}
	return out, nil
}

// identRE is a valid new name for rename: what the lexer reads as one
// identifier (not starting with a digit or a minus sign followed by one).
var identRE = regexp.MustCompile(`^(?:[\p{L}_$]|-[\p{L}_$-])[\p{L}\p{N}_$-]*$`)

// prepareRename reports the range of the name at pos when it resolves to a
// definition in the project (built-ins and CSS keywords can't be renamed).
func (s *Server) prepareRename(d *document, pos Position) (any, *rpcError) {
	if _, _, ok := d.resolve(pos); !ok {
		return nil, nil
	}
	_, start, end, _ := d.wordAt(pos)
	line := d.lines[pos.Line]
	return Range{
		Start: Position{Line: pos.Line, Character: utf16Col(line, start)},
		End:   Position{Line: pos.Line, Character: utf16Col(line, end)},
	}, nil
}

// rename rewrites every occurrence found by references, definitions
// included, across files.
func (s *Server) rename(d *document, pos Position, newName string) (any, *rpcError) {
	if !identRE.MatchString(newName) {
		return nil, &rpcError{Code: codeRequestFailed, Message: fmt.Sprintf("%q is not a valid Stylus name", newName)}
	}
	t, _, ok := d.resolve(pos)
	if !ok {
		return nil, &rpcError{Code: codeRequestFailed, Message: "nothing to rename here"}
	}
	changes := map[string][]TextEdit{}
	for _, l := range s.references(d, t) {
		changes[l.uri] = append(changes[l.uri], TextEdit{Range: l.rng, NewText: newName})
	}
	return WorkspaceEdit{Changes: changes}, nil
}
