# Session: go-styl differential testing (M9) + Stylus parity fixes (M10)

- **Date:** 2026-07-01 19:17
- **Session ID:** 27843f9e-9110-4520-93d6-7461891fc6fe
- **Repo:** `~/projs/go/go-styl/` (module `github.com/rohanthewiz/go-styl`, branch `main`)
- **Continuation of:** `2026-0701-1623-go-styl-m7-m8.md` (M7 + M8)

> This session: built **M9** (differential testing vs the reference Node.js
> stylus compiler, with a known-diffs ratchet and CI), then used its findings
> to fix all four **[compat-bug?]** items and close all five **[compat-gap]**
> features (**M10**). Compatibility score: **11/30 → 23/32 (72%)**. Every
> remaining baseline entry is a deliberate go-styl extension or a
> reference-stylus failure.

---

## 1. M9 — differential testing harness (commit `3777297`)

### Design

`difftest/` compiles every corpus file with **both** go-styl
(`styl.CompileFile`, compressed) and **npm stylus 0.64.0** (one node process
for the whole batch via `compile_stylus.mjs`, reading `[{id,path}]` JSON on
stdin → `{id:{css}|{error}}` on stdout), normalizes both outputs, and compares.

- **Corpus** = `examples/*.styl` + `testdata/*.styl` + `difftest/corpus/*.styl`
  (imports/ subdirs are partials, not compiled directly). Corpus grew to 32
  files: 14+ targeted stylus-canonical files were added because examples/
  testdata skew toward go-styl extensions.
- **Normalizer** (`normalize_test.go`) erases formatting-only differences —
  applied identically to both sides so equality is preserved, and quoted
  string contents are untouched (`mapOutsideStrings` walker):
  newline removal (stylus keeps one after @charset/@import even compressed),
  `;}`→`}`, `> ~` combinator spacing, ` + ` collapsed **only at paren depth 0**
  (calc's `+` spacing survives), one space before `!important`, leading-zero
  decimals (`.5`→`0.5`), named colors→hex, hex→lowercase 6/8-digit long form.
- **Ratchet** (`known_diffs.txt`): `<path> <note>` lines. Test FAILS when an
  unlisted file diverges (regression) AND when a listed file starts matching
  ("now agrees — remove it"), so the score only climbs.
  `UPDATE_KNOWN_DIFFS=1 go test ./difftest` regenerates mechanically,
  preserving notes and the leading header comment block.
- Outcome categories: match / both-error (= agreement) / diff / go-error /
  stylus-error; `diffExcerpt` pinpoints first divergence with context.
- Skips when node or `difftest/node_modules/stylus` is missing
  (`npm install --prefix difftest`; package.json pins stylus 0.64.0, lockfile
  committed, node_modules gitignored).
- **CI**: `.github/workflows/ci.yml` (repo's first workflow) — `test` job
  (build/vet/`go test ./...`) + `difftest` job (setup-node 22, `npm ci`,
  `go test -v -run TestDifferential ./difftest`).
- README: new "Compatibility with reference Stylus" section; roadmap gained
  M7/M8/M9 lines (they were missing).

### Initial findings (score 1/17 raw → 4/17 normalized → baseline of 19)

The harness immediately classified every divergence: 4 `[compat-bug?]`
(for-loop binding order, darken() math, mix rounding, `/` in property values),
6 `[compat-gap]` (`**`, ranges, color arithmetic, implicit returns,
transparent mixins — plus later found: `spin()` missing, zero-unit strip),
go-styl extensions (`{expr}` in @media/calc/quoted strings, single-line
`f(x)=expr`, lone `{expr}` values, `+mixin()` without block), stylus
auto-prefixing @keyframes, and one genuine reference-stylus internal crash
(examples/05 builtins mix).

## 2. Compat-bug fixes (commits `f69b929`, `912481d`)

All semantics **probed empirically** against npx stylus before implementing.

### `f69b929` — for-loop binding + color math

- **for a, b in list**: stylus binds VALUE first, INDEX second; go-styl had it
  reversed. One-line swap in `parseFor` (`f.Value, f.Index = vars[0], vars[1]`);
  examples/04 + SKILL.md updated to `for shade, idx in …`.
- **lighten/darken/saturate/desaturate** now implement stylus `adjust()`:
  `%` amounts are relative — positive lightness scales into headroom
  (`l += (1−l)·pct`), everything else scales the component (`l −= l·pct`,
  `s *= 1±pct`); **unitless amounts stay absolute percentage points**.
  Restructured `adjustHSL` to pass the raw `*value.Number` (unit matters);
  new `hslDelta(component, cur, amt, sign)` helper.
  darken(#e91e63, 10%) = #d81557 ✓; darken(#e91e63, 10) = #c1134e ✓.
- **mix/tint/shade floor** channels (stylus arithmetic rounds, but mix
  floors): `blend()` uses math.Floor; mix(#fff,#000) = #7f7f7f.
- **Bonus:** `spin(color, deg)` added (was missing entirely — emitted literal).
- Goldens: m3.css/.min.css fill → #7f7f7f; m3_test mix/tint updated + spin case.

### `912481d` — literal slash in property values

Stylus: in a declaration value at paren depth 0, `/` does NOT divide — the
operands still evaluate but render joined (`line-height x/2` → `20px/2`,
`font 14px/1.5 Arial` passes through). Parens/assignments/conditions/call args
divide.

- Parser: `exprParser` gains `propValue` + `depth`; declaration values parse
  via new `parsePropExpr`; depth-0 slashes set `Literal: true` on `ast.Binary`
  (depth tracked in LPAREN primary + parseArgs).
- Eval: `b.Literal` → new `value.SlashList{L,R}` (renders `L.CSS()/R.CSS()`,
  TypeName "literal").
- examples/01 changed to `(base / 2)` (its intent was real division);
  slash corpus file + 6 m6_test cases. Fuzz 30s clean.

## 3. M10 — compat gaps (commit `194738c`)

- **`**` exponent**: probe showed it is NOT the usual right-assoc/higher-prec —
  stylus puts it on the **multiplicative level, left-associative**
  (`2*3**2`=36, `2**3**2`=64); unary binds tighter (`-2**2`=4); left unit kept
  (`2px**2`=4px). token.POW via twoCharOps; `infixBP` 60; `Arith("**")`.
- **Ranges**: `1..3` inclusive, `0...3` exclusive, descending works; ranges
  are first-class lists (`r = 1..3`, `length(r)`=3). token.DOTDOT (`...` was
  already lexed for rest params; `scanNumber` already stopped before `..`);
  infix BP **45** (below arithmetic, above comparisons); `evalRange`
  materializes with step ±1, left unit wins, **capped at 65536 elements**
  (fuzz safety — `1..1e9` errors).
- **Color arithmetic**: `value.ColorArith` — `+ - * /` channel-wise
  color⊕color and color⊕number (RGB only, alpha kept), rounded (existing
  `roundByte`) and clamped. **Alpha follows stylus RGBA.operate** (probed):
  `+` adds clamped; `-` keeps LEFT alpha when rhs is opaque (a==1), else
  subtracts; `*`/`/` keep left alpha. (First attempt subtracted alpha always →
  `brand - #111111` wrongly transparent.)
- **Implicit returns**: new `ast.ExprStmt` (Pos + stmtNode + eval case).
  Parser fallback in parseStatement: if not declaration-shaped, or the
  property-value parse fails, try whole line as expression → ExprStmt.
  Eval sets `ctx.ret` WITHOUT `returned` → last expression evaluated wins;
  explicit `return` still short-circuits; `invoke` already returned fctx.ret.
  Bare-ident last lines (`n`): fallback in `evalMixinCall` — unknown mixin
  with no args that names a variable sets ctx.ret.
- **Transparent mixins**: in the Declaration eval case, a property naming a
  scope function (user-defined only; builtins NOT intercepted) becomes a
  MixinCall; `transparentArgs` unpacks space- OR comma-list items into
  separate args (probed: `m 1px 2px` ≡ `m 1px, 2px` ≡ m(1px, 2px)).
  **Self-guard**: `execCtx.mixin` (set in `invoke` from cl.Def.Name,
  propagated through evalRuleSet children) — inside a mixin's body its own
  name stays a plain property, so `border-radius(n)` emitting
  `border-radius n` doesn't recurse (stylus rule).
- **Zero-unit strip** (found via ranges corpus): compressed stylus renders
  zero LENGTHS as `0` (`0px`→`0`) but keeps `0%`, `0s/ms`, `0deg/rad/grad/turn`,
  `0fr`, dpi/dppx/hz/etc. Implemented in `Number.CSS(pretty=false)` with a
  `zeroKeepUnits` keep-list; pretty output unchanged.

**Tests** — `m10_test.go`: 28 table cases (pow precedence/assoc/units, ranges
incl/excl/descending/as-list, color ops + both alpha rules, implicit returns
incl. branch/bare-ident/last-wins/explicit-beats-later, transparent mixins
incl. self-guard + defaults, zero units) + pretty-keeps-0px + range-cap guard.
45s fuzz clean.

## Verification (session end)

`go build ./...`, `go vet ./...`, `gofmt -l .` clean; full `go test -count=1
./...` passing including difftest (23/32, 72%). Fuzz: 30s + 45s clean runs.

## Commits on `main` this session

1. `3777297` — M9: differential testing vs reference Stylus (score in CI)
2. `f69b929` — fix: for-loop binding order + HSL/mix color math (+spin)
3. `912481d` — fix: property-value `/` literal unless parenthesized
4. `194738c` — M10: `**`, ranges, color arithmetic, implicit returns,
   transparent mixins, zero-unit strip

Not pushed (CI runs on next push).

---

## Roadmap

- [x] M1–M8 (vertical slice → HTTP middleware)
- [x] M9 differential testing + CI (`difftest/`, ratchet baseline)
- [x] M10 Stylus parity (compat bugs + gaps) — score 23/32 (72%)
- [ ] Candidates: grow the canonical corpus (score denominator), decide fate
  of remaining extensions vs stylus (`{expr}` in @media/calc/strings — could
  gate or keep), list indexing `r[1]` (stylus supports, go-styl doesn't),
  stylus keyframes vendor-prefixing (probably skip — obsolete), CLI watch
  mode, WASM playground, `styl fmt`, benchmarks vs node stylus

## Carry-forward notes

- difftest requires `npm install --prefix difftest` locally; test skips
  without it. `UPDATE_KNOWN_DIFFS=1` regenerates baseline (preserves notes +
  header). Normalizer rules are equality-preserving; extend `namedColors` /
  `zeroKeepUnits` as corpus grows.
- known_diffs.txt now holds ONLY extensions + reference-stylus failures
  (9 entries). Tag future entries [compat-bug?] / [compat-gap].
- Stylus semantics learned (all probe-verified): `**` mult-level left-assoc;
  adjust() % = relative w/ lightness-headroom special case, unitless =
  absolute points; mix/tint/shade floor, arithmetic rounds; RGBA.operate
  alpha asymmetry; transparent-mixin self-name guard; `for val, index`;
  prop-value `/` literal at depth 0 (operands still evaluate); compressed
  zero drops length units only.
- examples/05 crashes reference stylus itself (their bug, not ours).
- rweb repo still has unpushed `feat/stylus-middleware` branch (user drives
  rweb PRs; Azure `go_origin` remote is backup-only — disregard).
