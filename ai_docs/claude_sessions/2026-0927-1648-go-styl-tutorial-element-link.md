# Session: tutorial links to element's tutorial (N-021) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Closed next-list item N-021. element's tutorial links to go-styl; the
go-styl tutorial now links back.

Same session as `2026-0927-1648-go-styl-playground-boot-tab` (N-020) and
the compiler items before it.

## Change

`playground/tutorial.js`, lesson `next` ("Where to next"): a new paragraph
after the README pointer:

> Building the HTML in Go too? element's tutorial covers its
> zero-dependency HTML builder — the natural companion to go-styl's typed
> class names.

It links to https://rohanthewiz.github.io/element/#tutorial with
`target="_blank" rel="noopener"`, like the README link above it.
`node --check playground/tutorial.js` passes. No WASM rebuild needed.

## Next

Closed: N-021. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
