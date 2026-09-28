package lsp

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/ast"
)

// lintSource tags lint diagnostics, so clients can tell them from compile
// errors.
const lintSource = "styl-lint"

// lint checks a parsed document for likely mistakes the compile accepts:
//
//   - an unused local variable (an assignment inside a function, mixin or
//     selector block that nothing reads);
//   - an unused function or mixin: a local one, or a root one in an entry
//     sheet (see below);
//   - a property set twice in one block.
//
// What it leaves alone, on purpose:
//
//   - Root variables. They are the sheet's public surface: `styl gen` turns
//     them into Go constants, CustomProperties exposes them as CSS custom
//     properties, and later partials read them from the shared root scope.
//   - Parameters and loop variables. A mixin keeps a parameter for its
//     callers' sake, and `for i, x in list` often reads only one of the two.
//   - Root functions in a partial (a `_name.styl` file, or one another open
//     document imports). A partial is a library: its users live elsewhere.
//     An entry sheet's root functions are checked against every file of its
//     compile, since an imported partial may call a mixin the entry defines.
//   - Variables in a sheet that calls lookup() or define(), which read and
//     bind variables by computed name.
//
// Unused definitions are hints tagged Unnecessary (clients fade them);
// duplicate properties are warnings.
func lint(an *analysis, path string, lines []string, imported bool, closure func() []*fileIndex) []Diagnostic {
	if an.sheet == nil {
		return nil
	}
	var out []Diagnostic
	out = append(out, duplicateProps(an.sheet.Statements, lines)...)

	// Resolve every use in the document to its symbol.
	used := map[string]bool{}
	dynamic := false
	for _, u := range an.uses {
		if u.Call && (u.Name == "lookup" || u.Name == "define") {
			dynamic = true
		}
		if d, ok := resolveUse(an, path, u); ok {
			used[symKey(d)] = true
		}
	}

	entry := !imported && path != "" && !strings.HasPrefix(filepath.Base(path), "_")
	var otherUses map[string]bool // names called in the rest of the compile, built on demand
	calledElsewhere := func(name string) bool {
		if otherUses == nil {
			otherUses = map[string]bool{}
			for _, fi := range closure() {
				own := map[string]bool{}
				for _, d := range fi.defs {
					if d.Kind == defFunc {
						own[d.Name] = true
					}
				}
				for _, u := range fi.uses {
					// A partial's call of a name it defines itself is
					// its own function, not the entry's; a property is a
					// call only as a transparent mixin, which can't be
					// told from a plain property without the entry's defs,
					// so it counts.
					if !own[u.Name] {
						otherUses[u.Name] = true
					}
				}
			}
		}
		return otherUses[name]
	}

	reported := map[string]bool{}
	for _, d := range an.defs {
		if d.File != path || used[symKey(d)] || reported[symKey(d)] {
			continue
		}
		var what string
		switch {
		case d.Kind == defVar && d.Role == roleAssign && !d.root() && !dynamic:
			what = "variable"
		case d.Kind == defFunc && !d.root():
			what = "function"
		case d.Kind == defFunc && entry && !calledElsewhere(d.Name):
			what = "function"
		default:
			continue
		}
		reported[symKey(d)] = true
		out = append(out, Diagnostic{
			Range:    nameRange(lines, d.Line-1, d.Name),
			Severity: SeverityHint,
			Source:   lintSource,
			Message:  fmt.Sprintf("%s %s is never used", what, d.Name),
			Tags:     []int{tagUnnecessary},
		})
	}
	return out
}

// duplicateProps reports a property declared again in the same block.
//
// The fallback idiom is allowed: consecutive declarations of one property
// with different values (`display -webkit-box` then `display flex`) let
// older browsers keep the first. A repeat anywhere else, or with the same
// value, is flagged on the later declaration: it overrides the earlier one
// (or repeats it), which is usually an edit left behind. Interpolated
// property names are skipped, as are declarations in if/for bodies, which
// are separate blocks here: whether they run is decided at compile time.
func duplicateProps(stmts []ast.Stmt, lines []string) []Diagnostic {
	var out []Diagnostic
	seen := map[string]*ast.Declaration{}
	var prev ast.Stmt
	for _, s := range stmts {
		if decl, ok := s.(*ast.Declaration); ok && !strings.Contains(decl.Property, "{") {
			key := strings.ToLower(decl.Property)
			if first, dup := seen[key]; dup {
				p, consecutive := prev.(*ast.Declaration)
				fallback := consecutive && strings.EqualFold(p.Property, decl.Property) &&
					!(reflect.DeepEqual(p.Value, decl.Value) && p.Important == decl.Important)
				if !fallback {
					out = append(out, Diagnostic{
						Range:    nameRange(lines, decl.Line-1, decl.Property),
						Severity: SeverityWarning,
						Source:   lintSource,
						Message:  fmt.Sprintf("duplicate property %s (also set on line %d)", decl.Property, first.Line),
					})
				}
			} else {
				seen[key] = decl
			}
		}
		prev = s
		for _, b := range bodies(s) {
			out = append(out, duplicateProps(b, lines)...)
		}
	}
	return out
}
