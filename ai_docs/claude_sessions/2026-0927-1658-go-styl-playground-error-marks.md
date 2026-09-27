# Session: compile errors marked in the playground editor (N-016) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Implemented next-list item N-016: "mark compile errors in the editor gutter
from the result's `line`/`col`." The playground editor is a highlighted
`<pre>` under a transparent `<textarea>`, with no gutter or line numbers.
So the feature was built for that editor instead: the error line gets a
tinted band with a red bar at the left edge, and the error message is
clickable to jump there.

Same session as the items from `2026-0927-1625` to `-1655`.

## Implementation

- `playground/highlight.js`: new `stylHi.errorMark(ta)`.
  - Creates a `.errband` div inserted just before the textarea, so it
    paints above the highlighted `<pre>` and below the textarea's text.
  - Positions it from the textarea's computed `padding-top` and
    `line-height` minus `scrollTop`, and re-places it on the textarea's
    `scroll`.
  - API: `show(line, col)`, `clear()`, `jump()` and `fromResult(r)`.
    `jump()` focuses the textarea, puts the caret at line:col (clamped to
    the line) and scrolls the line about a third of the way down.
    `fromResult(r)` shows the error only if it's in this source.
- `playground/index.html`: `.errband` and `pre.err.jumpable` CSS; the play
  editor's `srcMark`; the error box click jumps; the compile result calls
  `fromResult` (with a tooltip) or `clear()` on success.
- `playground/tutorial.js`: the same for the tutorial editor.

"In this source" means `r.file` is empty, `<input>`, or ends in
`playground.styl`. The WASM side compiles both editors as
`playground.styl` (`playground/wasm/main.go`). My first draft checked only
for `<input>` and never matched; browser testing caught it.

## Verification (Chrome, local server)

- Play tab, 4-line source with `width 1px +` on line 3: error
  `playground.styl:3:3`, band visible at `top 51.8px` (12.8px padding +
  2 × 19.5px), height 19.5px. Clicking the message puts the caret at 3:3.
  Screenshot shows the band and bar on line 3.
- Tutorial, a 122-line source with the error on line 122: click scrolls
  to `scrollTop` 2195, the band follows to 177.8px (in view), caret
  122:3. Reset clears the band.
- Gotcha: the local server's assets were cached across reloads. I forced
  a refetch with `fetch(u, {cache: 'reload'})` before reloading.

## Found along the way

`width 1px +` reports `unexpected "" in expression`. The EOF token's text
is empty. Raised as **N-039** (low).

## Next

Closed: N-016. Declined: None. Raised: N-039.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
