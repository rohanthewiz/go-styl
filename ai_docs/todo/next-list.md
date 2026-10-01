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

**Next ID:** N-051

## Open

None.

## Roadmap

Wanted, but deferred on purpose.

None.

## Non-goals

- **N-031** · declined `2026-0701-2034-m11-wasm-playground-deploy` — CLI
  watch mode (`-w`). External watchers, middleware recompile-on-request and
  `Result.Deps` cover it.
- **N-034** · declined `2026-0701-1917-go-styl-difftest-and-stylus-parity` —
  Stylus-style vendor-prefixed `@keyframes` copies (`@-moz-`, `@-webkit-`,
  `@-o-`). "Probably skip — obsolete"; kept as a known diff for
  `examples/08-at-rules.styl`.
- **N-035** · declined `2026-0927-1637-go-styl-bare-pseudo-decision` —
  Stylus's descendant reading of a bare nested pseudo-class (`.x` over `:hover` → `.x :hover`). go-styl attaches it
  (`.x:hover`), a convenience the README and the playground tutorial
  already document. That's almost always what the author meant, and
  switching would silently change sheets written from the tutorial.
  Recorded under README "Deliberate differences"; pinned by
  `difftest/corpus/bare-pseudo.styl`. `& :hover` gives the descendant form.

- **N-042** · declined `2026-0927-1838-go-styl-recovered-n040-n010-n011-n009`
  — `use()`. It loads a JavaScript plugin; go-styl reports a clear error
  and points at `Options.Globals` for passing Go values in.

## Closed

Closures before this file was seeded (M1–M15, M6a correctness fixes, etc.)
are recorded in the session docs.

- **N-050** · raised `2026-0930-0127-go-styl-n049-multi-file-source-maps` · closed 2026-10-01, `2026-1001-0030-go-styl-n050-map-relative-sources`
  Source names relative to the map file. `Options.MapFile` (in `Filename`'s
  terms) makes every `sources` name, the entry's included, relative to the
  map's directory (`eval.mapRelName`, both sides in key form first); empty
  keeps the old naming. The CLI passes `<out>.map` (`-o out/app.css
  styles/app.styl` → `../styles/app.styl`); stylserve passes
  `<Dir>/<cssPath>.map`, so served maps read `app.styl` instead of an
  absolute server path.

- **N-049** · raised `2026-0929-2350-go-styl-n003-value-source-maps` · closed 2026-09-30, `2026-0930-0127-go-styl-n049-multi-file-source-maps`
  `css.Pos.File` carries the evaluator's file key (entry `Filename`, or an
  import's resolved path; mixin bodies keep their defining file). An import
  joins `sources` (with its `sourcesContent`) when a segment first points
  into it, so variables-only partials stay out, and an unknown key gets no
  segment. Import names read in the entry path's terms
  (`eval.sourceName`): `testdata/imports/main.styl` → `testdata/imports/_vars.styl`,
  absolute for an absolute entry, fs paths under `Options.FS`.

- **N-003** · raised `2026-0624-1853-go-styl-m6a-m6b` · closed 2026-09-29, `2026-0929-2350-go-styl-n003-value-source-maps`
  Each declaration value gets its own segment, pointing at the value
  expression's first character (`ast.Declaration.ValueLine/ValueCol` →
  `css.Statement.ValuePos`); a variable maps to its use site, not its
  definition. Positions are now true source positions in every layout:
  `line.srcLine/srcCol` counts characters (a tab is one column, not
  `tabWidth`), and `bracesToIndent` returns a per-output-line position
  table (scanner `at` hook), so brace files, one-line blocks (which used to
  drift onto later lines) and a comment before a statement all map and
  report errors exactly. Multi-file maps → N-049.

- **N-048** · raised `2026-0927-2107-go-styl-n045-n046-n047-lsp-fmt` · closed 2026-09-27, `2026-0927-2203-go-styl-n048-builtin-signatures`
  `register("darken(color, amount)", f)`: each built-in registers under its
  signature, so none can lack one; context built-ins keep theirs in
  `ctxBuiltinSigs`. `eval.BuiltinSignatures` feeds signature help (built-in
  fallback, multiple forms for rgba), hover and completion detail.

- **N-047** · raised `2026-0927-1931-n005-lsp-and-fmt` · closed 2026-09-27, `2026-0927-2107-go-styl-n045-n046-n047-lsp-fmt`
  `styl fmt` statement spelling (`internal/parser/respell.go`), one rule
  per statement kind, located from the parse. Declarations: `prop: value`
  spacing, always a colon in braces, and in indentation syntax the file's
  majority form (a tie keeps each). ` = ` / ` ?= ` in assignments and
  parameter defaults, `, ` in values (not selectors, strings or `url()`).
  Per-line AST guard (`applyGuarded`). Mixed-syntax lines outside braces
  are re-indented by `bracesToIndent`'s depth rule. `FuzzFormat` checks
  idempotency; three stray `\r`/`\f` cases fixed.
- **N-046** · raised `2026-0927-1931-n005-lsp-and-fmt` · closed 2026-09-27, `2026-0927-2107-go-styl-n045-n046-n047-lsp-fmt`
  LSP references and rename (every open document taken as a compile root,
  so partials sharing the root scope are covered; use sites from AST names
  mapped onto source lines; property names count as transparent mixin
  calls), prepareRename, signature help for user mixins/functions, CSS
  property-name completion, and `styl-lint` diagnostics: duplicate
  properties (fallback idiom allowed), unused local variables and
  functions, unused root mixins in entry sheets. Root variables are never
  flagged. Built-in signatures → N-048.
- **N-045** · raised `2026-0927-1931-n005-lsp-and-fmt` · closed 2026-09-27, `2026-0927-2107-go-styl-n045-n046-n047-lsp-fmt`
  The LSP compiles through `overlayFS`: open buffers laid over the root
  DirFS. It records each compile's reads as deps, and a change re-analyzes
  the open documents that read the file. Closing an unsaved buffer reverts
  them to disk. Limit: glob imports list directories from disk only.

- **N-044** · raised `2026-0927-1838-go-styl-recovered-n040-n010-n011-n009` · closed 2026-09-27, `2026-0927-2036-go-styl-n044-migrate-split`
  `styl migrate -split -o dir` / `MigrateOptions.Split`: each root-level
  `.styl` import gets its own output root and becomes `@import "x.css"`
  (moved to the file top, noted if it passes a rule); files emitting no CSS
  vanish (decided after @extend, noted); :root goes to `tokens.css`
  (`-tokens`), imported first by the entry. Nested and repeated imports are
  still inlined, noted. `MigrateResult.Files`; split round-trip test over
  the corpus plus an fstest project.

- **N-043** · raised `2026-0927-1838-go-styl-recovered-n040-n010-n011-n009` · closed 2026-09-27, `2026-0927-2024-go-styl-n043-migrate-comments`
  `styl migrate` keeps source comments. `parser.ParseWithComments` adds
  `ast.Comment` statements (Parse is unchanged, so compile/fmt/LSP never see
  them); comments attach to real lines as lead/trail, never as tree lines.
  Migrate writes them in place (`//` as `/* */`), moves a root variable's
  doc comments into `:root`, drops a mixin/function's doc comment with a
  `comment` note, writes loop-body comments once. `-no-comments` /
  `MigrateOptions.NoComments` opt out. Invariant test + `FuzzParseWithComments`:
  minus comments the tree equals Parse's, and every comment appears once.

- **N-041** · raised `2026-0927-1838-go-styl-recovered-n040-n010-n011-n009` · closed 2026-09-27, `2026-0927-2007-go-styl-n041-block-mixins`
  User block mixins. `+m(args)` with an indented body passes the body to a
  user mixin, which runs it at `{block}` (`ast.BlockSlot`, matched as
  whole-line text; the brace normalizer copies `{block}` verbatim). The
  body keeps its call-site scope, file and import dir but emits into the
  slot's rule, selectors, @media and class prefix (`passedBlock`, carried
  on every nested `execCtx`); a `{block}` inside the body refers to the
  enclosing mixin's block. A user mixin shadows `+prefix-classes`.
  `styl migrate` expands it the same way. Corpus file
  `difftest/corpus/block-mixins.styl` matches stylus 0.64. Deliberate
  difference: `{block}` outside any mixin is an error (Stylus drops it); in
  a mixin called without a block it is empty, as in Stylus.

- **N-005** · raised `2026-0701-1623-go-styl-m7-m8` · closed 2026-09-27, `2026-0927-1931-n005-lsp-and-fmt`
  Stylus LSP (killer feature #4) and `styl fmt`.
  - `internal/parser/format.go`: `Format` rewrites whitespace only and
    never prints the AST, which has no comments (N-043). Indentation comes
    from the line-tree rule, or from `scanStructural` brace events in brace
    syntax. Comments move with their code, and continuation lines keep
    their offsets.
  - Every result must re-parse to an equal AST, ignoring Line/Col and
    whitespace in raw selectors/params. Otherwise fmt falls back to a
    whitespace-only pass, then to `ErrFormatUnsafe`.
  - Tests: corpus (examples, testdata, difftest) round-trip plus
    idempotency, golden cases, and `FuzzFormat`.
  - `styl fmt [-w] [-l]`, `styl.Format`.
  - `internal/lsp` + `cmd/styl-lsp`: a hand-rolled JSON-RPC layer (no new
    deps) with full sync. Each change runs parse + `ExtractManifest` in a
    Sandbox (1s, 2M steps) over a root DirFS. Features: diagnostics
    (import errors on the `@import` line, limits as warnings), completion,
    hover with computed values, scope-aware definition across imports,
    symbols, hex color swatches and picker, formatting. The last good
    parse serves broken text.
  - `eval.BuiltinNames` / `eval.ResolveImport` for tooling.
  - Tests: `internal/lsp/server_test.go` drives the server over pipes.
  - Follow-ups: N-045, N-046, N-047.

- **N-011** · raised `2026-0703-0846-m13-runtime-theming` · closed 2026-09-27, `2026-0927-1838-go-styl-recovered-n040-n010-n011-n009`
  Safe multi-tenant theme compilation (killer feature #6): `Options.Sandbox`
  (`sandbox.go`, `internal/eval/sandbox.go`). No OS filesystem (imports and
  `*File` read only `Options.FS`), `AllowImport`, `Timeout`/`Context`,
  `MaxSteps` (ticked per executed statement), `MaxValueBytes` (checked on
  binary-op, list-literal and call results — ranges exempt), and
  `MaxSourceBytes`/`MaxImports`/`MaxOutputBytes` (output includes @extend
  grafts). Errors wrap `styl.ErrLimit`; warn() without a hook is dropped.
  Every entry point honors it, including Migrate's own statement walk.
- **N-010** · raised `2026-0703-0846-m13-runtime-theming` · closed 2026-09-27, `2026-0927-1838-go-styl-recovered-n040-n010-n011-n009`
  `styl migrate`: Stylus → modern CSS (killer feature #5).
  `internal/eval/migrate.go` reuses the evaluator's scopes, expressions and
  built-ins, and swaps in its own statement walk. That walk builds a nested
  output tree instead of flat rules.
  - Nesting is native CSS nesting (a bare `:hover` becomes `&:hover`).
  - A root variable's first binding becomes a `:root` custom property.
    Only the properties the output reads are emitted.
  - Arithmetic becomes `calc()` when the units allow it, in `:root` too.
  - Mixins, loops, `if`, `@extend` (folded into the target's selector
    list) and `.styl` imports are resolved in place, each with a
    `/* styl-migrate: … */` note. Frozen values (computed from a variable)
    get one as well.
  - `&__elem` concatenation, rules under a pseudo-element, and
    `@keyframes`/`@font-face` inside a rule are hoisted after the
    enclosing block, inside the same at-rules.
  - `TestMigrateRoundTrip` flattens the migrated CSS for every example,
    fixture and difftest sheet and checks it against `Compile`. Migrate
    was added to `FuzzCompile`.
  - Follow-ups: comments (N-043), file-for-file output (N-044).

- **N-009** · raised `2026-0703-0846-m13-runtime-theming` · closed 2026-09-27, `2026-0927-1838-go-styl-recovered-n040-n010-n011-n009`
  Scoped component styles. `styl.Component`/`ComponentFile` return
  `*Scoped{CSS, Names, Manifest}`. `css.Scope` renames, in place, class
  tokens in selectors (own, `@extend` grafts, merged duplicates),
  `@keyframes` names, and `animation`/`animation-name` references to local
  keyframes, as `name_<8 base32 chars of SHA-256(src)>`. The hash ignores
  Options, so Globals never move the names. IDs, type and attribute
  selectors stay global, and `:global(...)` opts out. `Manifest.Scoped`
  makes `GoSource` emit scoped values (`Card = "card_…" // card`), and `styl
  gen -scoped [-css out.css]` wires it up. Tests in `component_test.go`.

- **N-040** · raised `2026-0927-1712-go-styl-more-builtins` · closed 2026-09-27, `2026-0927-1838-go-styl-recovered-n040-n010-n011-n009`
  Objects (hashes) and the context built-ins. `value.Hash` is an ordered,
  by-reference object: literals (`{a: 1}`, multi-line `x = {` folded onto
  one line by `joinObjectLiterals`), `obj.key` / `obj[k]` reads, member
  assignment (`obj.a.b = x`, `?=`), `in`, `for k, v in obj`, and
  object-aware `length`/`keys`/`values`/`clone`, plus `merge`/`extend`
  (deep with a trailing `true`), `contrast` (returns an object) and
  `json()` (both modes, a build dep). Context built-ins in
  `internal/eval/context.go`: `selector()`, `selectors()`,
  `selector-exists()`, `current-media()`, `define()`, `lookup()`,
  `add-property()`, `warn()` (`Options.Warn`), `+prefix-classes(p)`.
  `push`/`append`/`unshift`/`prepend`/`pop`/`shift` rebind the list
  variable they're given. `use()` → N-042; user block mixins → N-041 (closed).
  Deliberate differences: `current-media()` omits 0.64's stray parentheses;
  `selector-exists()` only sees rules compiled before it; an object as a
  property value is an error; `in` whose right side is a bare word stays CSS
  text in property values and call args (`to right in oklch`).

- **N-013** · raised `2026-0703-0959-playground-globals-exposure` · closed 2026-09-27, `2026-0927-1715-go-styl-live-reload`
  Dev-mode live reload. `stylserve.Options.LiveReload` makes `stylhttp`
  serve `_live.js` (it finds the page's stylesheets under its own prefix and
  opens an EventSource per sheet) and `_live?css=<name>.css`, an SSE stream
  that polls the engine every 500ms. A change event swaps in fresh CSS
  without a reload; compile errors go to the console and the last good CSS
  stays. No fs watcher dependency. Tests in `stylhttp/live_test.go`;
  browser-verified.

- **N-004** · raised `2026-0624-1853-go-styl-m6a-m6b` · closed 2026-09-27, `2026-0927-1712-go-styl-more-builtins`
  More built-ins, taken from a diff of Stylus's function list against the
  registry: `asin/acos/atan`, `radians-to-degrees`/`degrees-to-radians`,
  `sum`, `avg`, `odd`, `even`, `remove-unit`, `percent-to-decimal`,
  `base-convert`, `fade-in/out`, `grayscale`, `luminosity`, `blend`,
  `transparentify`, `component`, `pop`, `shift`, `range`, `list-separator`,
  `keys`/`values` (pairs), `clone`, `basename`/`dirname`/`extname`/
  `pathjoin`, `convert`, `opposite-position`, `error`. Parity fixes:
  `sin/cos/tan` take `deg` and round to 9 places; `saturate()`/`invert()`/
  `grayscale()` pass through as CSS filter functions; a builtin called as a
  statement runs (a function ending in `round(n)`, `error()` guards). 45
  cases in `builtins_stylus_test.go` match stylus 0.64. The rest is N-040.

- **N-012** · raised `2026-0703-0846-m13-runtime-theming` · closed 2026-09-27, `2026-0927-1706-go-styl-per-request-globals`
  Per-request globals. `stylserve.Engine.AssetWith(path, globals)` layers
  extra globals over `Options.Globals` and caches each variant by path plus
  a type-aware, order-independent fingerprint. Variants are capped by
  `Options.MaxVariants` (default 256; base builds are never evicted) and
  invalidated by source changes. `stylhttp.NewWithGlobals(opts,
  func(*http.Request) map[string]any, vary...)` sends Vary, or
  `Cache-Control: private` when no vary list is given.

- **N-014** · raised `2026-0706-1146-m15-critical-css-and-rweb-middleware` · closed 2026-09-27, `2026-0927-1704-go-styl-playground-prune-pane`
  Playground "prune to HTML (critical CSS)" box under the CSS output. With
  pasted HTML, the output shows `styl.Prune`'s result, with the size saving
  and the tag/class/id counts found. It works through a new `pruneHTML`
  compile option in the WASM API (result gains `pruned`, `used`). The HTML
  is saved locally and included in share links. Browser-verified.

- **N-015** · raised `2026-0706-1146-m15-critical-css-and-rweb-middleware` · closed 2026-09-27, `2026-0927-1702-go-styl-nethttp-critical`
  `stylhttp.Critical(stylcrit.Options) func(http.Handler) http.Handler`, the
  net/http twin of rweb's `middleware/critical`. A buffering writer decides
  at WriteHeader: non-2xx, non-HTML or Content-Encoding responses pass
  through unbuffered (Flush forwarded, so streams keep streaming). HTML is
  buffered, type-sniffed if unset, inlined via `stylcrit.Engine.Inline`, and
  loses its stale Content-Length and ETag. Compile errors give a positioned
  500. Tests in `stylhttp/critical_test.go`.

- **N-019** · raised `2026-0706-1250-playground-highlighting-tutorial` · closed 2026-09-27, `2026-0927-1700-go-styl-tutorial-go-highlighting`
  Go snippets in tutorial prose are highlighted. `stylHi.go(src)`
  (`playground/highlight.js`) is a single-pass tokenizer (comments, the three
  string kinds including multi-line raw strings, numbers, keywords,
  predeclared types/consts, calls, exported selectors) reusing the existing
  token classes. Text round-trips losslessly. Browser-verified.

- **N-039** · raised `2026-0927-1658-go-styl-playground-error-marks` · closed 2026-09-27, `2026-0927-1659-go-styl-eof-error-message`
  An expression that ends early now reports `unexpected end of expression`
  (a `token.EOF` case in `parseOperand`) instead of `unexpected ""`. Test in
  `TestM7ErrorPositions`.

- **N-016** · raised `2026-0706-1250-playground-highlighting-tutorial` · closed 2026-09-27, `2026-0927-1658-go-styl-playground-error-marks`
  Compile errors marked in the editor. The overlay editor has no gutter, so
  `stylHi.errorMark(ta)` draws a tinted band with a left-edge bar on the
  error line, kept aligned on scroll. Clicking the error message jumps the
  caret to line:col. Works in both the playground and the tutorial editor;
  only errors in the editor's own source (`playground.styl`) are marked.
  Browser-verified.

- **N-017** · raised `2026-0706-1250-playground-highlighting-tutorial` · closed 2026-09-27, `2026-0927-1655-go-styl-playground-share-links`
  Shareable playground URLs. A **share** button encodes the source, globals
  and options as `#z/<base64url(deflate-raw JSON)>` (`#u/` uncompressed
  fallback), puts it in the address bar and copies it. Opening a link loads
  it without overwriting the visitor's saved draft until they edit; the
  first edit drops the stale hash. Browser-verified.

- **N-018** · raised `2026-0706-1250-playground-highlighting-tutorial` · closed 2026-09-27, `2026-0927-1653-go-styl-tutorial-deep-links`
  Tutorial deep links. `#tut/<lesson-id>` opens the tutorial at that lesson,
  at load (`gsTutorial.init({lesson})`) or via `hashchange`. While the
  tutorial tab shows, the address bar tracks the open lesson
  (`replaceState`). The play tab clears a tutorial hash, an unknown id snaps
  back to the open lesson, and `#tutorial` still works. Browser-verified.

- **N-007** · raised `2026-0701-1917-go-styl-difftest-and-stylus-parity` · closed 2026-09-27, `2026-0927-1650-go-styl-extensions-decision`
  Decision: keep the go-styl extensions ungated. They're listed in a new
  README "Extensions" section, and each is pinned in `known_diffs.txt`. The
  strings extension was narrowed: `interpolateString` substitutes a `{…}`
  group only when it references a defined variable, and `\{` escapes. So
  `"{nope}"`, `"{1 + 2}"` and `"\{p}"` stay literal, as in Stylus
  (`TestStringInterpolationScope`). README Status list updated for this
  session's additions.

- **N-021** · raised `2026-0706-1417-element-playground-live-deploy-verified` · closed 2026-09-27, `2026-0927-1648-go-styl-tutorial-element-link`
  The tutorial's "Where to next" lesson links to element's tutorial
  (https://rohanthewiz.github.io/element/#tutorial), mirroring element's
  link to go-styl.

- **N-020** · raised `2026-0706-1417-element-playground-live-deploy-verified` · closed 2026-09-27, `2026-0927-1648-go-styl-playground-boot-tab`
  Playground boot calls `switchTab` unconditionally
  (`switchTab(tab === 'tutorial' ? 'tutorial' : 'play')`), so
  `body[data-tab]` is set from the first paint on either tab.

- **N-006** · raised `2026-0701-1917-go-styl-difftest-and-stylus-parity` · closed 2026-09-27, `2026-0927-1647-go-styl-cema-shaped-corpus`
  Difftest corpus grown with `difftest/corpus/cema-shaped.styl` (plus
  partials in `difftest/corpus/imports/site/`, which aren't compiled
  directly). It is an indented-syntax site sheet using `@require` with a
  glob, semicolons, mixins with defaults, multi-line selector groups,
  trailing `&`, `"\2022"`, the `%` operator, list indexing, `url()`
  variables and string `+`. It matches stylus 0.64: score 24/35. (The
  corpus also gained `undefined-mixin.styl` and `bare-pseudo.styl` as
  pinned deliberate differences this session.)

- **N-038** · raised `2026-0927-1645-go-styl-url-variables` · closed 2026-09-27, `2026-0927-1646-go-styl-interp-strings`
  Interpolating a quoted string now gives its raw text (`evalString` in
  `internal/eval/m4.go`), as in Stylus: `.a-{s}` → `.a-x`, and
  `url({base}x.png)` → `url(/img/x.png)`. Units and whole lists are still
  kept (Stylus drops units and uses only a list's first item), because
  go-styl's `@media` and `calc()` interpolation needs them. Tests in
  `interp_string_test.go`.

- **N-001** · raised `2026-0624-1853-go-styl-m6a-m6b` · closed 2026-09-27, `2026-0927-1645-go-styl-url-variables`
  Variables inside `url()`. `evalURL` (`internal/eval/eval.go`) evaluates a
  raw `url(...)` token when its contents parse as an expression that
  references a defined variable, and prints Stylus-style `url("…")`
  (strings unquoted, list items run together). Anything else stays verbatim,
  avoiding quoting churn on every existing `url()`. Tests in
  `url_vars_test.go` match stylus 0.64 semantically.

- **N-008** · raised `2026-0701-1917-go-styl-difftest-and-stylus-parity` · closed 2026-09-27, `2026-0927-1643-go-styl-list-indexing`
  List indexing. New `ast.Index`, parsed as a postfix on any operand when
  `[` is glued to it (`r[1]`, `(1 2 3)[1]`, `r[0][1]`). `evalIndex`: 0-based,
  negative from the end, past either end → null, a scalar is a one-item
  list, and a list or range index (`r[0 1]`, `r[0..1]`) returns several items.
  Tests in `index_test.go` match stylus 0.64. Hash/object members and
  subscript assignment (`r[1] = x`) are not supported (go-styl has no hashes).

- **N-037** · raised `2026-0927-1625-go-styl-semicolons` · closed 2026-09-27, `2026-0927-1640-go-styl-brace-parens-semicolon`
  `;` inside parentheses in brace syntax. `scanStructural` tracks `( )`
  depth per line and emits a `;` inside parens as text, so
  `.a { background: url(data:image/png;base64,…) }` works. The depth resets
  at newlines and block braces, so an unbalanced `(` can't swallow later
  statements. Tests in `TestSemicolonInParensBraceSyntax`.

- **N-036** · raised `2026-0927-1525-go-styl-string-escapes` · closed 2026-09-27, `2026-0927-1639-go-styl-string-ops`
  String operations. `length()` of a single string counts its characters
  (runes). A string on the left of `+` concatenates: the right side's text
  (strings unquoted, lists space-joined, `null` as "null") goes into a new
  `'…'` string, or an unquoted one when the left side is unquoted (Stylus's
  Literal: `s("%s", 1) + "px"` → `1px`). Tests in `string_ops_test.go` match
  stylus 0.64.

- **N-022** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · closed 2026-09-27, `2026-0927-1636-go-styl-cema-parity`
  go-styl side of compiling cema without stylus. With cema's source quirks
  patched in a scratch copy, all 7 sheets (`master.styl` + six
  `theme_masters`) match stylus 0.64 in rule order, selector order and every
  declaration. The one exception is `opacity: .00000001` vs Stylus's `1e-8`
  (the same number). The last go-styl fix: nested selector groups now
  combine child-major, as in Stylus. What's left is cema-side (five source
  fixes, three of which change cema's CSS, plus switching the build), listed
  in the session doc. Tooling in `ai_docs/tools/cema-parity/`.

- **N-030** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · closed 2026-09-27, `2026-0927-1631-go-styl-undefined-mixin-doc`
  Undefined mixin calls stay an error, now documented as deliberate: a
  README "Deliberate differences" subsection (with the other lenient
  choices from N-026…N-029), and `difftest/corpus/undefined-mixin.styl`
  pinned in `known_diffs.txt`.

- **N-029** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · closed 2026-09-27, `2026-0927-1630-go-styl-number-formatting`
  Number formatting. `formatNum` (`internal/value/value.go`) rounds to 15
  decimal places and then prints the shortest form, like Stylus's
  `parseFloat(n.toFixed(15))`, so `1rem * 1.8` prints `1.8rem`. Unlike
  Stylus, compressed numbers between -1 and 1 are rounded too. A value that
  rounds to zero drops its unit when compressed. Tests in `TestNumberCSS`.

- **N-028** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · closed 2026-09-27, `2026-0927-1628-go-styl-sprintf-operator`
  Stylus's sprintf operator. `evalBinary` sends a string `%` to the `s()`
  builtin, spreading a list on the right into one argument per item.
  `s()` gained `%d` (a number's bare value). Tests in `sprintf_test.go` match
  stylus 0.64. Two small differences remain: `type()` of the result is
  `string` (Stylus: `literal`), and a placeholder with no argument leaves
  its surrounding space.

- **N-027** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · closed 2026-09-27, `2026-0927-1625-go-styl-semicolons`
  Semicolons in the indentation syntax. `expandSemicolons`
  (`internal/parser/semis.go`) runs at the top of `parseBlock` and splits
  each line on top-level `;` (not inside strings, parens, brackets or
  interpolation) into sibling lines. Empty pieces are dropped, and the
  line's indented body goes to the last piece, as in Stylus. Each piece gets
  its own error column. Running after comma continuation means a multi-line
  value's closing `;` is handled too. Tests in `semicolon_test.go` match
  stylus 0.64. cema's 28 `;`-bearing sheets no longer fail on semicolons.

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

- **N-026** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · closed 2026-09-27, `2026-0927-1525-go-styl-string-escapes`
  Backslash escapes in strings. The lexer's `scanString` keeps each `\` and
  the rune after it verbatim, so a string's value is its raw source text as
  in Stylus (`"\2022"`, `"\f101"`, `unquote("\2022")` → `\2022`). An
  escaped quote stays inside the string (`'it\'s'`), where Stylus fails to
  parse. Tests in `string_escape_test.go` and `TestLexStringEscapes` match
  stylus 0.64.

- **N-025** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · closed 2026-09-27, `2026-0927-1423-go-styl-trailing-parent-ref`
  A parent reference anywhere in a selector. `combine` (`internal/eval/selector.go`)
  replaces every `&` in the child with the parent and prepends nothing
  (`.checkbox &` → `.checkbox .x .a`, `& + &`, `:not(&)`). At the top level
  `&` resolves to nothing (`& .a` → `.a`). As in Stylus, the substitution is
  textual, even inside attribute strings. Tests in `parent_ref_test.go` match
  stylus 0.64. One known difference: a top-level selector that is only `&`
  still prints `&{…}`, where Stylus drops the rule.

- **N-024** · raised `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat` · closed 2026-09-27, `2026-0927-1357-go-styl-multiline-selectors`
  Multi-line selector groups. `buildTree` appends the next non-blank line to
  a line that ends in a trailing comma, whatever that line's indentation (this
  covers both syntaxes and multi-line value lists). `groupSelectorLines`
  (`internal/parser/selgroup.go`) merges a run of selector-shaped leaf lines
  into the ruleset line that ends the run, using line-level cues from Stylus's
  `looksLikeSelector` (sigils, combinators, bare idents, `ident` + class/id/attr/
  `::`/a known pseudo). A run with a declaration-shaped line is not merged.
  Following Stylus, a bare ident over a selector block is a type selector,
  not a mixin call. Tests in `multiline_selector_test.go` match stylus 0.64.

- **N-032** · raised `2026-0624-1757-go-styl-m4-m5` · closed 2026-09-27, `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat`
  Unquoted `url(/path.ext)` tripping the value lexer. Verified fixed:
  `background url(/img/a.png)` compiles verbatim (M6a literal `url()`).
- **N-033** · raised `2026-0701-1623-go-styl-m7-m8` · closed 2026-09-27, `2026-0927-1300-go-styl-mixed-syntax-and-cema-compat`
  Nested module for stylrweb. Overtaken: the middleware moved into rweb as
  `middleware/stylus` in that doc's addendum, and go-styl is serr-only again.
