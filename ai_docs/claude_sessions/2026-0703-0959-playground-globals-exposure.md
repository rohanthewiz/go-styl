# Session: playground exposure of M13 Globals/CustomProperties

- **Date:** 2026-07-03 09:59
- **Session ID:** `566497df-3c30-4379-8695-c55a4f789faa`
- **Repo:** `~/projs/go/go-styl/` (module `github.com/rohanthewiz/go-styl`, branch `main`)
- **Continuation of:** `2026-0703-0920-m14-styl-gen-codegen.md` (M14 `styl gen`)

> This session: exposed M13 runtime theming in the wasm playground —
> `goStyl.compile` gained `{globals, customProperties}` opts and the page
> gained a collapsible Go-side options panel. Committed `eb3252c`, pushed
> (Pages redeploy ships it live). Next up: **M15 Critical CSS**
> (`styl.Prune` + element/rweb hook).

---

## 1. Playground exposure (commit `eb3252c`, pushed)

### Wasm API (`playground/wasm/main.go`)

- `goStyl.compile(src, opts)` now reads `opts.globals` (JS object) and
  `opts.customProperties` (JS array):
  - `toGlobals`: string → Go string (parsed as Stylus expression by M13),
    number → float64, boolean → bool, null → nil; other types dropped.
    Empty map → nil (option untouched).
  - `toStrings`: array → []string, non-string entries skipped.
- **Guard gotcha:** arrays are `js.TypeObject`, but so is any object — and
  `js.Value.Length()` on a non-array *panics* (undefined `.length` →
  `Int()` on non-number). Guard with
  `js.Global().Get("Array").Call("isArray", v).Bool()`, not a type check.
  `null` is `js.TypeNull` (not TypeObject) so a null `globals` is skipped
  by the object check for free.
- Doc comment + `playground/README.md` updated with the new signature.
- Modernized index loops to `for i := range n` (Go 1.22+ rangeint lint).

### UI (`playground/index.html`)

- New `details#globalsbox` panel at the bottom of the **source pane**
  (mirrors `#mapbox` at the bottom of the output pane). One directive per
  line:
  - `primary = #0af` → `Options.Globals` entry (value is a Stylus expr)
  - `--accent = #e91e63` → global **and** listed in `CustomProperties`
  - `--primary` (bare) → CustomProperties only (exposes a sheet var)
  - `//` comments: full-line and trailing (stripped by `(^|\s)\/\/.*$` —
    `url(http://…)` survives because its `//` isn't whitespace-preceded).
- Parser regex: `^(--)?([A-Za-z_][-\w]*)\s*(?:=\s*(\S.*?)\s*)?$`; garbage
  lines and bare names without `--` are silently ignored (Go-side compile
  errors surface in the existing error box).
- Summary bar shows a live count (`· 2 globals · 1 custom`).
- Persistence: `localStorage['go-styl-globals']`; panel auto-opens on load
  when saved content is non-empty. Input recompiles via the existing
  debounced `schedule()`.
- CSS: `#globals` textarea overrides the global `textarea` rule by id
  specificity (the shared rule's `flex:1` is inert inside `<details>`).

### Verification

- Gate green from repo root: build/vet/gofmt (native + `GOOS=js`),
  `go test -count=1 ./...` incl. root pkg + difftest.
- **Node e2e against the real `styl.wasm`** (13 checks): wasm_exec.js loads
  fine under Node 22 via `eval`; covered `:root` emission + `var()` refs,
  non-listed globals inlined, number/bool/null globals, `=` beats global /
  `?=` yields (M13 precedence), sheet-var-as-custom-property, bad global
  expression → error object (no crash), absent/empty/null/non-array opts.
- **UI logic tested headlessly**: inline `<script>` extracted from
  index.html → `new vm.Script` syntax check + `goSideOptions` unit-tested
  in a vm context with a stubbed `$` (10 cases).
- **Headless Chrome screenshots** (real browser, real wasm): collapsed
  panel renders; a seeded copy (`_seeded_test.html`, deleted after) showed
  `:root { --primary: #e91e63 }` + `background: var(--primary)` — i.e.
  sheet `=` overriding the global while staying runtime-themable.

### Gotchas (re)found

- **Shell cwd persists across tool calls**: a leftover `cd playground`
  made `go build ./...` fail with `build output "serve" already exists and
  is a directory` (Go 1.26 writes main-package binaries; `serve/` dir
  collides) and silently shrank `go test ./...` to playground packages.
  Run gates explicitly from repo root.
- `darken(#e91e63, 10%)` = `#d81557` (hand-computed `#d4145a` was wrong —
  trust the compiler in test expectations).
- `go run ./playground/serve` left on :8080 — kill via `lsof -ti :8080`.

## Commits on `main` this session

1. `eb3252c` — playground: expose M13 Globals/CustomProperties in wasm
   API + UI (**pushed**; Pages redeploy ships the panel live)

---

## Roadmap

- [x] M1–M14 + playground exposure of M13
- [ ] **M15 Critical CSS (#2) — NEXT, started this session**:
  `styl.Prune(css, usedSelectors)` + element/rweb hook; reuse
  `css.CollectNames`' selector scanner for used-name matching
- [ ] Scoped styles (#3 first half): `styl.Component` hashed class names →
  feed the same `GoSource` renderer
- [ ] LSP (#4) / migration tool (#5) / sandbox (#6) / live reload
- [ ] Per-request globals in stylserve with variable-set-keyed caching

## Carry-forward notes

- Playground UI now has **two** persisted localStorage keys:
  `go-styl-src`, `go-styl-globals` — bump/namespace both if the format
  ever changes.
- The globals panel passes **strings** for all values; the wasm shim's
  number/bool/null paths exist for direct JS API users.
- Playground panel syntax (`--name = value`) is playground-only sugar —
  don't let it leak into library docs as if it were Stylus syntax.
- M15 design note (from killer_features.md): Prune should take the
  *rendered CSS or resolved nodes* + a used-selector set; evalNodes already
  returns the evaluator for compile+inspect hooks (M14 refactor).
