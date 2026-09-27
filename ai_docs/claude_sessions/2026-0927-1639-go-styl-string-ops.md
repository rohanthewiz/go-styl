# Session: string concatenation and length() (N-036) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Implemented next-list item N-036:

- `length("abc")` was 1 (the string treated as a one-item list). Stylus
  gives 3.
- `"x" + "y"` failed with `cannot apply "+" to string and string`. Stylus
  gives `'xy'`.

Same session as N-027…N-030, N-022 and N-035 (`2026-0927-1625` … `-1637`).

## Stylus behavior (stylus 0.64 source and probes)

- `functions/length.js`: a single string argument returns `val.length`,
  otherwise the number of nodes. So `length(("abc" "de"))` is 2 and
  `length(foo)` is 1.
- `nodes/string.js`: `+` returns `new String(this.val + coerce(right).val)`.
  `coerce` keeps a string's value, space-joins an expression's nodes, and
  otherwise uses `toString()`. Results:
  - `"x" + 1px` → `'x1px'`, `"x" + #fff` → `'x#fff'`
  - `"x" + (1 2)` → `'x1 2'`, `"w" + null` → `'wnull'`
- The result is always single-quoted, whatever the operands used.
- A Literal on the left (from `unquote()`, `s()` or `%`) stays unquoted:
  `unquote("a") + "b"` → `ab`, `s("%s", 1) + "px"` → `1px`.
- `"x" * 2` is an error in Stylus too, so nothing else needed.

## Implementation

- `internal/builtin/list.go` (`length`): a `*value.Str` returns
  `utf8.RuneCountInString(Val)`. Runes, not UTF-16 units; they agree on
  `"héllo"` (5), checked against stylus.
- `internal/eval/eval.go` (`evalBinary`): if the op is `+` and the left
  side is a `*value.Str`, it returns a new `Str`. The quote is `'`, or 0
  when the left side is unquoted, which is go-styl's form of Stylus's
  Literal. It comes before number/color arithmetic.
- New helper `concatText` mirrors `String#coerce`: a string's raw value,
  lists (space or comma) joined with spaces, null → "null", anything else
  its CSS form.

## Tests

- `string_ops_test.go`: 17 cases (length of strings, empty strings,
  non-ASCII, lists, idents; concatenation with strings, mixed quotes,
  idents, units, colors, lists, null, chaining, unquoted left, `s()`
  result; number `+` unchanged). All match stylus 0.64.
- `go test ./...` passes.

## Next

Closed: N-036. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
