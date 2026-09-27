# Session: per-request globals in stylserve (N-012) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Implemented next-list item N-012: `stylserve.Options.Globals` was fixed per
engine ("use one Engine per theme"). Now the served CSS can vary per
request (tenant brand, user theme), with a cache keyed by the variable set.

Same session as the items from `2026-0927-1625` to `-1704`.

## API

- `stylserve.Engine.AssetWith(reqPath, globals map[string]any)`: `globals`
  is layered over `Options.Globals` (a per-call key wins). An empty map is
  the same as `Asset`, which is now `AssetWith(reqPath, nil)`.
- `stylserve.Options.MaxVariants` (default 256).
- `stylhttp.NewWithGlobals(opts, globals func(*http.Request) map[string]any, vary ...string)`.

## Design

- **Two caches.** `cache` (base builds, keyed by path, as before) and
  `variants` (keyed `"<path>\x00<fingerprint of merged globals>"`). Keeping
  them apart means evicting variants can't drop the base builds every
  plain request uses.
- **Fingerprint (`globalsKey`).** sha256 over the names sorted, each with
  `%T` and `%#v` of its value. It doesn't depend on map order, and it
  tells types apart: `"10"` (a Stylus expression string) and `10` (a
  number) key differently even though they print the same.
- **Cap.** The variable set may come from request data (Host, cookie,
  query), so an unbounded cache would be a memory problem. When a new
  variant would exceed `MaxVariants`, the variant map resets, the same
  policy as `stylcrit`'s `MaxCached`, since recompiles cost microseconds.
  A stale rebuild of an existing key doesn't count toward the cap.
- **Invalidation.** Variant entries carry the same dependency stamps, so
  a source or `@import` change rebuilds them.
- **HTTP caching.** The response depends on whatever the callback reads.
  `NewWithGlobals` adds each `vary` name as a `Vary` header, or sets
  `Cache-Control: private` when none are given, so a shared cache won't
  serve one tenant's CSS to another. ETags differ per variant because the
  bodies differ.

## Tests

- `stylserve/globals_test.go`:
  - layering: the per-call brand wins and the engine's `pad` stays; the
    variant ETag differs from the base; the same set comes from cache
    (same `*Asset`); an empty set returns the base asset;
  - `globalsKey` is type-aware and order-independent;
  - `MaxVariants: 3` holds over 10 distinct sets, and the base survives;
  - a source edit (mtime bumped) rebuilds a variant.
- `stylhttp/stylhttp_test.go` (`TestNewWithGlobals`): per-Host brands,
  the default for an unknown host, `Vary: Host`, and `Cache-Control:
  private` when no vary list is given.
- `go test -race ./stylserve ./stylhttp` passes.

The README's "Serving over HTTP" section gained a `NewWithGlobals` example,
and the `Options.Globals` doc comment points to `AssetWith`.

rweb's `middleware/stylus` lives in the rweb repo. It can adopt
`AssetWith` there; that's outside this repo, so no item was raised here.

## Next

Closed: N-012. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
