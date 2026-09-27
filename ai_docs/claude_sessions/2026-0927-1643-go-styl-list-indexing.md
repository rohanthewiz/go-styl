# Session: list indexing (N-008) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Implemented next-list item N-008: `r[1]` failed with
`unexpected "[" in expression`. Stylus gives the second element.

Same session as N-027…N-030, N-022, N-035…N-037 (`2026-0927-1625` …
`-1640`).

## Stylus behavior (probed against stylus 0.64)

With `r = 1px 2px 3px`, `lst = a, b, x`, `s = 5px`:

| Expression | Result |
|---|---|
| `r[1]` | `2px` |
| `r[-1]` | `3px` (negative counts from the end) |
| `r[5]`, `s[1]` | null (prints as nothing) |
| `lst[2]` | `x` (comma lists index the same way) |
| `s[0]` | `5px` (a scalar is a one-item list) |
| `r[i + 1]` | `3px` (any expression as the index) |
| `r[0] + 1` | `2px` |
| `(1 2 3)[1]`, `(r)[1]` | `2`, `2px` |
| `r[0 1]`, `r[0..1]` | `1px 2px` (several indexes → space list) |

Stylus's `unshift()` returns the new length rather than the list, so
`unshift(r, 0)[0]` is an error there. A first-draft test relied on
go-styl's list-returning `unshift` and was replaced.

## Implementation

- `internal/ast/ast.go`: new `Index{X, Index Expr}`.
- `internal/parser/expr.go`: `parsePrimary` now parses an operand
  (`parseOperand`, the old body) and then any number of subscripts. A
  subscript needs the `[` glued to the operand (`!SpaceBefore`), like the
  existing call rule for `(`. With a space (`1fr [main-start]`) it is not a
  subscript. The index is a full `parseValue` at depth+1.
- `internal/eval/eval.go`: new `evalIndex`. It derefs custom-property
  wrappers, treats a non-list as a one-item list (null as empty), and
  handles a number index (negative wraps once, out of range → `Null`) or a
  list index (each picked, one result unwrapped, several → space list). A
  non-number index is an error.
- Only the parser and evaluator walk expression nodes (checked by grep), so
  no other visitor needed the new node.

Not supported: hash/object members (`obj[key]`, `obj.key`) and subscript
assignment (`r[1] = x`). go-styl has no hash type. Nobody has asked for
them, so no item was raised.

Grid line names (`grid-template-columns [a] 1fr`) fail in both go-styl
(before and after this change) and Stylus. Unchanged.

## Tests

- `index_test.go`: `TestListIndexing` (13 cases above) and
  `TestListIndexingInControlFlow` (a function body `l[1]`, `if r[0] == a`,
  `for v in r[1..2]`). All match stylus 0.64.
- `go test ./...` passes; `FuzzCompile` ran 15s clean.

## Next

Closed: N-008. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
