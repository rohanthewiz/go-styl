# Session: undefined mixin calls documented as deliberate (N-030) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Closed next-list item N-030. A statement-level call to an undefined mixin
(`sermon()`, `+sermon`) is an error in go-styl, while stylus 0.64 drops it
silently. The item's own recommendation, taken as-is under the user's "use
your best recommendation" instruction: keep the error and record it as a
deliberate divergence. In cema the error found three dead calls
(`sermon()`, `fullcalendar()`, a bodiless `payment_form()`).

Same session as N-027 (`2026-0927-1625-go-styl-semicolons`), N-028
(`2026-0927-1628-go-styl-sprintf-operator`) and N-029
(`2026-0927-1630-go-styl-number-formatting`).

## Changes

- `README.md`: new "Deliberate differences" subsection under "Compatibility
  with reference Stylus". It lists the undefined-mixin error and the other
  lenient or intentional choices made while closing cema's blockers:
  escaped quotes in strings (N-026), `;` inside `url(...)` and
  `@extend .x;` (N-027), and always-rounded numbers (N-029).
- `difftest/corpus/undefined-mixin.styl`: a minimal `.card` / `sermon()` /
  `color red` sheet. go-styl errors and stylus outputs the rule, so it's a
  `go-error` outcome. It is pinned in `difftest/known_diffs.txt` with a
  hand-written note. The ratchet now fails if the behavior changes silently.
  The score is 23/33 (was 23/32; the new file is a known diff by design).

Gotcha: the first corpus draft called `shadow()`. go-styl's hint said
`did you mean "shade"?` (a builtin), which made the generated known-diffs
note misleading. Switched to cema's real `sermon()`.

## Next

Closed: N-030. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: N-022. Full list: `ai_docs/todo/next-list.md`.
