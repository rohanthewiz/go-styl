# Session: number formatting (N-029) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Implemented next-list item N-029: arithmetic results printed their binary
floating-point noise (`mf-font-size * 1.8` gave `1.7999999999999998rem`,
where Stylus gives `1.8rem`). It renders the same, but it showed up in every
N-022 diff against cema's output.

Same session as `2026-0927-1625-go-styl-semicolons` (N-027) and
`2026-0927-1628-go-styl-sprintf-operator` (N-028), working through the next
list without user input.

## Stylus behavior

`visitUnit` in stylus's `lib/visitor/compiler.js` prints a float as
`parseFloat(n.toFixed(15))`: rounded to 15 decimal places, then the
shortest form. One quirk: when compressing, a float between -1 and 1 skips
the rounding (`n.toString().replace('0.', '.')`), so `0.1 + 0.2` compressed
prints `.30000000000000004`.

## Implementation

`internal/value/value.go`:

- New `formatNum(f)`: integers print as before. Other values go through
  `FormatFloat(f, 'f', 15)` → `ParseFloat` → `FormatFloat(-1)`, which is
  the same round-then-shortest as Stylus. A result of -0 prints as `0`.
- `Number.CSS` uses it, so color alpha (`rgba(0,0,0,0.3)`) gets it too. It
  is the only float formatter in the codebase.
- The compressed "zero length drops its unit" check now looks at the
  formatted text, so `-1e-17px` compresses to `0` rather than `0px`.
- Deliberate divergence: compressed numbers between -1 and 1 are rounded
  too. That Stylus quirk only ever exposes noise.

## Tests

- `TestNumberCSS` gained cases: `1.7999999999999998rem` → `1.8rem`,
  `0.1 + 0.2` → `0.3` / `.3`, `1/3` → `0.333333333333333`, a tiny negative
  → `0px` / `0`, and a plain `123456.789`.
- Checked against stylus 0.64 on `a * 1.8`, `0.1 + 0.2`, `1/3`,
  `1px * 0.1 * 3` and `rgba(#000, 0.1 + 0.2)`: identical in pretty mode.
- `go test ./...` passes; difftest unchanged at 23/32.

## Next

Closed: N-029. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: N-022. Full list: `ai_docs/todo/next-list.md`.
