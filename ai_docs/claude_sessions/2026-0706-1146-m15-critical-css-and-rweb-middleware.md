# Session: M15 Critical CSS + stylcrit engine + rweb middleware

- **Date:** 2026-07-06 11:46 (work done 2026-07-03; doc saved on session resume)
- **Session ID:** `566497df-3c30-4379-8695-c55a4f789faa`
- **Repos:** `~/projs/go/go-styl/` (branch `main`) and `~/projs/go/rweb/` (branch `master`)
- **Continuation of:** `2026-0703-0959-playground-globals-exposure.md`

> This session (same conversation as the playground-globals save): built
> **M15 Critical CSS** end to end — `styl.Prune`/`PruneFile` +
> `styl.UsedFromHTML` (commit `99be013`), the cached `stylcrit` engine
> (`7c693dd`), **tagged go-styl `v0.2.0`**, and shipped
> `rweb/middleware/critical` + an element-rendered example in the rweb repo
> (`d788f64`, pushed). Killer feature #2 is done.

---

## 1. M15 core (go-styl `99be013`)

### `internal/css/prune.go`

- `css.Prune(nodes, UsedNames, pretty)` on the **resolved** node tree
  (post-extend/merge — same layer as M14's CollectNames; reuses its
  readIdent/skipQuoted/skipAttr/keyframesName helpers).
- `UsedNames{Classes, IDs, Tags map[string]bool}` — **nil map = "unknown,
  don't filter that axis"**; non-nil empty = known empty. Public mirror
  `styl.Used` has the same nil semantics on slices.
- Keep rule: a selector survives iff every class/ID/tag it *requires* is
  used. Requirements scanner (`scanSelector`): tracks `expectType` so tag
  idents are only read at start/after combinators (` >+~`); pseudo-classes
  are consumed after `:`/`::` so `hover` never reads as a tag; **names
  inside functional pseudo parens (`:not(...)`, `:is(...)`) are skipped**
  (never required — safe direction); quoted strings/attr blocks skipped.
- Requirement-free selectors always survive: `@font-face` (it's a `*Rule`
  whose Selector is the literal header!), `:root`, `*`, attribute-only.
- Per-selector pruning of `Selectors` (rebuild joined `Selector` only when a
  subset survives — needs `pretty` for the ", " join), `Extenders`, and
  `Duplicates` (merge groups). Shallow copies — eval's nodes untouched.
- `splitTopLevel` guards stray comma-joined strings (keep if ANY part
  matches; over-requiring would wrongly drop).
- Keyframes pass: collect ident + quoted-string tokens from
  `animation`/`animation-name` values (vendor prefix stripped via
  `stripVendor`) across the kept tree, then drop unreferenced `@keyframes`
  and re-drop emptied at-rules. Superset matching = only ever over-keeps.

### Public API (`prune.go`, `internal/eval/prune.go`)

- `styl.Prune(src, Used, opts)` / `PruneFile` — full Options honored
  (Globals, CustomProperties, FS, imports) via `eval.EvaluatePruned`
  (evalNodes → css.Prune → RenderSheet).
- `styl.UsedFromHTML(html) Used` — dependency-free byte scanner: tags +
  `class`/`id` attrs, quoted/unquoted values, comments/doctype/close-tags
  skipped; always returns non-nil sorted slices. Unquoted attr values stop
  at `/` so `class=foo/>` records `foo` (recording `foo/` would *wrongly
  prune* `.foo` — under-keep is the unsafe direction).

### Tests (`m15_test.go`)

- Flagship + 20 focused cases (compound/multi-selector/extend/merge/media/
  keyframes/:root/attr/charset) + **no-op invariant**: prune with
  everything-used == full compile byte-for-byte (pretty & compressed) +
  UsedFromHTML scanner cases + end-to-end page flow.
- Real-world check: pruning `examples/02-nesting.styl` against a fake page
  correctly dropped `nav > .brand` and narrowed `h1, h2, h3` → `h2`.

## 2. stylcrit engine (go-styl `7c693dd`)

- `stylcrit.New(Options{Path, FS, …, Safelist styl.Used, MaxCached})` →
  `Engine.CSS(html)` / `Engine.Inline(html)`.
- Cache keyed by sha256 of the used-name set (axis boundaries marked);
  **dep invalidation** via stat stamps from a throwaway `styl.BuildFile`
  (only way to learn the @import list); cache full → reset (compiles are µs).
- `Safelist` merged into every request's used set — for names client-side
  JS toggles after load (the classic critical-CSS footgun).
- `Inline` injects `<style>` before `</head>`, falls back `</body>`, then
  prepend; page needing zero CSS returned unchanged.
- README: new "Critical CSS (`styl.Prune`)" section + stylcrit usage;
  Status bullet; roadmap M15 checked; killer_features #2 marked shipped.

## 3. rweb middleware (rweb `d788f64`, pushed)

- `middleware/critical`: `s.Use(critical.Middleware(stylcrit.Options{…}))`
  — after `ctx.Next()`, post-processes only **2xx `text/html`** responses
  (rweb buffers bodies: `res.Body()`/`SetBody`; default status is 200);
  compile errors fail loudly via `serr.Wrap` (positioned messages).
- Tests render the page with **element** (`H2Class`, `DivClass`, …);
  cover inject/safelist/JSON-untouched/redirect-untouched/500-on-broken.
- `examples/critical_css`: two element-rendered pages over one
  `styles/app.styl` — Home inlines card/btn/spinner+keyframes, About drops
  them; `.menu--open` via Safelist; `.legacy-table` on neither.
- **Live-verified**: ran the example, curled both pages, and confirmed
  live cache invalidation by editing `app.styl` (color swap both ways).

## 4. Versioning dance

- Auto-mode classifier **denied** pushing an agent-chosen `v0.2.0` tag
  (publishing a release = user decision). Interim: pinned rweb to the
  pseudo-version of the pushed commit (`go get go-styl@7c693dd`).
- User then authorized: full rweb suite green → tagged **`v0.2.0`**
  (annotated: M13/M14/M15) → pushed tag → rweb bumped to `v0.2.0`,
  amended into the middleware commit, **pushed** to rweb `master`.
- Known nit: the amended rweb commit message still says "pseudo-version"
  in its body; left rather than force-pushing public `master`.

## Commits this session (after the playground-globals save)

- go-styl (`main`, all pushed): `99be013` M15 core · `7c693dd` stylcrit ·
  `213f817` killer-features doc · tag **`v0.2.0`**
- rweb (`master`, pushed): `d788f64` critical middleware + example +
  go-styl v0.2.0

---

## Roadmap

- [x] M1–M15 + stylcrit + rweb middleware (killer features #1, #2, #3-codegen ✅)
- [ ] Next candidates (`ai_docs/killer_features.md`):
  - **Scoped styles (#3 first half)**: `styl.Component` hashed class names →
    feed the same `GoSource` renderer
  - **LSP (#4)** — biggest adoption lever, biggest lift (`styl fmt` falls out)
  - Migration tool (#5) / sandbox (#6) / stylserve live reload
  - Playground: a "prune against this HTML" pane (M15 exposure)
  - net/http critical-CSS adapter (stylhttp twin of rweb/middleware/critical)

## Carry-forward notes

- **Prune safety direction is the design invariant**: every heuristic
  (pseudo-paren skip, comma-split any-match, keyframes superset tokens,
  HTML-scanner over-collection) must err toward *keeping*. Under-keeping is
  a correctness bug (missing styles); over-keeping is just bytes.
- `stylcrit` restamp does a full `BuildFile` just for `Result.Deps` — if a
  `Prune`-with-deps API ever lands, use it there.
- rweb middleware relies on rweb buffering the whole response —
  streaming/SSE responses never match `text/html` + 2xx guard, but keep in
  mind if rweb grows streamed HTML.
- gofmt in rweb: several pre-existing unformatted files (Context_test.go,
  Cookie.go, core/rtr/*…) — don't "fix" them incidentally.
- Shell cwd gotcha hit again (go test from `examples/critical_css`);
  gates from repo root.
- go-styl v0.2.0 is public: `styl.Used`'s nil-vs-empty slice semantics and
  the Prune keep-rules are now API — document-before-change territory.
