# Session: go-styl M13 — runtime theming (Globals + CustomProperties)

- **Date:** 2026-07-03 08:46
- **Session ID:** 72070f92-24b5-4760-98ac-90d856644486
- **Repo:** `~/projs/go/go-styl/` (module `github.com/rohanthewiz/go-styl`, branch `main`)
- **Continuation of:** `2026-0701-2034-m11-wasm-playground-deploy.md` (M11; M12
  benchmarks landed between sessions)

> This session: brainstormed post-parity **killer features** (saved to
> `ai_docs/killer_features.md`), then built #1 — **M13 runtime theming**:
> `Options.Globals` (Go values injected as Stylus variables) and
> `Options.CustomProperties` (theme tokens emitted as `:root` `--vars`,
> direct references compiling to `var(--name)`). Committed as `9e0ed3f`.

---

## 1. Killer-features brainstorm (`ai_docs/killer_features.md`)

Framing: go-styl is an **in-process compiler that costs microseconds** — the
killer features live where Node toolchains structurally can't go (request-time
compilation with app data), plus the "Stylus tooling is dead" gap. Ranked:

1. **Runtime theming** (Globals + custom-properties emission) — ✅ shipped this session
2. **Critical CSS per response** — render HTML (element), collect used classes,
   emit only matching rules inline; `styl.Prune(sheet, usedSelectors)` + rweb hook
3. **Scoped component styles + typed class names** — `styl.Component(src)`
   (hashed classes + name map) and a `styl gen` codegen emitting Go constants
   for classes/variables (typo-proof `b.Div(css.Card)` in element)
4. **Stylus LSP** (single static binary) — diagnostics, completion,
   go-to-def across @import, hover computed values, color swatches; `styl fmt`
   + linter fall out of the same AST work
5. **Stylus → modern CSS migration tool** — exit ramp for legacy codebases
   (native nesting, `--vars`, `color-mix()`)
6. **Safe multi-tenant theme compilation** — `Sandbox` option: no OS fs,
   import allowlist, output cap, timeout
- Honorable mention: dev-mode live reload (SSE in stylserve + error overlay)

## 2. M13 implementation (commit `9e0ed3f`)

### `Options.Globals map[string]any` (public + eval)

- `internal/eval/globals.go` — `seedGlobals` runs before the sheet executes,
  binding into the **root scope** in **sorted-name order** (deterministic).
  Conversion (`globalValue`): string → `parser.ParseExpr` + `evalExpr` (full
  expressions work: `"#0af"`, `"10px"`, `"1px solid red"`,
  `"darken(#0af, 10%)"`); int/uint/float variants → unitless `Number`;
  bool → `Bool`; nil → `Null`; anything else errors.
- Error shape: `global "x": <msg>` — position deliberately **stripped**
  (`globalErr` pulls `diag.Error.Msg`) since the mistake is in Go code, not
  the sheet; a `file:line:col` prefix would mislead.
- Semantics: sheet `=` overrides a global; sheet `?=` is a default the global
  overrides (`scope.Has` sees the seeded value → `?=` skips). Globals are
  visible in imported sheets (imports share the root scope).

### `Options.CustomProperties []string` (public + eval + value)

- `internal/value/var.go` — new `value.Var{Name, Inner}`: `CSS()` renders
  `var(--name)`; `Deref(v)` unwraps chains to the concrete value;
  `TypeName()` delegates to Inner. `Truthy` derefs internally (ops.go).
- `internal/eval/customprops.go` — `wrapVar` wraps assignments **only when
  the binding lands in the root scope** (`scope == ev.rootScope`) and the
  name is listed; rule-local vars with a listed name stay concrete (no broken
  `var()` without a `:root` entry). `customPropsRule` builds the leading
  `:root` rule after evaluation: final root-scope value per listed name, list
  order, undefined names skipped, `Pos{1,1}` so source maps stay valid.
- **Lazy-deref sites** (where the concrete value is used):
  - `evalUnary`, `evalBinary` (arith/range/comparisons/EQ — but *after* the
    `b.Literal` SlashList branch), builtin call args in `evalCall`,
    `iterItems` (for-loops), `evalString` (m4 interpolation — selectors,
    `{...}`, calc), `evalAtVars` (m5 media-query bare vars).
- **`var()` survives** (valid CSS at runtime): direct refs, items inside
  value lists (`1px solid var(--c)`), literal-slash shorthand
  (`font: 14px/var(--lh)`), unknown-function passthrough
  (`translateX(var(--x))`), and **user function/mixin arguments** — params
  bind the wrapper, body ops deref lazily, so `bordered(primary)` emits
  `border: 1px solid var(--primary)` while `double(pad)` computes.

### Plumbing

- `styl.go`: both fields on `Options`, passed at all three eval call sites
  (`Compile`/`Build`/`CompileMap`).
- `cmd/styl`: `-D name=value` (repeatable, value is a Stylus expression) and
  `-cssvar name` (repeatable), both also on the `-sourcemap` path.
- `stylserve`: `Options.Globals` + `Options.CustomProperties` — **fixed at
  engine creation** so the cache stays valid; per-tenant = one engine per
  theme. (Per-request globals + variable-set-keyed cache = possible follow-up.)
- Docs: README "Runtime theming" section + Options table + CLI + roadmap M13;
  SKILL.md §1 Options table rows, theming blurb, CLI flags, stylserve list.

### Verification

- `m13_test.go`: table tests for Globals (12 cases: conversions, expression
  globals, `?=`/`=` precedence, quoted strings), global errors (3), custom
  properties (18: every keep-var / use-concrete site), the flagship
  Globals+CustomProperties combo, pretty output, globals-in-imports.
  `stylserve` gained `TestGlobalsAndCustomProperties`.
- Full gate green: `go build/vet ./...`, `gofmt -l` clean,
  `go test -count=1 ./...` **including difftest** (no parity regressions —
  both options are additive extensions), `GOOS=js GOARCH=wasm go build/vet
  ./playground/wasm`, CLI end-to-end (`-D`/`-cssvar`, sourcemap with `:root`
  block mapped at 1:1).

### Gotchas found

- `margin -g` renders `-g` — dash-prefixed **identifier**, not unary minus
  (pre-existing, matches Stylus); `-(g)` is the unary form.
- `@media (min-width: bp * 20)` doesn't evaluate arithmetic (pre-existing
  documented limitation; `{bp * 20}` interpolation works and derefs).
- A `GOOS=js go build` without `-o` dropped a stray `wasm` binary in the repo
  root — deleted before commit.

## Commits on `main` this session

1. `9e0ed3f` — M13: runtime theming — Globals + CustomProperties (**not pushed yet**)

---

## Roadmap

- [x] M1–M12 (vertical slice → benchmarks)
- [x] **M13** Runtime theming: `Globals` + `CustomProperties`
- [ ] Next candidates (from `ai_docs/killer_features.md`):
  - **`styl gen` codegen** (killer feature #3, second half) — Go package of
    typed class/variable constants from a `.styl`; weekend-sized, big element synergy
  - **Playground exposure of M13** — globals/custom-props in the wasm API + UI
  - **Critical CSS** (#2): `styl.Prune(css, usedSelectors)` + element/rweb hook
  - **Scoped styles** (#3 first half): `styl.Component` with hashed class names
  - **LSP** (#4, biggest lift) / **migration tool** (#5) / **sandbox** (#6)
  - Per-request globals in stylserve with variable-set-keyed caching

## Carry-forward notes

- Push of `9e0ed3f` pending — pushing will trigger the Pages redeploy
  (path filter matches `internal/**`, `styl.go`); playground is unaffected
  functionally (doesn't use the new options yet).
- `value.Var` invariant: anything that type-switches on concrete value types
  must `value.Deref` first; new builtins get this free (deref happens at
  `evalCall`), but any **new operation site** in eval must deref explicitly.
- CustomProperties names are emitted verbatim after `--` (no sanitization);
  document-level assumption: names are CSS-ident-safe.
- Reassigned listed vars: every direct ref renders the same `var(--name)`;
  the `:root` block carries the **final** root-scope value (documented as
  "theme tokens should be assigned once").
