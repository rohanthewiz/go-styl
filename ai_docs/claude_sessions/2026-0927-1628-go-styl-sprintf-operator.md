# Session: sprintf operator (N-028) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Implemented next-list item N-028: Stylus's string `%` operator
(`"calc(100vh - %s)" % x`) failed with `cannot apply "%" to string and
unit`. cema uses it 5 times in `_mid_layout.styl`, so it blocked N-022.

This is the same session as `2026-0927-1625-go-styl-semicolons` (N-027).
The user asked for as many next-list items as possible without their input,
with a sess-wrap between items.

## Stylus behavior (probed against stylus 0.64)

In `nodes/string.js`, a string `%` calls the `s()` function: `s(fmt, args…)`
(`functions/s.js`).

- The right side is unwrapped: a list (space or comma, literal or in a
  variable) spreads into one argument per item. `"%s|%s" % z` with
  `z = 1px, 2px` gives `1px|2px`.
- `%s` prints the argument's compiled form. Strings keep their quotes
  (`"%s" % "str"` gives `"str"`).
- `%d` prints a number's bare value (`3px` gives `3`). A non-number is an
  error ("%d requires a unit").
- A missing argument is `null`, which prints as nothing.
- `%%` is not an escape: it stays `%%`.
- `%` binds like `*`: `"%s" % 1px 3px` gives `1px 3px`.
- The result is a literal (unquoted), and `type()` reports `'literal'`.

## Implementation

- `internal/eval/eval.go` (`evalBinary`): if the op is `%` and the left side
  is a `*value.Str`, it builds `[fmt, items…]` (spreading a `*value.List`)
  and calls the `s` builtin through `builtin.Lookup`. Number `%` (modulo) is
  unchanged. The parser already parsed this, at multiplicative binding power.
- `internal/builtin/string.go` (`sprintf`): added `%d`. It errors on a
  non-number, as Stylus does. The doc comment records the missing-argument
  and `%%` behavior.

Known small differences, not worth a type:

- `type("%s" % x)` is `string`, where Stylus says `literal`. go-styl has no
  literal type, and `s()` already returns an unquoted `Str`.
- `"%s %s" % x` prints `50px ;` (the space before the empty placeholder
  stays). Stylus prints `50px`.

## Tests

- `sprintf_test.go`: 16 cases (calc, expression argument, list and
  list-variable spreading, comma list, quoted strings, `%d`, `%%`, color,
  binding power, null, inside a mixin, number modulo). All match stylus 0.64.
- `go test ./...` passes.

cema's `master.styl` still stops at the column-0 `.form-help {` quirk in
`_material_form.styl` (N-022's cema-side list), so the real `_mid_layout`
lines get checked in the N-022 re-run.

## Next

Closed: N-028. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: N-022. Full list: `ai_docs/todo/next-list.md`.
