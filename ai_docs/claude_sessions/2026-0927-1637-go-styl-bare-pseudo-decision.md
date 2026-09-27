# Session: bare nested pseudo-class kept as an extension (N-035) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Next-list item N-035 said a nested selector starting with a pseudo-class
joins its parent without a space (`.x` over `:hover` → `.x:hover`), while
Stylus gives `.x :hover` (a descendant). The user asked for next-list items
done on my best recommendation. The recommendation was to **decline**:
keep go-styl's behavior and document it.

Same session as N-027…N-030 and N-022 (`2026-0927-1625` … `-1636`).

## Findings

- stylus 0.64: `.x` over `:hover`, `::before` and `:not(.a)` all give
  descendants (`.x :hover`, `.x ::before`, `.x :not(.a)`). `&:focus`
  attaches.
- go-styl's `combine` (`internal/eval/selector.go`) has attached a leading
  `:` since the initial commit. It is **documented**: README feature list
  ("pseudo-class attachment") and the playground tutorial ("a bare
  pseudo-class line (`:hover`) attaches to the parent too — go-styl treats
  it like `&:hover`").
- No example, test or corpus file depended on either reading. cema doesn't
  use it (N-022 parity was unaffected).

## Decision and why

Keep attaching:

- A descendant `:hover` means "any hovered element inside `.x`", which is
  almost never what someone writing a nested `:hover` wants.
- Switching would silently change the meaning of sheets written from
  go-styl's own tutorial, with no error.
- Stylus sheets that really want the descendant form can write `& :hover`,
  which gives `.x :hover` in both compilers (verified).

## Changes

- `README.md`, "Deliberate differences": new bullet on bare pseudo-classes,
  including the `& :hover` escape hatch.
- `difftest/corpus/bare-pseudo.styl` pinned in `known_diffs.txt` with a
  hand-written note. Score 23/34.
- Next list: N-035 moved to Non-goals with the reason.

## Next

Closed: None. Declined: N-035. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
