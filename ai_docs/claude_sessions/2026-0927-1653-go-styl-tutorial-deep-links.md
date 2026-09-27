# Session: tutorial deep links (N-018) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Implemented next-list item N-018: the playground only handled `#tutorial`.
Now `#tut/<lesson-id>` links straight to a lesson, and the URL always
names the open lesson, so it can be copied and shared.

Same session as the items from `2026-0927-1625` to `-1650`.

## Behavior

- **At load:** `#tut/lists` switches to the tutorial and opens the Lists
  lesson (the lesson opens when the WASM compiler boots, where the
  tutorial is initialized). `#tutorial` opens the last-open lesson as
  before.
- **While the tutorial tab shows:** the address bar is kept at
  `#tut/<id>` of the open lesson (Prev/Next, the nav list, switching back
  from the play tab). `history.replaceState` avoids a history entry per
  lesson and a `hashchange` loop.
- **Play tab:** switching to it drops a `#tut…`/`#tutorial` hash.
- **`hashchange`:** a `#tut/<id>` link clicked or pasted while loaded
  opens that lesson; an unknown id snaps back to the open lesson's hash.

## Implementation

- `playground/tutorial.js`:
  - `init(opts)` takes `opts.lesson`, which overrides the stored lesson.
  - `syncHash()` runs in `open()`.
  - the returned API adds `syncHash` and `openId(id)`.
  - module helpers `indexOf(id)` and `lessonFromHash(hash)` (regex
    `^#tut/([\w-]+)$`); `lessonFromHash` is exported on `gsTutorial`.
- `playground/index.html`:
  - `linkedLesson` is read once next to `let tutorial` (declared before the
    boot callback that uses it, so boot timing doesn't matter).
  - the boot tab check accepts it; `switchTab` syncs or clears the hash.
  - a `hashchange` listener.

## Verification (Chrome, local `go run ./playground/serve`)

- `/#tut/lists`: tutorial tab, "6 / 20", Lists lesson.
- Next: hash `#tut/mixins`. Play tab: hash cleared, `data-tab=play`. Back
  to tutorial: `#tut/mixins`.
- `location.hash = '#tut/loops'`: opens "Loops & ranges".
  `#tut/nope`: back to `#tut/loops`.
- Reload on `/#tutorial`: last lesson, hash becomes `#tut/loops`.
- Reload with no hash after using the play tab: play tab, CSS compiled. No
  console errors.

## Next

Closed: N-018. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
