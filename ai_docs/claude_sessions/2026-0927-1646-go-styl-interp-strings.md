# Session: interpolated strings lose their quotes (N-038) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Fixed next-list item N-038, raised in the previous item (N-001, url
variables). With `s = "x"`, `.a-{s}` compiled to `.a-"x"`. Stylus gives
`.a-x`. The same bug made `url({base}x.png)` into `url("/img/"x.png)`, which
is invalid CSS.

Same session as N-027…N-030, N-022, N-035…N-037, N-008 and N-001
(`2026-0927-1625` … `-1645`).

## Stylus behavior

`Evaluator#interpolate` (`lib/visitor/evaluator.js`) turns each segment
into text with `toString(node)`:

- idents and functions → name
- strings and literals → `node.val` (raw, no quotes)
- units → `node.val` (the **number without its unit**, except `%`)
- expressions → `toString(first node)` (**only the first item** of a list)

Probed: `.a-{s}` → `.a-x`; `{p}-top` with `p = "margin"` → `margin-top`;
`.b-{n}, .c-{s}` → `.b-foo,.c-x`.

## Implementation

`internal/eval/m4.go` (`evalString`, the bridge for all `{…}`
interpolation): after the custom-property deref, a `*value.Str` returns
its `Val`. Everything else keeps its CSS form as before.

I kept the other two Stylus rules out on purpose. go-styl's interpolation
also works in `@media` preludes, `calc()`, `url()` and quoted strings (all
go-styl extensions), and there `{bp}` must stay `768px`, not `768`.
Stylus's unit-dropping only matters in selectors like `.col-{w}` with a
unit value, which nobody has hit. It's noted in the code comment.

## Tests

`interp_string_test.go`: selector, property name, selector group, url
path, inside a quoted string (`"a {s} b"` → `"a x b"`), and a units-kept
`@media` guard. `go test ./...` passes, difftest unchanged. The
`testdata/m4.styl` quoted-string extension stays a known diff.

## Next

Closed: N-038. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
