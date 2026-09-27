# Next List — go-styl

The living work list for go-styl. Sessions edit this file in place and commit
it with their session doc, so items are not copied forward from doc to doc. A
session doc's `## Next` section records only what changed:
`Closed: … Raised: …`.

Seeded 2026-09-27 (`2026-0927-1300-go-styl-mixed-syntax-and-cema-compat`) from the 14 earlier session docs
in `ai_docs/claude_sessions/` (2026-0624-1641 … 2026-0706-1929) and from that
session's attempt to compile cema's stylesheets with go-styl.

## Conventions

- IDs are permanent and never reused.
- `raised` is the session-doc stem the item first appeared in. An item raised
  in another repo's session is written `<repo>:<stem>`.
- Age is computed, never stored: the number of session docs
  (`ai_docs/claude_sessions/`) newer than the `raised` doc. A `<repo>:` item
  counts as age 0 until the first go-styl doc after its date.
- Value is the payoff of doing it, not the effort:
  - **high**: something is being worked around today, or a second
    independent consumer has arrived.
  - **medium**: it blocks one named thing, or it is a visible defect nobody
    has to route around yet.
  - **low**: a gap nobody has bumped into, or contingent on something that
    does not exist yet.
- **Open** is what we intend to pick up next. **Roadmap** is wanted, but
  later. **Non-goals** is what we are likely never to do.
- Nothing leaves Open or Roadmap without a line in another section.
- Open and Roadmap stay in ID order.

**Next ID:** N-035

## Open

- **N-001** · raised `2026-0624-1853-go-styl-m6a-m6b` · value low
  Evaluate bare variables inside `url()`. Stylus turns `p = "a.png"` +
  `url(p)` into `url("a.png")`; go-styl emits `url(p)`. (Originally raised
  together with `calc()`, but Stylus leaves `calc()` args literal too, so only
  `url()` is a gap.) Dropped off the list after this doc.
- **N-003** · raised `2026-0624-1853-go-styl-m6a-m6b` · value low
  Value-level source mapping. Maps are selector/declaration/at-rule granular
  today. Still in the README's Future line; dropped from the session lists
  after `2026-0701-2034-m11-wasm-playground-deploy`.
- **N-004** · raised `2026-0624-1853-go-styl-m6a-m6b` · value low
  More built-ins and deeper compress parity. In the README's Future line;
  dropped from the session lists after
  `2026-0701-2034-m11-wasm-playground-deploy`.
- **N-005** · raised `2026-0701-1623-go-styl-m7-m8` · value low
  A Stylus LSP (killer feature #4), with `styl fmt` falling out of it. `styl
  fmt` was raised on its own here; `2026-0703-0846-m13-runtime-theming` folded
  it into the LSP.
- **N-006** · raised `2026-0701-1917-go-styl-difftest-and-stylus-parity` · value medium
  Grow the difftest corpus (15 files). It missed basic syntax that cema uses:
  selectors continued across lines with a trailing comma fail even at the top
  level (N-024). Add a sheet shaped like cema's `styles/styl` (indented
  syntax, trailing `;`, mixin-heavy). Dropped off the list after
  `2026-0701-2034-m11-wasm-playground-deploy`.
- **N-007** · raised `2026-0701-1917-go-styl-difftest-and-stylus-parity` · value low
  Decide the fate of the go-styl extensions (`{expr}` in `@media`, `calc()`
  and strings; single-line `f(x) = expr`): gate them or keep them. Note that
  the natural workaround for N-028 (`calc(100vh - {x})`) relies on one of
  them. Dropped off the list after
  `2026-0701-2034-m11-wasm-playground-deploy`. A standalone `{expr}` is also
  unsupported anywhere in a file that uses braces; `s("…%s…", x)` avoids it.
- **N-008** · raised `2026-0701-1917-go-styl-difftest-and-stylus-parity` · value low
  List indexing `r[1]`. Still missing: `unexpected "[" in expression`; Stylus
  gives the second element. Dropped off the list after
  `2026-0701-2034-m11-wasm-playground-deploy`.
- **N-009** · raised `2026-0703-0846-m13-runtime-theming` · value low
  Scoped component styles, `styl.Component` with hashed class names (the
  first half of killer feature #3), feeding the same `GoSource` renderer as
  `styl gen`.
- **N-010** · raised `2026-0703-0846-m13-runtime-theming` · value low
  `styl migrate`: Stylus → modern CSS migration tool (killer feature #5).
- **N-011** · raised `2026-0703-0846-m13-runtime-theming` · value low
  Safe multi-tenant theme compilation / sandbox (killer feature #6).
- **N-012** · raised `2026-0703-0846-m13-runtime-theming` · value low
  Per-request globals in stylserve, with a cache keyed by the variable set.
  `stylserve.Options.Globals` is still fixed per engine. Dropped off the list
  after `2026-0703-0959-playground-globals-exposure`.
- **N-013** · raised `2026-0703-0959-playground-globals-exposure` · value low
  Dev-mode live reload for stylserve (killer features, honorable mention).
- **N-014** · raised `2026-0706-1146-m15-critical-css-and-rweb-middleware` · value low
  Playground pane that prunes the compiled CSS against pasted HTML (M15
  exposure).
- **N-015** · raised `2026-0706-1146-m15-critical-css-and-rweb-middleware` · value low
  net/http critical-CSS adapter: a `stylhttp` twin of rweb's
  `middleware/critical`, on top of `stylcrit`.
- **N-016** · raised `2026-0706-1250-playground-highlighting-tutorial` · value low
  Playground: mark compile errors in the editor gutter from the result's
  `line`/`col`.
- **N-017** · raised `2026-0706-1250-playground-highlighting-tutorial` · value low
  Playground: shareable URLs (source in the hash).
- **N-018** · raised `2026-0706-1250-playground-highlighting-tutorial` · value low
  Tutorial lesson deep-links (`#tut/<id>`). Only `#tutorial` is handled today.
- **N-019** · raised `2026-0706-1250-playground-highlighting-tutorial` · value low
  A Go highlighter for `go` blocks in tutorial prose (currently escaped plain
  text).
- **N-020** · raised `2026-0706-1417-element-playground-live-deploy-verified` · value low
  Playground boot: call `switchTab(tab)` unconditionally. `index.html` still
  does `if (tab === 'tutorial') switchTab('tutorial')`, leaving
  `body[data-tab]` unset when booting to the play tab. Harmless until
  something styles `body[data-tab="play"]`.
- **N-021** · raised `2026-0706-1417-element-playground-live-deploy-verified` · value low
  Tutorial's "where to next" lesson: link to element's tutorial
  (https://rohanthewiz.github.io/element/#tutorial), mirroring element's link
  here.
- **N-022** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · value medium
  Compile cema's `styles/styl` (`master.styl` + six `theme_masters`) with
  zero semantic diff against stylus 0.64, so cema can drop the `stylus` npm
  dependency (cema branch `roh/use-go-styl`). Blocked by N-024…N-030
  (N-002, mixed syntax, is closed). With every gap patched around in a
  scratch copy, the remaining diffs were all go-styl bugs listed here, plus
  cema-side source quirks that
  cema must fix itself (a column-0 `.form-help {` in `_material_form.styl`,
  `/* */` inside `//` comments in `_banner.styl`, and three dead mixin calls).
- **N-024** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · value medium
  Multi-line selector groups. (a) A trailing-comma continuation (`.a,` then
  `.b` on the next line) is a parse error, even at the top level. (b) Stacked
  selector lines sharing one block (`&:after` then `&:before`) are a parse
  error, or are **silently misparsed**: `td:nth-child(1)` over
  `td:nth-child(2)` becomes the declaration `td: nth-child(1)`.
- **N-025** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · value medium
  A trailing parent reference (`.checkbox &`) is emitted literally
  (`.x .a .checkbox &`) instead of `.checkbox .x .a`. Silent wrong CSS.
- **N-026** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · value medium
  Backslash escapes in strings are dropped: `content "\2022"` outputs
  `"2022"`, so the page shows the text instead of a bullet. Silent wrong CSS.
- **N-027** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · value medium
  Semicolons in indentation syntax: a trailing `;` (`margin: 0 auto;`) and
  several declarations on one line (`color:black; background: #72962d`) are
  parse errors. cema has 337 such lines across 15 files.
- **N-028** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · value medium
  Stylus's sprintf operator: `"calc(100vh - %s)" % x` → `cannot apply "%" to
  string and unit`. cema uses it 5 times.
- **N-029** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · value low
  Number formatting: `mf-font-size * 1.8` prints `1.7999999999999998rem`
  where Stylus prints `1.8rem`. Renders the same.
- **N-030** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · value low
  A statement-level call to an undefined mixin (`sermon()`, a bodiless
  `payment_form()`) is an error in go-styl but silently dropped by Stylus.
  The error found three dead lines in cema, so keeping it seems right; record
  it as a deliberate divergence in `difftest/known_diffs.txt` and the README.

## Roadmap

Wanted, but deferred on purpose.

(none — sort Open items here as priorities settle)

## Non-goals

- **N-031** · declined `2026-0701-2034-m11-wasm-playground-deploy` — CLI
  watch mode (`-w`). External watchers, middleware recompile-on-request and
  `Result.Deps` cover it.
- **N-034** · declined `2026-0701-1917-go-styl-difftest-and-stylus-parity` —
  Stylus-style vendor-prefixed `@keyframes` copies (`@-moz-`, `@-webkit-`,
  `@-o-`). "Probably skip — obsolete"; kept as a known diff for
  `examples/08-at-rules.styl`.

## Closed

Closures before this file was seeded (M1–M15, M6a correctness fixes, etc.)
are recorded in the session docs.

- **N-002** · raised `2026-0624-1853-go-styl-m6a-m6b` · closed 2026-09-27, `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat`
  Mixed indentation + brace syntax in one file. `bracesToIndent` now takes a
  statement's depth from the source indentation outside braces and from the
  braces inside them; a closed braced header takes no indentation children.
  Tests in `mixed_syntax_test.go`, expected values checked against stylus 0.64.
  Difftest unchanged at 23/32. cema's flattened sheets compile with
  `_material_form.styl`'s braces intact, identical to the hand-stripped run.

- **N-023** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · closed 2026-09-27, `2026-0927-1348-go-styl-require-and-glob-imports`
  `@require` and glob import paths. The parser reads `@require` as an
  `ast.Import` with `Once` set. `resolveImport`/`resolveImportFS` expand a glob
  (`.styl` appended, as in Stylus) into its matches in sorted order, taken from
  the first base that has any. The evaluator skips each resolved file already
  in its `required` set. Only `@require` reads and writes that set, as in
  Stylus's requireHistory. Tests in `require_test.go` match stylus 0.64. The one
  known difference: a literal `.css` require passes through, while Stylus
  fails if the `.css` file doesn't exist.

- **N-032** · raised `2026-0624-1757-go-styl-m4-m5` · closed 2026-09-27, `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat`
  Unquoted `url(/path.ext)` tripping the value lexer. Verified fixed:
  `background url(/img/a.png)` compiles verbatim (M6a literal `url()`).
- **N-033** · raised `2026-0701-1623-go-styl-m7-m8` · closed 2026-09-27, `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat`
  Nested module for stylrweb. Overtaken: the middleware moved into rweb as
  `middleware/stylus` in that doc's addendum, and go-styl is serr-only again.
