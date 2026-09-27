# Session: net/http critical-CSS middleware (N-015) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Implemented next-list item N-015: a `stylhttp` twin of rweb's
`middleware/critical`, built on `stylcrit`, so `net/http` servers get
per-response critical CSS too.

Same session as the items from `2026-0927-1625` to `-1700`.

## API

```go
handler := stylhttp.Critical(stylcrit.Options{Path: "styles/app.styl"})(mux)
```

`Critical(opts) func(http.Handler) http.Handler` is the standard
middleware shape, so it composes with any router. It takes the same
`stylcrit.Options` as the rweb middleware (Path/FS, Safelist, Globals,
etc.).

## Design

rweb's middleware reads the finished body after `ctx.Next()`. `net/http`
writes straight to the client, so the handler's writer is wrapped in
`critWriter`, which decides once, at `WriteHeader` (explicit, or implied by
the first `Write`):

- **Pass-through**: a non-2xx status, a `Content-Encoding` (can't rewrite
  gzip), or a `Content-Type` that is set and isn't `text/html`. Headers go
  out immediately, writes go straight through, and `Flush` is forwarded.
  SSE, JSON and downloads are never buffered.
- **Buffer**: everything else. In `finish`:
  - sniff the type with `http.DetectContentType` if none was set (what
    `net/http` would have done);
  - run `stylcrit.Engine.Inline` on an HTML body;
  - on change, drop `Content-Length` (the body grew) and `ETag` (it would
    describe the old body and survive a stylesheet change).
- A compile error gives a 500 with the positioned message
  (`critical CSS: broken.styl:2:3: …`), matching `stylhttp.New`'s "fail
  loudly".
- `Unwrap()` exposes the underlying writer to `http.ResponseController`.
  `Flush` on a buffered page is a no-op, since the whole body is needed to
  prune.

## Tests (`stylhttp/critical_test.go`, fstest.MapFS stylesheet)

- An HTML page gets `<style>.card{background:#0af}</style></head>`, with
  unused rules pruned and a stale ETag/Content-Length dropped.
- Untyped HTML is sniffed and rewritten.
- The Safelist keeps `.menu--open`.
- Pass-through, no rewrite: JSON, a 404, gzip.
- An event stream flushes to the recorder before the handler returns.
- A broken stylesheet gives a 500 with `broken.styl:2:3`.
- A 204 with no body keeps its status.
- `go test -race ./stylhttp ./stylcrit` passes. (A duplicate `body` test
  helper clashed with `stylhttp_test.go`'s identical one and was removed.)

README "Critical CSS" now names both adapters, with a `net/http` snippet
and the pass-through rules.

## Next

Closed: N-015. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
