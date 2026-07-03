# go-styl — killer feature ideas (post-Stylus-parity)

*Brainstormed 2026-07-03, after M12. The framing insight: go-styl is an
**in-process compiler that costs microseconds**. Node Stylus is a black box you
run at build time; go-styl can run at request time, inside your app, with data
from your app. The killer features live in that gap — plus one in the "Stylus
tooling is dead" gap.*

Ranked:

## 1. Runtime theming — inject Go values as Stylus variables ✅ SHIPPED as M13

`styl.Options{Globals: map[string]any{"primary": "#0af", "radius": "8px"}}`.
Per-tenant, per-user, per-A/B-arm stylesheets compiled on the fly — at 3–30µs a
sheet, you can afford to compile *per request* and cache by variable set. A
SaaS whitelabel story no Node pipeline can touch (they bake themes at build
time or fall back to raw custom properties). Pairs perfectly with `stylserve`:
`/css/app.css?tenant=acme`. Cheapest to build (the evaluator already has a
scope to pre-seed) and the most unique.

Natural companion: an **`EmitCustomProperties` mode** that compiles designated
variables to `--vars` under `:root` instead of inlining them — Stylus predates
custom properties, and bridging the two (compile-time math *over*
runtime-themable tokens, auto dark-mode blocks) modernizes the language itself.

## 2. Critical CSS per response — the `element` synergy

Because HTML generation (element) and CSS compilation both run in-process, we
can do what the Node world needs Puppeteer for: render the page, collect the
class names actually used, and emit only the matching rules inline in
`<head>`. A `styl.Prune(sheet, usedSelectors)` API plus an rweb/element hook.
Zero-render-blocking CSS as a middleware flag. Genuinely novel for Go SSR.

## 3. Scoped component styles + typed class names (CSS Modules for Go)

Two halves of one feature:

- `styl.Component(src)` → hashed class names (`.card_x3f2`) plus a name→class
  map. Component-scoped styles for server-rendered Go, no build step.
- A `styl gen` codegen (run via `go generate`) that emits a Go package of
  constants for every class/variable in a `.styl` — so `b.Div(css.Card)` in
  element is typo-proof and refactorable. Typed-css-modules, but for Go.

## 4. A Stylus LSP — single static binary

Stylus editor tooling is essentially abandoned upstream. We already have a
real lexer/AST with positions, positioned errors, and did-you-mean. An LSP
server gives: diagnostics as you type, completion for variables/mixins/
built-ins, go-to-definition across `@import`, hover showing the *computed*
value, color swatches. `go install .../cmd/styl-lsp` and every VS Code/Neovim
Stylus user is a potential adopter. `styl fmt` and a linter fall out of the
same AST work.

## 5. Stylus → modern CSS migration tool

The exit-ramp play: legacy Stylus codebases want *off* Stylus now that vanilla
CSS has native nesting, custom properties, and `color-mix()`. A `styl migrate`
that outputs modern CSS *preserving structure* (nesting kept as nesting,
variables as `--vars` where possible, mixins flagged for manual review) would
capture an audience that no longer wants a preprocessor at all — and go-styl
is the only Stylus implementation healthy enough to build it on.

## 6. Safe multi-tenant theme compilation

Pure Go + the existing recursion/selector caps means we're most of the way to
safely compiling *user-submitted* themes (Discourse-style theming). Add a
`Sandbox` option: no OS filesystem, import allowlist, output-size cap, compile
timeout. Embedding Node Stylus for this is a security nightmare; for go-styl
it's a hardening pass.

## Honorable mention: dev-mode live reload

SSE endpoint in `stylserve` (rweb's SSE hub fits), CSS hot-swap without page
refresh, and a Vite-style in-browser error overlay using the positioned
errors.

## Suggested order

**#1 (Globals + custom-properties emission)** first — smallest lift, biggest
differentiation, feeds #2 and #6 naturally. #4 (LSP) is the biggest adoption
lever but the biggest lift; #3's codegen half is a weekend-sized win in
between.
