# Session: parent reference anywhere in a selector (N-025) (2026-09-27)

Session ID: `6e41c92e-5a0b-4cdf-b02a-b5b9e0524fcc`

This session implemented next-list item N-025: a parent reference (`&`) that
isn't at the start of a nested selector. Before the fix, go-styl emitted
`.checkbox &` literally: under `.x .a` it gave `.x .a .checkbox &` instead of
`.checkbox .x .a`. The CSS was silently wrong. This was one of the blockers
for N-022 (compile cema with go-styl). It is the same session as
`2026-0927-1357-go-styl-multiline-selectors`, which did N-024.

## Stylus behavior (probed against stylus 0.64)

- **Every `&` is replaced.** When a nested selector contains `&` anywhere,
  each `&` becomes the parent selector and nothing is prepended. Examples:
  - `.c &` under `.a, .b` gives `.c .a, .c .b`
  - `& + &` gives `.x + .x`
  - `html.ie &.y` gives `html.ie .x.y`
  - `:not(&)` gives `:not(.x)`
  - `> &` gives `> .x`
- **The replacement is plain text.** Stylus replaces `&` even inside an
  attribute string: `[data-a="&"]` gives `[data-a=".x"]`.
- **Groups resolve per selector.** `.p &, .q` gives `.p .x, .x .q`.
- **Deeper nesting uses the resolved selector.** Children of a rule like
  `.a &` nest under that result: `.b` gives `.a .x .b`, and `&:hover` gives
  `.a .x:hover`.
- **At the top level, `&` is nothing.**
  - `.checkbox &` gives `.checkbox`
  - `& .a` gives `.a`
  - A selector that is only `&` makes Stylus drop the rule.

## What changed

All changes are in `internal/eval/selector.go`:

- **`combine`**: if the child selector contains `&`, it returns
  `strings.ReplaceAll(child, "&", parent)`. That check runs before the switch
  on the first character, so it replaces the old `case '&'` branch, which only
  handled a leading `&`. The doc comment now covers the any-position rule. It
  also records why strings are not skipped: to stay byte-compatible with
  Stylus.
- **`combineSelectors`**, top level: each selector with `&` has the `&`
  removed and is then trimmed. A selector that ends up empty is dropped. If
  that would leave the group empty, the original selectors are kept, so the
  rule still prints.

## Tests

- `parent_ref_test.go` has 13 cases, all checked against stylus: a trailing
  `&`, a parent group, `& + &`, `&` mid-selector, a `&-icon` suffix,
  `:not(&)`, an attribute string, a mixed group, children and a `&:hover`
  under a rule like `.a &`, `> &`, and two top-level cases.
- `go test ./...` passes. Difftest is unchanged at 23/32.

## Notes

- **Known difference:** a top-level selector that is only `&` still prints
  `&{…}`, while Stylus drops the rule. It's a nonsense input, so I left it and
  wrote it into the N-025 closure line.
- The untracked `.cats-todo/` folder isn't from this work and is left out of
  commits.

## Next

Closed: N-025. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: N-022 (blockers now N-026…N-030). Full list: `ai_docs/todo/next-list.md`.
