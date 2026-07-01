package eval

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/css"
	"github.com/rohanthewiz/go-styl/internal/diag"
	"github.com/rohanthewiz/go-styl/internal/parser"
)

// interpolate resolves `{expr}` interpolation in a raw string (selector, property
// name, or string literal), substituting each group's evaluated CSS form. Strings
// without a `{` are returned unchanged. Nested braces are balanced; an unbalanced
// `{` is left verbatim.
func (ev *evaluator) interpolate(s string, scope *Scope) (string, error) {
	if !strings.Contains(s, "{") {
		return s, nil
	}
	runes := []rune(s)
	var b strings.Builder
	for i := 0; i < len(runes); i++ {
		if runes[i] != '{' {
			b.WriteRune(runes[i])
			continue
		}
		end := matchBrace(runes, i)
		if end < 0 {
			b.WriteString(string(runes[i:])) // unterminated: keep literally
			break
		}
		out, err := ev.evalString(string(runes[i+1:end]), scope)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
		i = end
	}
	return b.String(), nil
}

// evalString parses and evaluates a single expression from raw source, returning
// its CSS form. It is the bridge used to resolve interpolation contents.
func (ev *evaluator) evalString(src string, scope *Scope) (string, error) {
	e, err := parser.ParseExpr(strings.TrimSpace(src), 0)
	if err != nil {
		return "", err
	}
	v, err := ev.evalExpr(e, scope)
	if err != nil {
		return "", err
	}
	return v.CSS(ev.opts.Pretty), nil
}

// matchBrace returns the index of the '}' matching the '{' at open, or -1 if the
// group is unterminated.
func matchBrace(runes []rune, open int) int {
	depth := 0
	for i := open; i < len(runes); i++ {
		switch runes[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// wholeInterp reports whether name is exactly a single `{expr}` group spanning the
// whole string, returning the inner expression text.
func wholeInterp(name string) (string, bool) {
	runes := []rune(name)
	if len(runes) < 2 || runes[0] != '{' {
		return "", false
	}
	if matchBrace(runes, 0) == len(runes)-1 {
		return string(runes[1 : len(runes)-1]), true
	}
	return "", false
}

// allPlaceholders reports whether every selector is a `$placeholder` (and there is
// at least one), so the rule should be suppressed unless extended.
func allPlaceholders(sels []string) bool {
	if len(sels) == 0 {
		return false
	}
	for _, s := range sels {
		if !strings.HasPrefix(s, "$") {
			return false
		}
	}
	return true
}

// evalExtend records an @extend request: the current rule's selectors are queued
// to be grafted onto every rule matching the (interpolated) target.
func (ev *evaluator) evalExtend(s *ast.Extend, ctx *execCtx) error {
	if ctx.rule == nil {
		return fmt.Errorf("@extend %q must appear inside a selector", s.Target)
	}
	target, err := ev.interpolate(s.Target, ctx.scope)
	if err != nil {
		return err
	}
	extenders := make([]string, len(ctx.parents))
	copy(extenders, ctx.parents)
	ev.extends = append(ev.extends, extendReq{extenders: extenders, target: target})
	return nil
}

// evalImport handles `@import`. A literal import is emitted verbatim; otherwise
// the referenced .styl file is resolved, parsed, and executed inline in the
// current scope (so its variables and mixins are shared).
func (ev *evaluator) evalImport(s *ast.Import, ctx *execCtx) error {
	if s.Literal {
		*ctx.sink = append(*ctx.sink, &css.RawNode{Text: importStmt(s.Path)})
		return nil
	}

	abs, err := resolveImport(ev.opts.FS, ctx.dir, s.Path, ev.opts.IncludePaths)
	if err != nil {
		return err
	}
	if ev.importing[abs] {
		return fmt.Errorf("import cycle detected at %q", abs)
	}
	ev.deps = append(ev.deps, abs)

	data, err := ev.readFile(abs)
	if err != nil {
		return fmt.Errorf("@import %q: %w", s.Path, err)
	}
	sheet, err := parser.Parse(string(data))
	if err != nil {
		return diag.SetFile(err, abs)
	}

	ev.importing[abs] = true
	defer delete(ev.importing, abs)

	importCtx := *ctx
	importCtx.dir = dirOf(ev.opts.FS, abs)
	importCtx.file = abs
	return ev.execBlock(sheet.Statements, &importCtx)
}

// importStmt renders a passthrough @import line for literal imports.
func importStmt(path string) string {
	if strings.HasPrefix(strings.ToLower(path), "url(") {
		return "@import " + path + ";"
	}
	return `@import "` + path + `";`
}

// readFile reads a resolved import, from the configured fs.FS when set,
// otherwise from the OS filesystem.
func (ev *evaluator) readFile(name string) ([]byte, error) {
	if ev.opts.FS != nil {
		return fs.ReadFile(ev.opts.FS, name)
	}
	return os.ReadFile(name)
}

// dirOf returns the directory of a resolved import path, slash-separated in
// fs.FS mode and OS-separated otherwise.
func dirOf(fsys fs.FS, p string) string {
	if fsys != nil {
		return path.Dir(p)
	}
	return filepath.Dir(p)
}

// resolveImport locates a .styl import. It searches dir first, then each
// include path, trying the path as given and with a ".styl" extension (and an
// index.styl inside a matching directory). With a non-nil fsys, resolution uses
// slash-separated fs.FS paths (a leading '/' is treated as the FS root);
// otherwise the OS filesystem, returning an absolute path.
func resolveImport(fsys fs.FS, dir, imp string, includePaths []string) (string, error) {
	if fsys != nil {
		return resolveImportFS(fsys, dir, imp, includePaths)
	}

	var bases []string
	if filepath.IsAbs(imp) {
		bases = []string{""}
	} else {
		bases = append(bases, dir)
		bases = append(bases, includePaths...)
	}

	for _, base := range bases {
		cand := imp
		if base != "" {
			cand = filepath.Join(base, imp)
		}
		for _, p := range []string{cand, cand + ".styl", filepath.Join(cand, "index.styl")} {
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				abs, err := filepath.Abs(p)
				if err != nil {
					return "", err
				}
				return abs, nil
			}
		}
	}
	return "", fmt.Errorf("@import %q: file not found", imp)
}

// resolveImportFS is resolveImport over an fs.FS.
func resolveImportFS(fsys fs.FS, dir, imp string, includePaths []string) (string, error) {
	var bases []string
	if strings.HasPrefix(imp, "/") {
		imp = strings.TrimPrefix(imp, "/")
		bases = []string{"."}
	} else {
		if dir == "" {
			dir = "."
		}
		bases = append(bases, dir)
		bases = append(bases, includePaths...)
	}

	for _, base := range bases {
		cand := path.Join(base, imp)
		for _, p := range []string{cand, cand + ".styl", path.Join(cand, "index.styl")} {
			if !fs.ValidPath(p) {
				continue
			}
			if info, err := fs.Stat(fsys, p); err == nil && !info.IsDir() {
				return p, nil
			}
		}
	}
	return "", fmt.Errorf("@import %q: file not found", imp)
}
