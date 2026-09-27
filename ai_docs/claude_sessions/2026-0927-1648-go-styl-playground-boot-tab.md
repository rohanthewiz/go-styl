# Session: playground boot always sets the tab (N-020) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Closed next-list item N-020. The playground's boot code ran
`if (tab === 'tutorial') switchTab('tutorial')`, so booting to the default
play tab never called `switchTab`, and `body[data-tab]` stayed unset until
the first tab click. It was harmless until some CSS keyed on
`body[data-tab="play"]`.

Same session as the go-styl compiler items earlier today
(`2026-0927-1625` … `-1647`).

## Change

`playground/index.html`, end of the boot IIFE: it now calls
`switchTab(tab === 'tutorial' ? 'tutorial' : 'play')`. The ternary also
normalizes an unexpected stored value (from localStorage) to `play`.
`switchTab` sets `body.dataset.tab`, the view `hidden` flags and tab
classes, and saves the choice, so the first paint matches a clicked tab.

No WASM rebuild is needed (HTML only). Pages deploys on push via
`.github/workflows/pages.yml`. Not browser-tested this session; the change
is one line in the boot path, and `switchTab('play')` is the same call the
Playground tab button makes.

## Next

Closed: N-020. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
