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

**Next ID:** N-038

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
- **N-035** · declined `2026-0927-1637-go-styl-bare-pseudo-decision` —
  Stylus's descendant reading of a bare nested pseudo-class (`.x` over `:hover` → `.x :hover`). go-styl attaches it
  (`.x:hover`), a convenience the README and the playground tutorial
  already document. That's almost always what the author meant, and
  switching would silently change sheets written from the tutorial.
  Recorded under README "Deliberate differences"; pinned by
  `difftest/corpus/bare-pseudo.styl`. `& :hover` gives the descendant form.

## Closed

Closures before this file was seeded (M1–M15, M6a correctness fixes, etc.)
are recorded in the session docs.

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
  fixes, two of which change cema's CSS, plus switching the build), listed
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
