# Session: shareable playground links (N-017) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Implemented next-list item N-017: the playground had no way to share code.
A **share** button now puts the play tab's state in the URL.

Same session as the items from `2026-0927-1625` to `-1653`.

## Design

- **Hash format.**
  - `#z/<base64url(deflate-raw(JSON))>`, using the browser's
    `CompressionStream`, so no library is needed (the page has no build
    step).
  - `#u/<base64url(UTF-8 JSON)>` as a fallback where `CompressionStream`
    is missing.
  - JSON is `{ s: source, g: globals text, o: { pretty, merge, sourcemap } }`.
  - The single-letter prefixes can't collide with `#tutorial` or
    `#tut/<id>` (N-018).
- **Share click.** Encodes the state, then `history.replaceState` to the
  hash, then tries `navigator.clipboard.writeText`. The status shows
  "link copied" or "link in address bar" plus the URL length, and clears
  after 4s.
- **Opening a link.** At load (after the normal restore) or on
  `hashchange`, `decodeShare` returns the state (or null for any other or
  malformed hash). `applyShare` fills the editor, globals (opening the
  panel if non-empty) and options, switches to the play tab and compiles.
- **The visitor's draft is protected.** Opening a link sets
  `persist = false`, so `compile()` skips the localStorage writes.
  `endShare()` runs on the first source or globals edit, an example pick,
  or "open in playground". It turns persistence back on and drops the
  now-stale share hash. Sharing your own code keeps `persist = true`.

## Files

- `playground/index.html`: share button and status span (`.playctl`, so
  they hide on the tutorial tab), CSS; the "shareable links" block
  (`b64url`, `pipeBytes`, `encodeShare`, `decodeShare`, `applyShare`,
  `endShare`, click handler); `persist` gate in `compile()`; `endShare`
  hooks; the share branch in the `hashchange` handler; decode at load.

## Verification (Chrome, local server)

- Custom source, globals `pad = 4px`, merge on, then share: `#z/…` hash,
  a 155-char URL. The clipboard fell back to "link in address bar",
  expected for a script-triggered click with no user gesture.
- Set localStorage src to `SAVED DRAFT`, then open the share URL: source,
  globals and the merge option all restored, compiled output contains
  `c0ffee`, the saved draft untouched, the hash present.
- One edit: hash cleared, localStorage now holds the edited shared code.
- Header screenshot: the button matches the header styling.

## Next

Closed: N-017. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
