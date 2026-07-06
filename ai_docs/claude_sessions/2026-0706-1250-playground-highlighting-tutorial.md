# Session: playground syntax highlighting + interactive tutorial

- **Date:** 2026-07-06 12:50
- **Session ID:** `fff553c0-f15a-4416-b910-700e304b8006`
- **Repo:** `~/projs/go/go-styl/` (module `github.com/rohanthewiz/go-styl`, branch `main`)
- **Continuation of:** `2026-0706-1146-m15-critical-css-and-rweb-middleware.md` (M15 critical CSS)

> This session: added toggleable **syntax highlighting** to both playground
> panes (new `highlight.js`, "highlight" header checkbox) and a 20-lesson
> **interactive tutorial** as a second tab (`tutorial.js`), with every lesson
> snippet batch-verified against the real compiler and the whole UI driven in
> headless Chrome. No compiler changes — playground-only.

---

## 1. Syntax highlighting (`playground/highlight.js`)

Dependency-free, self-contained (playground stays CDN-free). Exposes
`stylHi = { styl, css, escape, editor }`; also `module.exports` so Node can
require it for testing.

### Stylus tokenizer (`stylHi.styl`)

Per-line scanner with one piece of context: **a line whose next non-blank
line is indented deeper (or that ends in `{` or `,`) opens a block** and is
highlighted as a selector; other lines are declarations / assignments /
control flow / at-rules / mixin calls. That heuristic distinguishes `body a`
(selector) from `color red` (declaration) without a real parse. Shared
`value()` expression scanner handles comments (block state carries across
lines), strings, hex colors, numbers+units, `!important`, `{interp}` braces,
`fn(` names, keywords, operators.

### CSS tokenizer (`stylHi.css`)

Context-stack scanner (`sel` / `body`): at-rule params, selectors, `prop:`,
values. `@media`/`@supports`/`@keyframes`/`@layer`/`@container` push a `sel`
context (their braces contain selectors); other at-rules push `body`.

### Overlay editor (`stylHi.editor(textarea, code, enabled)`)

Classic transparent-textarea-over-highlighted-`<pre>`:

- textarea on top: `color: transparent; caret-color: var(--fg)`; pre behind:
  `overflow: hidden; pointer-events: none`; scroll synced on the textarea's
  `scroll` event. Highlighted HTML gets a trailing `\n` so scroll heights match.
- **Invariant: highlighted HTML text content must be character-identical to
  the source** — that's why hex colors get inline **swatches only in the
  output pane** (an inline-block `<i>` shifts columns) and **colored
  underlines in the editor** (`text-decoration-color` doesn't affect metrics).
- Toggle = `body.nohl` class (pre hidden, textarea text restored) — checkbox
  persisted as `go-styl-hl` in localStorage; applies to playground + tutorial.

Token colors are CSS vars (`--tk-*`) with dark + light palettes in
`index.html` under `prefers-color-scheme`.

## 2. Interactive tutorial (`playground/tutorial.js` + Tutorial tab)

### Structure

- Header gains a tab bar (Playground | Tutorial); playground-only controls
  hidden via `body[data-tab="tutorial"] .playctl`. Tab persisted
  (`go-styl-tab`); `index.html#tutorial` deep-links.
- Tutorial layout: lesson nav sidebar (✓ ticks) · prose column (highlighted
  snippets rendered through `stylHi`) · live editor + CSS output bench ·
  footer (reset / solution / open-in-playground / prev / next).
- Lesson data: `{id, title, nav, prose[], code, task, check(css, flat),
  solution, opts}`. `prose` mixes raw-HTML strings and `{code, lang}` blocks.
  `opts` merges into `goStyl.compile` opts — the theming lesson compiles with
  real `globals: {brand: '#7c3aed', radius: '10px'}` +
  `customProperties: ['brand','radius']`.
- Persistence: `go-styl-tut-done` (ids), `go-styl-tut-cur`, per-lesson drafts
  `go-styl-tut-draft-<id>`. Checkless lessons mark done on visit.
- **Node-requirable**: `module.exports = { LESSONS, init }` — enables the
  verification harness below.

### The 20 lessons

hello → nesting/& → variables (`?=`, scoping) → arithmetic/units →
colors → lists → mixins → functions → rest args (`args...`, `size(w, h = w)`)
→ conditionals (`dark()`, `unless`) → loops/ranges → interpolation →
built-ins tour → `@extend`/`$placeholders` → `@media`/`@supports` →
`@keyframes`/`@font-face` → brace syntax → `@import` (resolves live against
the bundled examples FS: `imports/_theme`) → runtime theming (go-styl
extension) → where-to-next.

## 3. Verification (the interesting part)

### Lesson harness (scratchpad `verify_lessons.js`)

`require`s `LESSONS`, compiles every starter **and** solution with the real
`cmd/styl` binary (theming lesson via `-D`/`-cssvar` derived from
`lesson.opts`; import lesson compiled in a dir with `examples/imports/`
copied in), asserts: starter compiles, starter does **not** already pass the
check, solution compiles **and** passes. 20/20 green after fixes.

### Compiler facts learned (tutorial drafts corrected by the compiler)

- **Syntax detection is per-file**: mixing indent + brace syntax in one sheet
  does not compile (`.hint` after a brace block → "unexpected character").
  An `@import`-ed partial may use the other style.
- Implicit return of a bare value **inside an `if` block** parses as a mixin
  call ("undefined mixin white") — use explicit `return` when branching.
  Top-level implicit return (`n * 2` as last expression) works.
- `& + &` only substitutes the **first** `&` (`.card + &`) — write `& + .card`.
- `darken(transparent, …)` errors: named `transparent` is an ident, not a color.
- `grayscale()` is **not** implemented (passes through unevaluated);
  `desaturate`, `spin`, `complement`, `invert`, `hue`, `mix`, `alpha` all work.
- `replace()` is `(pattern, replacement, string)` — Stylus arg order.
- `min(8px, 1rem)` computes numerically (→ `1rem`), ignoring mixed units — to
  emit literal CSS `min()`, wrap in `unquote('min(8px, 1rem)')`.
- `@extend` groups render one-line: `.menu, .crumbs {`.
- Bare `:hover` line attaches to parent like `&:hover` ✓; `1...4` exclusive ✓;
  color arithmetic `#222 + #111` → `#333` ✓; `index()` zero-based ✓;
  transparent mixins (`border-radius 4px` forwarding to a user mixin) ✓.

### Highlighter round-trip (scratchpad `verify_highlight.js`)

For every lesson snippet, prose block, repo example, and compiled CSS output:
strip tags/unescape from the highlighted HTML and require **exact equality**
with the input (the overlay alignment invariant). All clean.

### Browser drive (playwright-core + system Chrome)

`npm install playwright-core`, `chromium.launch({channel: 'chrome'})` —
no browser download needed. 25 steps, all pass: wasm ready (wait for
`#loading` detach), tokens + swatches in both panes, overlay text identity,
typing recompiles, positioned error surfaces, highlight toggle strips spans +
persists, tutorial nav = 20, lesson-1 task completes live (`color tomato` →
"✓ task complete" + nav tick), solution button, theming lesson emits
`:root {--brand}` + `var(--brand)` in-browser, imports lesson resolves in
wasm, open-in-playground, reload restores tab + progress, `#tutorial` hash.

- **Mobile bug caught by screenshot**: tutorial editor pane collapsed to ~0
  height at narrow widths (absolute-positioned overlay children give
  `.editor` no intrinsic height), text bleeding over the CSS OUT bar → fixed
  with `min-height` on bench panes + `overflow: hidden` on `.editor`.
- Console `favicon.ico` 404 is pre-existing noise.
- zsh gotcha: `echo ====` fails (`=word` expansion) — quote it.

## 4. Files touched

- `playground/highlight.js` **new** — tokenizers + overlay editor
- `playground/tutorial.js` **new** — 20 lessons + tutorial UI
- `playground/index.html` — tabs, overlay editors, token palettes, tutorial
  layout, highlight checkbox wiring
- `playground/README.md` — new pieces documented
- `.github/workflows/pages.yml` — stage `playground/*.js`
- `.claude/skills/verify/SKILL.md` **new** — build/serve/drive recipe
  (playwright-core + system Chrome, overlay invariant, lesson harness pattern)

## 5. Next-session pointers

- Wasm binary unchanged (API already had `globals`/`customProperties`) — no
  rebuild needed; Pages workflow rebuilds anyway on push.
- Possible follow-ups: line-error highlighting in the editor gutter (compile
  result carries `line`/`col`), shareable playground URLs (source in the
  hash), tutorial lesson deep-links (`#tut/<id>`), a Go highlighter for
  prose `go` blocks (currently escaped plain).
