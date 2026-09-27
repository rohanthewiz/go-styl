# Session: @require and glob import paths (N-023) (2026-09-27)

Session ID: `b81ec384-8b11-4927-b3cb-0820befea8e2`

Implemented next-list item N-023: `@require` (import once per compile) and
glob import paths (`@require '_styl/*'`). cema uses `@require` 44 times, so
this was one of the blockers for N-022 (compile cema with go-styl). Commit
`aed9f45`.

## What changed

- **AST** (`internal/ast/ast.go`): `ast.Import` gained `Once bool`, true for
  `@require`.
- **Parser** (`internal/parser/parser.go`): `@require` is parsed by
  `parseImport` like `@import`. Before, it went through as a verbatim leaf
  at-rule. `parseImport` now takes the keyword, which it uses in error
  messages and to set `Once`.
- **Evaluator** (`internal/eval/eval.go`, `internal/eval/m4.go`):
  - A new `required map[string]bool` sits beside `importing`.
  - `evalImport` resolves to a list of files and checks each one against
    `required` when `Once` is set. The per-file part (cycle check, deps,
    parse, run inline) moved into a new `importFile`.
  - A literal `@require` (`.css`, `url()`) is deduplicated by its raw path
    text. The key has a NUL prefix so it can't collide with a resolved file
    path.
- **Resolver**: `resolveImport` and `resolveImportFS` return `[]string`.
  - A path with `*?[` is globbed with `filepath.Glob` or `fs.Glob`.
  - `.styl` is appended to the pattern unless it already ends in `.styl`,
    which is Stylus's rule, so `.css` files beside the partials are skipped.
  - Matches come back in sorted order, from the first search directory that
    has any. Directories are skipped. No match means "file not found".
  - Plain paths resolve as before, now as a one-item list.

## Semantics (checked against stylus 0.64)

The rules follow Stylus's evaluator `requireHistory`:

- Only `@require` reads and writes the required set. `@import` always imports
  again.
- `@import x` then `@require x` imports twice. A second `@require x` after
  that is skipped.
- Dedup is per resolved file, so `@require '_styl/_b'` then
  `@require '_styl/*'` outputs `.b` and then `.a` only.
- Glob results are in sorted order.

I ran every case through both `stylus -c -p` and `go run ./cmd/styl`, and the
output matched.

Known differences:

- **Literal `.css` requires:** `@require 'x.css'` passes through as
  `@import "x.css";`, while Stylus errors when the file is missing. go-styl
  already treated literal `@import` this way.
- **`**` in globs:** Go's glob has no `**`, so it acts like `*`. This is noted
  in the resolver's doc comment.

## Tests

- `require_test.go` (package `styl`):
  - A table of cases over an `fstest.MapFS`: require twice, a nested require,
    import not deduplicated, require after import, glob order, glob with an
    already-required file, a literal require, brace syntax with `;`.
  - `TestRequireGlobNoMatch`.
  - `TestRequireGlobOS`: an on-disk fixture in `testdata/require/`, checked
    with `BuildFile`. Deps are main + `_a` + `_b`; the `c.css` beside them is
    ignored.
- `go test ./...` passes, difftest included.

## Notes

- An untracked `.cats-todo/` folder appeared in the repo during the session.
  It isn't from this work, so it was left out of commits.

## Next

Closed: N-023. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: N-022 (blockers now N-024…N-030). Full list: `ai_docs/todo/next-list.md`.
