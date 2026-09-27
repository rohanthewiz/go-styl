# Session: dev-mode live reload (N-013) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Implemented next-list item N-013 (dev-mode live reload for served
stylesheets, a "killer features" honorable mention). Edit a `.styl` file
and the open page picks up the new CSS in place, with no reload and no
lost page state.

Same session as the items from `2026-0927-1625` to `-1712`.

## Design

- **Opt-in flag** `stylserve.Options.LiveReload`. The engine ignores it;
  adapters read it. It lives on the shared options so rweb's adapter can
  adopt it the same way.
- **`stylhttp` serves two extra paths** under the stylesheet prefix
  (`stylhttp/live.go`):
  - **`_live.js`** (no-store). It takes the prefix from
    `document.currentScript.src`, so it works behind any mux prefix. It
    matches each `link[rel~=stylesheet]` under that prefix and opens an
    `EventSource` for it. On `change` it clones the link with
    `?v=<etag>`, inserts the clone, and removes the old link on load, so
    the page is never unstyled in between. On `error` it calls
    `console.error` with the positioned message.
  - **`_live?css=app.css`** (SSE). It sends a comment line to open the
    stream, records the current ETag, and then calls
    `AssetWith(name, requestGlobals)` every `livePoll` (500ms).
    `AssetWith` already rebuilds when a source or `@import` changes, so
    this reuses the exact invalidation. A new ETag sends `change` (data:
    the etag), a new compile error sends `error` (multi-line safe). It
    stops when the request context ends.
- **Why polling, not fsnotify.** No new dependency, it works for any
  `fs.FS` with real mtimes, and one stat per dependency per 500ms is
  nothing in development. The README says to keep it off in production.
- `handler` gained `live` and a shared `requestGlobals` helper, so a
  `NewWithGlobals` handler streams the caller's own variant.

## Tests (`stylhttp/live_test.go`, `livePoll` = 20ms)

- `_live.js` is served as `text/javascript` and contains `EventSource`.
- With LiveReload off it's a 404 (falls through to the normal asset
  lookup).
- Against a real `httptest.Server`, the stream reports `change` after an
  edit, `error` after a broken edit, and `change` after the fix. The
  source mtime is bumped explicitly so a same-second rewrite still counts.
- `_live` without `?css=` gives 400.
- `go test -race ./stylhttp` passes.

## Browser verification

A throwaway server (`.livedemo/`, deleted afterwards) served a page with
`/css/app.css` and `/css/_live.js`. In Chrome:

- The box was `rgb(255,0,0)`, then text was typed into an input.
- The source was edited to blue: about 1s later the box was
  `rgb(0,0,255)`, the input kept its text, one link remained with
  `?v=…`, and the console showed `[styl] reloaded app.css`.
- A broken edit logged `[styl] app.css: .livedemo/styles/app.styl:2:3:
  undefined mixin "nope"`, and the box stayed blue.
- The fix turned it `rgb(0,128,0)`.

README "Serving over HTTP" has a LiveReload subsection.

## Next

Closed: N-013. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
