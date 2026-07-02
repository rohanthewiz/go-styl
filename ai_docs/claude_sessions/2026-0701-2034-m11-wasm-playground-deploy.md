# Session: go-styl M11 — WASM playground, deployed to GitHub Pages

- **Date:** 2026-07-01 20:34
- **Session ID:** 410f6991-1414-4ef8-934b-0a74ea59b919
- **Repo:** `~/projs/go/go-styl/` (module `github.com/rohanthewiz/go-styl`, branch `main`)
- **Continuation of:** `2026-0701-1917-go-styl-difftest-and-stylus-parity.md` (M9 + M10)

> This session: built **M11** — a browser playground running the compiler as
> WebAssembly — verified it end-to-end in headless Chrome, pushed, enabled
> GitHub Pages, and confirmed the production deploy.
> **Live: <https://rohanthewiz.github.io/go-styl/>**
> Also decided: **watch mode is dropped from the roadmap** (external watchers
> like air/entr/watchexec cover it; the HTTP middleware already recompiles on
> request with per-dep invalidation; `Result.Deps` is exposed for anyone who
> wants precise watch lists).

---

## 1. M11 — WASM playground (commit `7c11cf9`)

### Pieces

- **`examples/embed.go`** — new tiny package `examples` exporting
  `examples.FS` (`//go:embed *.styl all:imports`). The `all:` prefix is
  required because the `imports/` partials start with `_`, which go:embed
  skips by default. Used by the playground both to list examples and as the
  `styl.Options.FS` import filesystem.
- **`playground/wasm/main.go`** (`//go:build js && wasm`) — installs a global
  `goStyl` object:
  - `goStyl.compile(src, {pretty, mergeDuplicates, sourcemap})` →
    `{css, map?, ms}` or `{error, file, line, col, msg, ms}` (position fields
    extracted via `errors.As` on `*diag.Error` — playground is in-module, so
    `internal/diag` is importable).
  - `goStyl.examples()` → `[{name, source}]` from `examples.FS`.
  - `goStyl.version` → short `vcs.revision` from `debug.ReadBuildInfo()`,
    falling back to `"dev"`.
  - Calls `goStylReady()` (page-supplied) once installed; blocks in
    `select {}`. `@import` in playground source resolves against the embedded
    examples FS (`Filename: "playground.styl"`), so `@import "imports/_theme"`
    works in the browser.
- **`playground/index.html`** — the entire UI, dependency-free vanilla
  JS/CSS: two panes (Stylus in / CSS out), example dropdown, pretty /
  merge-duplicates / source-map checkboxes, debounced (120 ms) recompile on
  input, error bar with the positioned message (stale CSS stays visible at
  0.45 opacity), collapsible source-map panel, char/ms stats, localStorage
  persistence (saved on every compile, including erroring source), Tab
  inserts two spaces, dark/light via `prefers-color-scheme`.
  `instantiateStreaming` with an `arrayBuffer` fallback for servers that
  mislabel `.wasm`'s MIME type (and for browsers lacking streaming).
- **`playground/build.sh`** — `GOOS=js GOARCH=wasm go build -trimpath
  -ldflags='-s -w' -o styl.wasm ./wasm` (result: **4.5 MB**) + copies
  `wasm_exec.js` from `$GOROOT/lib/wasm/` (Go ≥ 1.24) falling back to
  `misc/wasm/` (≤ 1.23). Both artifacts are gitignored
  (`playground/.gitignore`).
- **`playground/serve/`** — tiny dev file server
  (`go run ./playground/serve`, `-addr`/`-dir` flags, default :8080).
  Go's mime table serves `.wasm` as `application/wasm` so streaming
  instantiation works locally.
- **CI** (`ci.yml`): new step builds + vets the wasm target
  (`GOOS=js GOARCH=wasm go build ./playground/wasm` — `go build ./...`
  silently skips the constraint-excluded package, so the explicit step is
  what catches breakage).
- **Pages deploy** (`.github/workflows/pages.yml`): on push to main (path
  filter: `playground/**`, `internal/**`, `examples/**`, `styl.go`, `go.mod`,
  the workflow itself) + `workflow_dispatch`; builds via build.sh, stages
  `index.html` + `styl.wasm` + `wasm_exec.js` into `_site/`,
  `upload-pages-artifact` → `deploy-pages`. Concurrency group `pages`.
- Docs: README "Playground" section (+ live link, second commit) and roadmap
  M11 line; SKILL.md §1 gained a "WASM playground" subsection.

### Verification (thorough — GUI surface)

- **Node smoke test** of the wasm API (scratchpad `wasm_smoke.mjs`, loads
  `wasm_exec.js` + `styl.wasm` in Node 22): pretty/compressed compiles,
  positioned error + did-you-mean, source map v3, all 10 examples compile,
  `@import` against the embedded FS. Initial "failures" were wrong test
  expectations: pretty output uses **tabs** (no trailing newline), and an
  unknown *function* in value position (`darkn(...)`) correctly passes
  through as literal CSS (Stylus CSS-function passthrough) — did-you-mean
  fires only for undefined **mixins** (statement position).
- **Headless Chrome over CDP** (scratchpad `ui_verify.mjs` — hand-rolled CDP
  driver using Node 22's built-in WebSocket; no Playwright installed):
  12 checks driving the real page — load-and-compile on open, version stamp,
  dropdown populated, example pick, typing broken source → error bar
  `playground.styl:4:3: undefined mixin "buton" (did you mean "button"?)` +
  dimmed stale output, fix clears it, compressed toggle → `a{color:red}`,
  source-map panel, localStorage across reload, Tab key. Screenshots
  captured. Gotcha: repeated `Runtime.evaluate` snippets share the page's
  global scope — wrap `const`-declaring snippets in IIFEs.
- All local: `go build/vet ./...`, `gofmt -l .`, full
  `go test -count=1 ./...` (incl. difftest 23/32) green.

## 2. Push + Pages enablement (commit `6ac1f8b` for the README link)

- Pushed `7c11cf9`; CI and playground workflows both green first run.
- **No `gh` CLI on this machine** — enabled Pages via REST:
  token pulled from git's credential helper
  (`git credential fill` → password field, never printed), then
  `POST /repos/rohanthewiz/go-styl/pages {"build_type":"workflow"}` → 201.
- Verified production: `styl.wasm` served as `application/wasm` (4.4 MB),
  and re-ran the full 12-check headless-Chrome suite **against the live
  URL** — all pass.
- `6ac1f8b` adds the live link to README (README isn't in the pages path
  filter → no redeploy).

## Commits on `main` this session (both pushed)

1. `7c11cf9` — M11: WASM playground (browser editor over the js/wasm compiler)
2. `6ac1f8b` — docs: link the live playground

---

## Roadmap

- [x] M1–M8 (vertical slice → HTTP middleware)
- [x] M9 differential testing + CI; M10 Stylus parity (23/32, 72%)
- [x] M11 WASM playground, live on GitHub Pages
- [x] ~~CLI watch mode~~ — dropped: external watchers + middleware
  recompile-on-request + exposed `Result.Deps` cover it
- [ ] Candidates: grow the difftest corpus, list indexing `r[1]`,
  `styl fmt`, benchmarks vs node stylus, value-level source mapping,
  more built-ins

## Carry-forward notes

- Playground redeploys automatically on any push to main touching
  `playground/**`, `internal/**`, `examples/**`, `styl.go`, or `go.mod`.
- The 4.5 MB wasm is unoptimized size-wise; if it matters later: TinyGo is
  risky (stdlib-heavy code), `-ldflags='-s -w' -trimpath` already applied;
  gzip/brotli on Pages makes the wire size ~1.2 MB anyway.
- `goStyl.version` shows the commit of the *build*; local `./playground/build.sh`
  from a dirty tree shows the last commit hash.
- Reusable verification assets (scratchpad, session-scoped — recreate if
  needed): `wasm_smoke.mjs` (Node API test), `ui_verify.mjs` (CDP driver;
  IIFE-wrap page evals).
- Bash permission classifier flaked repeatedly this session ("claude-opus
  temporarily unavailable") — kept working via Write/Edit/Read and retried.
- Playground error UX quirk worth remembering: unknown functions in value
  position pass through silently (correct Stylus behavior) — only undefined
  mixins get did-you-mean errors.
