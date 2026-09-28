package eval

import (
	"sort"

	"github.com/rohanthewiz/go-styl/internal/builtin"
)

// Entry points for editor tooling (the language server): the evaluator's
// view of names and import resolution, exported so the tooling agrees with
// what a compile does instead of keeping its own copy of the rules.

// BuiltinNames returns every built-in function name, both the pure value
// built-ins (builtin.Registry) and the evaluator-aware ones (ctxBuiltins:
// define, lookup, selector, warn, …), sorted and duplicate-free.
func BuiltinNames() []string {
	seen := map[string]bool{}
	for name := range builtin.Registry {
		seen[name] = true
	}
	for name := range ctxBuiltins {
		seen[name] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ResolveImport resolves a .styl import path on the OS filesystem exactly as
// a compile would: relative to dir, then each include path, trying the path
// itself, path.styl and path/index.styl, and expanding globs. It returns
// absolute paths.
func ResolveImport(dir, imp string, includePaths []string) ([]string, error) {
	return resolveImport(nil, dir, imp, includePaths)
}
