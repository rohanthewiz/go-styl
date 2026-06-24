# Session: go-styl repo split + M3 (built-in function library)

- **Date:** 2026-06-24 17:12
- **Session ID:** d70bff87-4117-49cb-bece-678a56c10f16
- **Active repo:** `~/projs/go/go-styl/` (module `github.com/rohanthewiz/go-styl`, branch `main`)
- **Continuation of:** `2026-0624-1641-go-styl-m0-m2.md` (M0–M2 build)

> This session picks up right after M0–M2 were committed. It covers: committing
> and pushing the early work, splitting the project into its own clean repo,
> resolving the license, cleanup, and implementing **M3** (the built-in function
> library).

---

## 1. Commit & push of M0–M2

- Created branch `rebuild-go-styl` in `~/goprojs/scarlet`, committed M0+M1 and M2
  separately, plus a session-log doc commit.
- Pushed `rebuild-go-styl` to the old remote `origin` (`rohanthewiz/scarlet.git`).
- Flagged a mismatch: module path was `github.com/rohanthewiz/go-styl` but the
  GitHub repo was still `scarlet`.

## 2. Repo split into a clean go-styl project

User created a new repo `https://github.com/rohanthewiz/go-styl.git` and asked to
**separate the new work from the old code entirely**, locally at `~/projs/go/go-styl/`.

Decision (asked): **fresh single commit**, no scarlet history.

Steps performed:
- Exported the tracked working tree via `git archive HEAD | tar -x` into
  `~/projs/go/go-styl/` (gets only tracked files, no `.git`, no `.idea`).
- **Stripped stale files**: `.drone.yml` (old aerogo Drone CI with their
  codecov/discord secrets) and `.github/FUNDING.yml` (funded the original authors).
- `git init -b main`, single initial commit, `remote add origin` → the new repo,
  `push -u origin main`. 32 tracked files; build/test/vet green in the new location.

## 3. License resolution (flagged before pushing public)

The inherited `LICENSE` was a non-standard **"Sponsorship License" by Eduard
Urbach** requiring **$10/month for commercial use**. Surfaced this rather than
silently keeping or deleting it.

Decision (asked): **replace with MIT** (© 2026 Rohan Allison). Done.

## 4. Cleanup

- Old repo `~/goprojs/scarlet`: switched back to `master` (original scarlet code
  restored, untouched), deleted local `rebuild-go-styl` branch.
- **Blocked:** deleting the remote `origin/rebuild-go-styl` on `scarlet.git` was
  denied by the safety classifier (remote-branch deletion needs explicit
  authorization). Left for the user to run manually:
  `git -C ~/goprojs/scarlet push origin --delete rebuild-go-styl`

## 5. M3 — built-in function library (committed `823dfb6`, pushed)

**~55 functions**, organized with an `init()`-based registry + shared arg helpers.

Supporting infrastructure in `internal/value`:
- `color.go` — HSL↔RGB conversion (`(*Color).HSL`, `NewColorHSL`), clamps.
- `names.go` — full CSS named-color keyword → hex table.
- `builtin.go` `toColor()` coerces `*Color` and color-keyword idents (e.g. `blue`,
  `rebeccapurple`) to colors so color functions accept named colors.

Functions (`internal/builtin/{color,math,list,string,type}.go`):

| Category | Functions |
|---|---|
| color | rgb rgba hsl hsla red green blue alpha hue saturation lightness lighten darken saturate desaturate mix tint shade complement invert |
| math | abs ceil floor round sqrt sin cos tan min max pow percentage |
| list | length push append unshift prepend index last first join |
| string | unquote quote s uppercase lowercase substr replace split |
| type | typeof type unit match light dark |

Spot-checked correctness, e.g.:
`lighten(#000,50%)→#808080`, `darken(#fff,25%)→#bfbfbf`,
`hsl(120,50%,50%)→#40bf40`, `mix(#fff,#000)→#808080`,
`complement(red)→#0ff`, `percentage(0.25)→25%`, `max(3px,9px,5px)→9px`.

Tests added:
- `testdata/m3.styl` + `m3.css` / `m3.min.css` golden.
- `m3_test.go` — 41-case table covering every category (compiled compressed).

---

## Verification status (session end)

`go build ./...`, `go vet ./...`, `gofmt -l .`, and `go test ./...` all clean /
passing in `~/projs/go/go-styl/`.

## Commits on `main`

1. `799c534` — Initial commit: go-styl (M1 + M2)
2. `823dfb6` — M3: built-in function library (color/math/list/string/type)

(Both pushed to `origin` = `github.com/rohanthewiz/go-styl.git`.)

## Note on tooling

gopls emits "not in workspace" / "undefined" diagnostics for files under
`~/projs/go/go-styl` because the editor workspace still points at the old scarlet
module. These are false positives — `go build`/`go test` are authoritative and
pass. (A `go.work` could silence them if desired.)

---

## Known carry-forward limitations (still open)

- Whitespace-sensitive `-`/`+` (currently always binary operators).
- `calc()` not special-cased (its args get evaluated).
- Selector comma-splitting doesn't respect brackets/strings.
- No `!important` handling.
- At-rules (`@media`, `@keyframes`, …) currently error.

## Next up — M4

Interpolation (`{expr}` in selectors/properties/strings), `@extend` +
`$placeholder` selectors, and `@import`. This replaces the at-rule error path and
unlocks dynamic selectors like `.col-{i}` inside `for` loops.

Roadmap:
- [x] M1 vertical slice
- [x] M2 control flow + functions/mixins
- [x] M3 built-in function library
- [ ] M4 interpolation, `@extend`/placeholders, `@import`
- [ ] M5 at-rules, compress parity, brace syntax, sourcemaps
