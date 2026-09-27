# Session: playground prune-to-HTML pane (N-014) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Implemented next-list item N-014: expose M15's critical-CSS pruning in the
playground. Paste a rendered page and see only the CSS it would inline.

Same session as the items from `2026-0927-1625` to `-1702`.

## WASM API (`playground/wasm/main.go`)

`goStyl.compile(src, opts)` accepts `pruneHTML` (a string). After the
normal build succeeds, it runs `styl.UsedFromHTML(pruneHTML)` and then
`styl.Prune(src, used, opts)` with the same options, and adds to the
result:

- `pruned`: the pruned CSS (or `pruneError` if Prune fails);
- `used`: `{classes, ids, tags}`, converted with a new `toAny` helper,
  since `syscall/js` needs `[]any`;
- `ms` re-measured to cover both passes.

The doc comment at the top of the file describes the new fields. The
worker and runner pass options and results through unchanged, so no JS
plumbing was needed.

## UI (`playground/index.html`)

- `<details id="prunebox">` under the CSS output, styled like the globals
  box, with a textarea whose placeholder explains it.
- `compile()` sends `pruneHTML` only while the box is open and non-empty.
  With `pruned`, the output pane renders the pruned CSS and the status
  reads `pruned 73 of 118 chars (−38%)`. The summary shows
  `· 3 tags · 2 classes · 0 ids` (or the prune error).
- Typing in the textarea recompiles (debounced), and toggling the box
  recompiles. The HTML is saved in localStorage (`go-styl-prunehtml`,
  behind the N-017 `persist` gate) and restored. Share links carry it as
  `p` when the box is open.

## Verification (Chrome, rebuilt WASM, local server)

- Source with `.card`, `.btn`, `.unused`, `#hero`, `h1`, pruned against
  `<div class="card"><h1>Hi</h1><button class="btn">ok</button></div>`,
  keeps exactly `.card`, `.btn` and `h1`. Stats as above.
- Closing the box brings back the full CSS (including `.unused`).
- Screenshot: pruned output with the stats in the bars. Both bottom
  textareas are in the viewport (y 898–1002 of 1002).
- `GOOS=js GOARCH=wasm go vet ./playground/wasm` is clean.

## Next

Closed: N-014. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
