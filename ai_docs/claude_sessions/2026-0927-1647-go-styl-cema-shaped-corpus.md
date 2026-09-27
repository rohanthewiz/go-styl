# Session: cema-shaped difftest corpus sheet (N-006) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Closed next-list item N-006: the difftest corpus had missed the basic
syntax a real indented-syntax project (cema) uses. This adds a sheet shaped
like cema's `styles/styl`, so those features stay locked to Stylus's output.

Same session as N-027…N-030, N-022, N-035…N-038, N-008 and N-001
(`2026-0927-1625` … `-1646`).

## What was added

- `difftest/corpus/cema-shaped.styl`: a `body` / `#main` site sheet that
  exercises, in the way cema writes them:
  - `@require 'imports/site/*'` (glob, import once; N-023)
  - trailing `;` and several declarations per line (N-027)
  - `"calc(100vh - %s)" % mid-diff` and `% 70%` (N-028)
  - `font-size-base * 1.8` printing `1.8rem` (N-029)
  - multi-line selector groups, nested groups ordered child-major (N-024,
    N-022)
  - `.checkbox &,` / `.radio &` trailing parent refs (N-025)
  - `content: "\2022"` (N-026)
  - `sizes[0]`, `sizes[-1]` (N-008)
  - `url(icon-path + "search.svg")` (N-001)
  - `"Section " + 2` (N-036)
  - mixins with default args and `&:hover, &:focus`, `clearfix()`
- `difftest/corpus/imports/site/_globals.styl` and `_mixins.styl`:
  partials. `corpusGlobs` only picks up `difftest/corpus/*.styl`, so they
  aren't compiled on their own. `_mixins` re-requires `_globals` to
  exercise import-once.

Result: the sheet matches stylus 0.64 under the difftest normalizer.
Score 24/35 (from 23/34); no known-diff entry needed.

## Next

Closed: N-006. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
