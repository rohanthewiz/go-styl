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
	"github.com/rohanthewiz/go-styl/internal/value"
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

// interpolateString resolves `{expr}` inside a quoted string literal, a
// go-styl extension (Stylus keeps string contents literal). To keep the
// extension from changing strings that were never meant to be templates, a
// group is substituted only when it parses as an expression that references
// a variable in scope:
//
//	p = col
//	"x-{p}"     → "x-col"     (p is a variable)
//	"{nope}"    → "{nope}"    (no variable: literal, as in Stylus)
//	"{1 + 2}"   → "{1 + 2}"   (no variable: literal)
//	"\{p}"      → "\{p}"      (escaped: literal, as in Stylus)
//
// Anything left literal is copied byte for byte, backslash included, so the
// output matches Stylus's.
func (ev *evaluator) interpolateString(s string, scope *Scope) (string, error) {
	if !strings.Contains(s, "{") {
		return s, nil
	}
	runes := []rune(s)
	var b strings.Builder
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if c == '\\' && i+1 < len(runes) {
			// An escape (N-026 keeps these verbatim): copy it and the rune
			// after it, so `\{` never opens a group.
			b.WriteRune(c)
			b.WriteRune(runes[i+1])
			i++
			continue
		}
		if c != '{' {
			b.WriteRune(c)
			continue
		}
		end := matchBrace(runes, i)
		if end < 0 {
			b.WriteString(string(runes[i:])) // unterminated: keep literally
			break
		}
		src := strings.TrimSpace(string(runes[i+1 : end]))
		e, err := parser.ParseExpr(src, 0)
		if err != nil || !refsVariable(e, scope) {
			b.WriteString(string(runes[i : end+1]))
			i = end
			continue
		}
		out, err := ev.evalString(src, scope)
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
	// Interpolation asks for the value's text; a var(--name) reference would
	// be invalid in selectors and media queries, so custom-property wrappers
	// resolve to their compile-time value.
	v = value.Deref(v)
	// A string contributes its raw text, without quotes, as in Stylus's
	// interpolate(): with s = "x", `.a-{s}` is `.a-x`, not `.a-"x"`.
	// (Stylus also drops units and keeps only a list's first item there;
	// go-styl keeps both, since its @media/calc interpolation needs units.)
	if s, ok := v.(*value.Str); ok {
		return s.Val, nil
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

// evalImport handles `@import` and `@require`. A literal import is emitted
// verbatim; otherwise the referenced .styl file(s) are resolved, parsed, and
// executed inline in the current scope (so their variables and mixins are
// shared). A glob path expands to every matching file, imported in sorted
// order.
//
// @require (s.Once) follows Stylus: each resolved file is imported at most
// once per compile, and the check is made per file, so `@require '_styl/*'`
// after `@require '_styl/_vars'` skips just _vars. Only @require consults and
// records the set; a plain @import always re-imports, matching Stylus's
// requireHistory.
func (ev *evaluator) evalImport(s *ast.Import, ctx *execCtx) error {
	if s.Literal {
		// Literal requires are deduped by their raw path text. The NUL prefix
		// keeps these keys apart from resolved file paths in the same set.
		if s.Once {
			key := "\x00literal:" + s.Path
			if ev.required[key] {
				return nil
			}
			ev.required[key] = true
		}
		*ctx.sink = append(*ctx.sink, &css.RawNode{Text: importStmt(s.Path)})
		return nil
	}

	files, err := resolveImport(ev.opts.FS, ctx.dir, s.Path, ev.opts.IncludePaths)
	if err != nil {
		return err
	}
	for _, abs := range files {
		if s.Once {
			if ev.required[abs] {
				continue
			}
			ev.required[abs] = true
		}
		if err := ev.importFile(s.Path, abs, ctx); err != nil {
			return err
		}
	}
	return nil
}

// importFile parses one resolved import (abs) and executes it inline in ctx's
// scope. imp is the path as written, for error messages.
func (ev *evaluator) importFile(imp, abs string, ctx *execCtx) error {
	if ev.importing[abs] {
		return fmt.Errorf("import cycle detected at %q", abs)
	}
	ev.deps = append(ev.deps, abs)

	data, err := ev.readFile(abs)
	if err != nil {
		return fmt.Errorf("@import %q: %w", imp, err)
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

// isGlob reports whether an import path contains glob metacharacters.
func isGlob(imp string) bool {
	return strings.ContainsAny(imp, "*?[")
}

// globPattern appends ".styl" to a glob that doesn't already name that
// extension, as Stylus does, so `_styl/*` matches only the .styl partials in
// the directory and skips any .css or other files beside them.
func globPattern(imp string) string {
	if strings.HasSuffix(strings.ToLower(imp), ".styl") {
		return imp
	}
	return imp + ".styl"
}

// resolveImport locates the file(s) for a .styl import. It searches dir first,
// then each include path. A plain path resolves to one file: the path as
// given, with a ".styl" extension, or index.styl inside a matching directory.
// A glob path (`_styl/*`) resolves to every matching .styl file in the first
// base that has any match, in sorted order. Go's glob syntax applies, so `**`
// behaves like `*` (no recursive descent). With a non-nil fsys, resolution
// uses slash-separated fs.FS paths (a leading '/' is treated as the FS root);
// otherwise the OS filesystem, returning absolute paths.
func resolveImport(fsys fs.FS, dir, imp string, includePaths []string) ([]string, error) {
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

		if isGlob(imp) {
			// filepath.Glob returns matches in lexical order, so imports run
			// in a stable, name-sorted order (_a before _b), as in Stylus.
			matches, err := filepath.Glob(globPattern(cand))
			if err != nil {
				return nil, fmt.Errorf("@import %q: %w", imp, err)
			}
			var files []string
			for _, m := range matches {
				if info, err := os.Stat(m); err != nil || info.IsDir() {
					continue
				}
				abs, err := filepath.Abs(m)
				if err != nil {
					return nil, err
				}
				files = append(files, abs)
			}
			if len(files) > 0 {
				return files, nil
			}
			continue
		}

		for _, p := range []string{cand, cand + ".styl", filepath.Join(cand, "index.styl")} {
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				abs, err := filepath.Abs(p)
				if err != nil {
					return nil, err
				}
				return []string{abs}, nil
			}
		}
	}
	return nil, fmt.Errorf("@import %q: file not found", imp)
}

// resolveImportFS is resolveImport over an fs.FS.
func resolveImportFS(fsys fs.FS, dir, imp string, includePaths []string) ([]string, error) {
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

		if isGlob(imp) {
			// fs.Glob, like filepath.Glob, yields lexically sorted matches.
			matches, err := fs.Glob(fsys, globPattern(cand))
			if err != nil {
				return nil, fmt.Errorf("@import %q: %w", imp, err)
			}
			var files []string
			for _, m := range matches {
				if info, err := fs.Stat(fsys, m); err == nil && !info.IsDir() {
					files = append(files, m)
				}
			}
			if len(files) > 0 {
				return files, nil
			}
			continue
		}

		for _, p := range []string{cand, cand + ".styl", path.Join(cand, "index.styl")} {
			if !fs.ValidPath(p) {
				continue
			}
			if info, err := fs.Stat(fsys, p); err == nil && !info.IsDir() {
				return []string{p}, nil
			}
		}
	}
	return nil, fmt.Errorf("@import %q: file not found", imp)
}
