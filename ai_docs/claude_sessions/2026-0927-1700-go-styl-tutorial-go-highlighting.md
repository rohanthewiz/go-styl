# Session: Go highlighting in tutorial prose (N-019) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Implemented next-list item N-019. The tutorial's `go` snippets (the
`embed.FS` example in the imports lesson, the `Options.Globals` example in
runtime theming) were escaped plain text, while Stylus and CSS snippets
were highlighted.

Same session as the items from `2026-0927-1625` to `-1659`.

## Implementation

`playground/highlight.js`: new `go(src)`, exported as `stylHi.go`.

- It scans the whole snippet left to right, not per line, because raw
  `` `…` `` strings can span lines. At each position it tries, in order:
  - comments (`//…` including `//go:embed`, and `/* */`)
  - strings (`"…"` and `'…'` with escapes, raw backticks)
  - numbers (hex, decimals, exponents, `_` separators)
  - identifiers
  - operators (`:=`, `...`, `&&`, `||`, `<-`, compound assignments)
  - anything else, escaped as is
- Identifier classes, first match wins: keyword → `t-kw`;
  `true/false/nil/iota` → `t-num`; followed by `(` → `t-fn`; predeclared
  type → `t-cls`; capitalized right after a `.` (like `styl.Options`) →
  `t-cls`; otherwise `t-id`.
- It reuses the existing token classes and CSS variables, so both themes
  work unchanged.

`playground/tutorial.js` (`block`): `lang === 'go'` now uses
`stylHi.go`.

## Verification

- Node: a snippet with `//go:embed`, a selector, a call, a composite
  literal, hex, `true`, a multi-line raw string, an escaped quote, a
  trailing comment and a `func` declaration all tokenized as intended.
  With the tags stripped and entities unescaped, the output equals the
  input.
- Chrome: `#tut/imports` renders the `embed.FS` snippet with 15 token
  spans. The screenshot shows the comment italic, `var` as a keyword,
  `CompileFile` as a call, and `FS`/`Options` as types.

## Next

Closed: N-019. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
