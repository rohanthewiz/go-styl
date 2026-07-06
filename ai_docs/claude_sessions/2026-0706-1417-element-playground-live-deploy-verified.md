# Session: element playground built, deployed to Pages, live-verified

- **Date:** 2026-07-06 14:17
- **Session ID:** `7cd26f52-f304-47c8-bac8-b06d6a1b30b1`
- **Repos:** work done in `~/projs/go/element/` (branch `master`); session run
  from `~/projs/go/go-styl/` (continuation of
  `2026-0706-1250-playground-highlighting-tutorial.md`)
- **Element commits:** `a687a17` playground+tutorial (43 files) ·
  `93c4fdf` session notes · `13508f9` build.sh cold-cache fix
- **Detailed notes live in the element repo:**
  `element/ai_docs/claude_sessions/2026-0706-1357-playground-yaegi-wasm-tutorial.md`

> This session: reached across to the **element** repo and built the
> counterpart of go-styl's playground — a client-side playground + 14-lesson
> interactive tutorial where user **Go programs run in the browser** via the
> yaegi interpreter compiled to wasm — then deployed it to GitHub Pages and
> verified the live site end-to-end (29/29 checks at
> https://rohanthewiz.github.io/element/).

---

## 1. The core trick (element ≠ go-styl)

go-styl's playground compiles Stylus→CSS in wasm; element's API *is* Go, so
its playground embeds **yaegi** (Go interpreter) in the wasm binary along
with the **element + serr sources**, interpreted from a virtual GOPATH
(`fstest.MapFS` + `SourcecodeFilesystem`, `GoPath: "."` — fs.FS paths must
not start with `/`). What runs in the page is the real library: generics
(`ForEach`), the components package, debug mode. Stdout = output pane.

- Full yaegi stdlib symbols → 38 MB wasm; trimmed to 18 extracted packages
  (`symbols/gen.sh`) → **12 MB, 2.9 MB gzipped**, ~15–70 ms per run.
- `playground/wasm` is its own module (yaegi stays out of element's deps;
  `replace => ../..`). Same runner builds as a native CLI (`main_native.go`)
  for harnesses.
- 8 examples-menu snippets are each **compilable main packages** — CI-honest.

## 2. yaegi landmines (transferable knowledge)

- **v0.16.1 is broken for element**: composite literal assigned to a *named
  return value* returns a **zeroed struct** (exactly `element.New`'s shape —
  every Element lost its buffer). Fixed on yaegi master; pinned
  `v0.16.2-0.20260209085605-fcb76d1ece0c`.
- No `//go:embed` under interpretation → runner inlines the debug-table
  assets as quoted string literals at FS-build time.
- No Go 1.21 builtins `clear`/`min`/`max` (even master) → targeted rewrite +
  int shims injected into the interpreted `components` package. All
  transforms **fail loudly** if the source patterns drift.
- `for {}` in interpreted code **is cancellable** via `EvalWithContext` even
  single-threaded in wasm (done-channel checks at loop back-edges) — 5 s
  timeout verified in-browser.

## 3. UI + tutorial (mirrors go-styl's, adapted)

- New dependency-free **Go tokenizer** and **HTML tokenizer** (CSS tokenizer
  ported for `<style>` bodies) in `eleHi`; same overlay-editor
  **character-identity invariant**; output pane gains **html/preview**
  sub-tabs (preview = `sandbox="allow-scripts"` iframe fed `srcdoc`).
- 14 live-checked lessons: hello/execution-order → attributes → single tags
  → text (`T`/`F`, no-escaping caveat) → ForEach → conditionals →
  components → full pages → Table → UI kit → debug mode ("No element
  concerns found.") → pooling/caching (Cached render-counter) → pretty →
  where-to-next (links back to go-styl's playground).
- **JS template-literal trap:** `\n` in lesson Go strings must be `\\n`
  (bit the perf lesson); no backticks or `${` in lesson code, ever.
- Boot gotcha fixed (also latent in go-styl!): call `switchTab(tab)`
  unconditionally at startup — go-styl's `if (tab === 'tutorial')` pattern
  leaves `body[data-tab]` unset when booting to the play tab.

## 4. Verification ladder (all green)

1. **Lesson harness** (node → native runner CLI): starter runs / starter
   does NOT pre-pass / solution runs AND passes — 14/14.
2. **Highlighter round-trip**: strip-tags+unescape == input for every lesson
   snippet, prose block, example, and interpreter output — 92/92.
3. **Local browser drive** (playwright-core + system Chrome, 29 steps).
4. **Live browser drive** against https://rohanthewiz.github.io/element/ —
   same 29 steps, all pass.

## 5. Deploy — and the one CI failure

- `element/.github/workflows/pages.yml` (element's first workflow): trigger
  branch is **master** (element's default — go-styl uses main; caught after
  committing with `[main]`), `go-version-file: playground/wasm/go.mod`
  (go 1.26 — symbols extracts carry a go1.26 build tag).
- **First deploy failed**; no `gh` CLI on this machine and the Actions log
  API needs auth, so the failure was *reproduced locally with a cold module
  cache*: `GOMODCACHE=$(mktemp -d) ./build.sh` → `go list -m -f '{{.Dir}}'
  github.com/rohanthewiz/serr` prints **empty** when the module isn't in
  the cache → `cp /*.go`. Warm local caches hide this class of bug.
  Fix (`13508f9`): `go mod download` first + hard fail if dir is empty.
  Rerun green; Pages live.
- Useful API polling without gh:
  `curl -s 'https://api.github.com/repos/<o>/<r>/actions/runs?per_page=1'`.

## 6. go-styl follow-ups spotted from this work

- The `switchTab` boot gotcha (§3) exists in go-styl's `index.html` too —
  harmless today (nothing styles `body[data-tab="play"]`), worth fixing on
  next touch.
- go-styl's `build.sh` has no cold-cache dependency staging (it compiles
  everything, no module-dir copying) — not affected.
- Element's tutorial closes the loop by linking to go-styl's playground;
  go-styl's "where to next" lesson could reciprocate with a link to
  element's (https://rohanthewiz.github.io/element/#tutorial).

## 7. Next-session pointers (element playground)

- Web-worker runner (kill hung runs instantly; keep typing responsive),
  editor gutter markers from `{line,col}`, shareable URLs (source in hash),
  lesson deep-links, forms lesson, wasm gofmt.
- Adding a lesson that imports a new stdlib package ⇒ regenerate
  `symbols/` (wasm size is the tax).
- If element adopts `min`/`max`/`clear`/`embed` in new files, extend
  `runner.elementCompat` (it fails the virtual-FS build with a pointed
  error).
