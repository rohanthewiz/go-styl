# go-styl

A pure-Go compiler for the [Stylus](https://stylus-lang.com/) (`.styl`) CSS preprocessor — no Node.js, no cgo. The only dependency is [serr](https://github.com/rohanthewiz/serr), a zero-dependency structured-error wrapper.

> Inspired by [aerogo/scarlet](https://github.com/aerogo/scarlet) but rebuilt around a real
> lexer → AST → evaluator pipeline so it can target the full Stylus language rather than a
> Stylus-inspired subset.

## Status

Under active development -- consider this Alpha. The compiler currently supports:

- Both the **indentation syntax** and the CSS-like **brace/semicolon syntax**
- Indentation-based nesting with `&` parent references and pseudo-class attachment
- Compile-time **variables** (inlined, with lexical scoping) and `?=` conditional assignment
- Unit-aware **arithmetic** (`+ - * / %`) and comparisons
- Comma and space **value lists**, with indexing (`list[0]`, `list[-1]`,
  `list[0..1]`)
- **Control flow**: `if` / `else if` / `else` / `unless`, and `for … in` loops
- User-defined **functions** (return values) and **mixins** (emit declarations/rules),
  with default and rest (`args…`) parameters, in single-line and block forms.
  **Block mixins**: `+m(args)` followed by an indented body passes that body
  to `m`, which emits it wherever it writes `{block}` (e.g. a `mobile()`
  mixin wrapping `{block}` in an `@media`)
- **Built-in functions** across color (`rgb`/`rgba`/`hsl`/`hsla`/`lighten`/`darken`/
  `saturate`/`mix`/`tint`/`shade`/`complement`/`invert`/`grayscale`/`fade-in`/
  `fade-out`/`blend`/`transparentify`/`luminosity`/`component`/`hue`/`alpha`/…),
  math (`abs`/`ceil`/`floor`/`round`/`min`/`max`/`pow`/`percentage`/`sin`/`asin`/
  `sum`/`avg`/`odd`/`even`/`remove-unit`/`base-convert`/…), list
  (`length`/`push`/`pop`/`shift`/`index`/`last`/`join`/`range`/`keys`/`values`/
  `merge`/…; `push`/`pop`/`shift`/`unshift` also update the list variable),
  string and path (`unquote`/`quote`/`s`/`substr`/`replace`/`split`/`uppercase`/
  `basename`/`dirname`/`extname`/`pathjoin`/`convert`/…), and type
  (`typeof`/`unit`/`match`/`light`/`dark`/`opposite-position`/`error`), with CSS
  named-color support. `grayscale()`, `saturate()`, `invert()` and
  `contrast()` with non-color arguments pass through as the CSS filter
  functions.
- **Objects** (hashes): `theme = { bg: #fff, sizes: { sm: 10px } }` (one
  line or several), `theme.bg` / `theme[key]`, member assignment
  (`theme.sizes.lg = 20px`), `'bg' in theme`, `for key, val in theme`,
  `merge`/`extend`, `contrast(fg, bg).ratio`, and `json('tokens.json',
  { hash: true })` (or one variable per leaf without the option). Objects are
  shared by reference, as in Stylus; `clone()` copies one.
- **Context built-ins**: `selector()`, `selectors()`, `selector-exists()`,
  `current-media()`, `define()`, `lookup()`, `add-property()`, `warn()`
  (routed to `Options.Warn`) and `+prefix-classes('ui-')` over an indented
  block
- String operators: concatenation (`"a" + b`) and sprintf (`"calc(100% - %s)" % x`)
- Unknown functions pass through as literal CSS (`translateX(10px)`, `url(...)`)
- **Interpolation** (`{expr}`) in selectors, property names, strings, and identifiers
- **`@extend`** (and `@extends`) plus **`$placeholder`** selectors
- **`@import`** and **`@require`** (import once), with glob paths
  (`@require 'partials/*'`): `.styl` files are inlined (sharing
  variables/mixins); `.css` and `url(...)` imports pass through verbatim
- **At-rules**: `@media` / `@supports` (with selector bubbling and variables in
  queries), `@keyframes`, `@font-face`, and verbatim passthrough for leaf at-rules
  (`@charset`, …)
- Literal **`url()`** and **`calc()`** (operators/paths preserved; `{interp}` still
  resolves; `url(base + "x.png")` evaluates variables as in Stylus),
  **`!important`**, and whitespace-sensitive unary `-`/`+`
  (`margin 10px -5px` is a list; `10px - 5px` subtracts)
- Pretty and **compressed** output, plus an optional duplicate-rule **merge** pass
- **Source maps** (Source Map v3) mapping selectors and declarations back to the
  `.styl` source
- **Positioned errors**: compile errors read `file:line:col: message` (with
  "did you mean" hints for misspelled mixins) and carry `file`/`line`/`col` as
  structured [serr](https://github.com/rohanthewiz/serr) attributes for
  serr-aware loggers
- **`io/fs.FS` sources**: compile from an `embed.FS` (or any `fs.FS`) with
  `@import` resolved inside it — ship `.styl` sources in the binary
- **HTTP middleware**: serve compiled CSS straight from `.styl` sources with
  caching, ETags/304s, and dev source maps — a `net/http` adapter here
  ([`stylhttp`](stylhttp/)); the [rweb](https://github.com/rohanthewiz/rweb)
  adapter ships with rweb (`rweb/middleware/stylus`)
- **Runtime theming** (go-styl extensions — see
  [Runtime theming](#runtime-theming)): inject Go values as Stylus variables
  (`Options.Globals`) and expose theme tokens as CSS custom properties
  (`Options.CustomProperties`, compiling direct references to `var(--name)`)
- **Critical CSS** (see [Critical CSS](#critical-css-stylprune)): `styl.Prune`
  compiles only the rules a rendered page actually uses
  (`styl.UsedFromHTML(page)`) — per-response inline CSS with no headless browser
- **Scoped component styles** (see
  [Scoped component styles](#scoped-component-styles-stylcomponent)):
  `styl.Component` hashes class and `@keyframes` names per component (CSS
  Modules semantics, `:global(...)` opt-out), with `styl gen -scoped` for typed
  constants

See [the roadmap](#roadmap) for what's next.

## Benchmarks

Two layers:

```sh
go test -bench . -run xxx       # Go-native benchmarks (examples + synthetic sheets)
go run ./bench                  # go-styl vs reference stylus over the shared corpus
```

`go run ./bench` compiles every corpus file plus generated synthetic sheets
(`internal/benchgen`) with both compilers — go-styl in-process, reference
stylus timed inside a single node process — from in-memory source with the
filename set, so both pay import I/O but neither re-reads the top-level file,
and node startup is excluded (reported separately). Files only one side
compiles (go-styl extensions, reference-stylus crashes) are skipped.

Representative results (Apple M1 Pro, stylus 0.64.0 on node 22):

| input | go-styl | stylus | speedup |
|---|---|---|---|
| small sheets (~0.2–0.8 KB) | 3–32 µs | 380–710 µs | 18–140× |
| synthetic, 100 components (~20 KB) | 1.7 ms | 10.1 ms | 5.8× |
| synthetic, 400 components (~80 KB) | 7.3 ms | 42 ms | 5.7× |
| whole 29-file corpus, one compile each | 10 ms | 68 ms | 6.8× |

Geomean per-file speedup is ~24×, dominated by reference stylus's ~0.4 ms
per-compile floor (it re-imports its own built-in `.styl` function library on
every render); on large sheets the steady-state throughput advantage is
~5.7×. A stylus CLI invocation additionally pays ~40 ms of node startup that
go-styl doesn't have.

## Installation

```shell
go get github.com/rohanthewiz/go-styl
```

## Library usage

```go
import styl "github.com/rohanthewiz/go-styl"

css, err := styl.Compile(src, styl.Options{Pretty: true})
// or styl.CompileFile("styles.styl", opts) / styl.CompileReader(r, opts)

// With a source map (self-contained: the original source is embedded):
css, mapJSON, err := styl.CompileMap(src, styl.Options{Filename: "app.styl", OutFile: "app.css"})
// or styl.CompileFileMap("app.styl", opts)

// Build/BuildFile also report the files read (for cache invalidation):
res, err := styl.BuildFile("app.styl", styl.Options{SourceMap: true})
// res.CSS, res.Map, res.Deps ("app.styl" plus every inlined @import)

// Compile from an embedded filesystem (imports resolve inside it):
//go:embed styles/*.styl
// var styles embed.FS
css, err = styl.CompileFile("styles/app.styl", styl.Options{FS: styles})
```

`Options`:

| Field | Meaning |
| --- | --- |
| `Pretty` | Expanded, human-readable output (otherwise compressed). |
| `MergeDuplicates` | Fold rules with identical bodies into one selector group. |
| `IncludePaths` | Extra directories searched for `@import`. |
| `BaseDir` | Directory relative `@import` paths resolve against (defaults to `Filename`'s dir). |
| `Filename` | Source path, used in errors, to derive `BaseDir`, and as the map's `sources` entry. |
| `OutFile` | Generated CSS filename recorded in the source map's `file` field. |
| `FS` | An `fs.FS` (e.g. `embed.FS`) that sources and `@import` resolve through instead of the OS. |
| `SourceMap` | Ask `Build`/`BuildFile` to also produce a source map. |
| `Globals` | Go values seeded as root-scope variables before the sheet runs (see [Runtime theming](#runtime-theming)). |
| `CustomProperties` | Variables to expose as CSS custom properties on `:root` (see [Runtime theming](#runtime-theming)). |
| `Sandbox` | Confine a compile of untrusted source: no OS filesystem, vetted imports, time/step/size budgets (see [Untrusted themes](#untrusted-themes-sandbox)). |

## Runtime theming

Because go-styl compiles in-process in microseconds, stylesheets can be
parameterized *by your application at runtime* — per tenant, per user, per
A/B arm — instead of baked at build time. Two options (both go-styl
extensions) work together:

**`Globals`** seeds Go values as variables in the sheet's root scope. Strings
are parsed as Stylus value expressions, so colors, units, lists, and even
function calls work; numbers become unitless numbers, bools are bools. In the
sheet, declare overridable defaults with `?=`:

```stylus
// theme.styl
primary ?= #06c        // default; a Global overrides it
radius  ?= 4px

.btn
  background primary
  border-radius radius
```

```go
css, _ := styl.Compile(src, styl.Options{
    Globals: map[string]any{"primary": "#0af", "radius": "8px"},
})
```

**`CustomProperties`** lists theme tokens to expose as CSS custom properties.
The output gains a leading `:root` block declaring each token, and direct
references compile to `var(--name)` instead of inlining — so the compiled CSS
can be re-themed *in the browser* (dark mode, user themes) without
recompiling:

```go
css, _ := styl.Compile(src, styl.Options{
    Globals:          map[string]any{"primary": "#0af"},
    CustomProperties: []string{"primary", "radius"},
})
```

```css
:root{--primary:#0af;--radius:4px}
.btn{background:var(--primary);border-radius:var(--radius)}
```

Uses that must compute at compile time — arithmetic (`radius * 2`),
built-in calls (`darken(primary, 20%)`), comparisons, `{interpolation}`, and
media queries — use the token's compile-time value; direct references
(including inside value lists, `font: 14px/lh` shorthands, and pass-through
CSS functions like `translateX(x)`) stay `var(--name)`. Mixin arguments keep
the `var()` reference too, so `bordered(primary)` emits
`border: 1px solid var(--primary)`.

Both options are available on the CLI (`-D name=value`, `-cssvar name`) and
in the HTTP middleware (`stylserve.Options.Globals` / `.CustomProperties` —
fixed per engine, so cached output stays valid; run one engine per theme for
per-tenant CSS).

### Untrusted themes (`Sandbox`)

To compile stylesheets your *users* write (Discourse-style theme editors),
set `Options.Sandbox`. The compiler is pure Go with no plugin hook, so this is
a hardening pass rather than a jail:

```go
css, err := styl.Compile(tenantSrc, styl.Options{
    FS:      sharedPartials,             // the only filesystem imports can see
    Globals: tenantTokens,
    Sandbox: &styl.Sandbox{
        Context:     r.Context(),        // cancel with the request
        AllowImport: func(p string) bool { return strings.HasPrefix(p, "shared/") },
    },
})
if errors.Is(err, styl.ErrLimit) {
    // too expensive, or reached for a file it may not: reject the theme
}
```

| Limit | Default | Stops |
|---|---|---|
| no OS filesystem | — | `@import` and `CompileFile` read only `Options.FS` (no FS → imports fail) |
| `AllowImport` | all of `FS` | imports outside an allowlist |
| `Timeout` / `Context` | 2s | wall-clock runaways |
| `MaxSteps` | 1,000,000 statements | nested loops (each capped, but they multiply) — deterministic |
| `MaxValueBytes` | 256 KiB | doubling bombs (`s = s + s`, `l = l l`) |
| `MaxSourceBytes` | 1 MiB (sheet + imports) | oversized input |
| `MaxImports` | 256 | import fan-out |
| `MaxOutputBytes` | 4 MiB (incl. `@extend` grafts) | small sheets that render huge |

`0` means the default, a negative value means unlimited. `warn()` goes to
`Options.Warn` or is dropped — never to the host's stderr. The sandbox adds
to limits every compile has (call depth 256, 16384 selectors per rule,
ranges of 65536). Every entry point honors it (`Compile`, `Build`,
`CompileMap`, `Extract`, `Prune`, `Component`, `Migrate` and their `*File`
forms). Errors keep their `file:line:col` position.

## Typed class names (`styl gen`)

`styl gen` compiles a stylesheet and emits a Go package of constants for
every class name, element ID, `@keyframes` name, and root-level variable that
appears in the output — CSS Modules-style typo-proofing for server-rendered
Go. Wire it up with `go:generate`:

```go
//go:generate go run github.com/rohanthewiz/go-styl/cmd/styl gen -pkg css -o css_gen.go app.styl
```

```stylus
// app.styl
primary = #0af

.card
  background primary
  .card__title--big
    font-weight: bold

@keyframes spin
  from
    transform: rotate(0)
```

generates

```go
// Code generated by styl gen from app.styl; DO NOT EDIT.
package css

// Class names.
const (
	Card         = "card"
	CardTitleBig = "card__title--big"
)

// Keyframes animation names (suffix Anim).
const (
	SpinAnim = "spin"
)

// Variable values at compile time (suffix Var).
const (
	PrimaryVar = "#0af" // primary
)
```

so [element](https://github.com/rohanthewiz/element) markup can write
`b.Div("class", css.Card)` — a typo is a compile error, and renaming a class
in the stylesheet surfaces every stale reference. Names become identifiers by
splitting on non-alphanumerics and title-casing (`card__title--big` →
`CardTitleBig`); IDs, keyframes, and variables carry `ID`/`Anim`/`Var`
suffixes so groups can't collide, and any two names that would map to the
same constant fail the generation with an error naming both.

The same data is available programmatically: `styl.Extract` /
`styl.ExtractFile` return a `Manifest` (sorted classes, IDs, keyframes names,
and root variables with their final values, honoring `Globals` and
`@import`), and `Manifest.GoSource(pkg)` renders the constants file.

## Scoped component styles (`styl.Component`)

`styl.Component` compiles a stylesheet as a component: every class name and
`@keyframes` name gets a suffix hashed from the source, so two components can
both style `.title` or animate `fade` without colliding. It is CSS Modules for
server-rendered Go, with no bundler.

```stylus
.card
  padding 1rem
  animation fade .2s
  .title
    font-weight bold
  & :global(.htmx-request)
    opacity .5

@keyframes fade
  from
    opacity 0
```

```go
//go:embed card.styl
var cardSrc string

var card = must(styl.Component(cardSrc, styl.Options{}))

// card.CSS:
//   .card_ulwiepau { padding: 1rem; animation: fade_ulwiepau 0.2s; }
//   .card_ulwiepau .title_ulwiepau { font-weight: bold; }
//   .card_ulwiepau .htmx-request { opacity: 0.5; }
//   @keyframes fade_ulwiepau { ... }

b.Div("class", card.Class("card")).R(
	b.H2("class", card.Class("title")).T("Hello"),
)
```

- **Renamed:** `.class` tokens in selectors (including inside `:not(...)`,
  `@media`, `@extend` grafts and merged duplicates), `@keyframes` names
  (vendor-prefixed too), and references to those keyframes in `animation` /
  `animation-name` values.
- **Left alone:** element IDs, type and attribute selectors, and anything in
  `:global(...)`. The wrapper is dropped: `.card :global(.is-open)` becomes
  `.card_x .is-open`. A nested selector starting with `:` attaches to its
  parent, so write `& :global(.x)` for a descendant. Animations naming
  keyframes defined elsewhere keep their name.
- **The suffix** is the first 40 bits of the SHA-256 of the source text, as 8
  base32 characters. `Options` doesn't feed it, so per-request `Globals`
  never change the names. Editing the file does change them.

`Scoped.Names` maps each local name to its scoped form. `Scoped.Class(names...)`
joins them for a `class` attribute and passes unknown names through, so you
can mix in global utility classes.

For compile-checked references, `styl gen -scoped` runs `styl.ComponentFile`
and feeds its `Manifest` (with `Manifest.Scoped` set) through the same
`GoSource` renderer:

```go
//go:generate go run github.com/rohanthewiz/go-styl/cmd/styl gen -scoped -pkg cardcss -o cardcss/gen.go -css card.css card.styl
```

```go
const (
	Card        = "card_ulwiepau" // card
	HtmxRequest = "htmx-request"
	Title       = "title_ulwiepau" // title
)
const (
	FadeAnim = "fade_ulwiepau" // fade
)
```

The hash depends only on the source text, so a runtime
`styl.Component(cardSrc, …)` of the same embedded file produces exactly these
names. (The names shown are the real ones for `card.styl` exactly as above.) `-css` also writes the scoped CSS, for serving it as a static file.

## Critical CSS (`styl.Prune`)

Because HTML rendering and CSS compilation both run in-process, go-styl can do
per-response critical CSS without the headless browser the Node world needs:
render the page, collect the names it actually uses, and compile a stylesheet
containing only the matching rules — small enough to inline in `<head>`.

```go
page := renderPage()                            // element, templates, …
css, err := styl.Prune(src, styl.UsedFromHTML(page), styl.Options{})
// <style>…critical css…</style> straight into the response
```

`UsedFromHTML` scans rendered HTML for tag names, classes, and IDs.
`Prune` (and `PruneFile`) compiles like `Compile` — `Globals`,
`CustomProperties`, `@import`, `fs.FS` all honored — then keeps a selector
only if every class, ID, and tag it requires is present in the `Used` set.
Pruning errs toward keeping:

- Selectors that require nothing survive: `@font-face`, `:root`, `*`,
  attribute-only selectors like `[data-theme="dark"]`.
- Names inside functional pseudo-class arguments are never required —
  `.btn:not(.disabled)` needs only `btn`.
- A nil `Used` axis means "unknown, don't prune on it": leave `Tags` nil when
  pruning against an HTML *fragment* rather than the whole page.
- `@keyframes` are kept only while a surviving `animation`/`animation-name`
  declaration references them (vendor-prefixed and quoted names included),
  and at-rules emptied by pruning disappear.

Compiles cost microseconds, so pruning per response is practical — and the
[`stylcrit`](stylcrit/) package packages the whole flow as a cached engine
for middleware use:

```go
crit := stylcrit.New(stylcrit.Options{
    Path:     "styles/app.styl",
    Safelist: styl.Used{Classes: []string{"menu--open"}}, // names JS toggles
})
page, err := crit.Inline(renderPage()) // <style> injected before </head>
```

Output is cached by the page's used-name set (pages sharing a layout compile
once) and invalidated when the stylesheet or any `@import` changes. Two
middleware adapters apply it to every HTML response: `stylhttp.Critical` for
`net/http`, and rweb's `middleware/critical`.

```go
handler := stylhttp.Critical(stylcrit.Options{Path: "styles/app.styl"})(mux)
http.ListenAndServe(":8080", handler)
```

`stylhttp.Critical` rewrites only 2xx, uncompressed `text/html` responses
(sniffing the type when the handler sets none); anything else, such as JSON,
event streams or gzip, passes through unbuffered. A rewritten page drops its
`Content-Length` and `ETag`.

## Migrating off Stylus (`styl migrate`)

`styl migrate` converts a Stylus sheet into modern CSS you can keep editing
by hand. Unlike a compile, it keeps the sheet's structure:

- **Nesting stays nesting**, as native CSS nesting. A bare nested `:hover`
  becomes `&:hover`, since go-styl attaches it (CSS nesting would read a
  bare `:hover` as a descendant).
- **Root-level variables become custom properties** on `:root`, and direct
  references become `var(--name)`. Arithmetic stays live as `calc()` when
  the units allow it (`base * 10` → `calc(var(--base) * 10)`). That holds
  in `:root` too (`mid = top * 2` → `--mid: calc(var(--top) * 2)`). Only
  variables the output reads get a property.
- **Stylus-only constructs are resolved in place**: mixins expand, `for`
  loops unroll, `if` picks its branch, `@extend` adds the extending
  selectors to the target rule's list, and `.styl` imports are inlined.
- **Each of those spots gets a `/* styl-migrate: … */` comment**, and the
  same notes go to stderr as `file:line:col: kind: msg`, so a reviewer can
  find what changed shape.

```stylus
primary = #0af
base = 8px

button(bg)
  background bg
  &:hover
    background darken(bg, 10%)

.card
  color primary
  width base * 10
  :hover
    color red
  &__body
    padding base
  .btn
    button(primary)
```

`styl migrate card.styl`:

```css
/* Migrated from card.styl by styl migrate. Review each styl-migrate note. */

:root {
  --primary: #0af;
  --base: 8px;
}

.card {
  color: var(--primary);
  width: calc(var(--base) * 10);

  &:hover {
    color: red;
  }

  .btn {
    /* styl-migrate: mixin: expanded button(var(--primary)) */
    background: var(--primary);

    &:hover {
      /* styl-migrate: frozen: background computed at migrate time from --primary; it won't follow runtime changes */
      background: #0099e6;
    }
  }
}

/* styl-migrate: hoisted: "&__body" can't nest in CSS; moved after the enclosing block as .card__body (check cascade order) */
.card__body {
  padding: var(--base);
}
```

Some things CSS nesting can't express. They are written out in full after
the enclosing block (wrapped in the same `@media`), with a `hoisted` note:

- `&` concatenation such as BEM `&__elem` or `&-mod`. CSS `&` is a whole
  selector, never a name prefix.
- Rules nested under a pseudo-element (`a::before { &:hover … }`).
- `@keyframes`, `@font-face` and `@page` inside a rule.

A value computed from a variable (`darken(primary, 10%)`, `8px + 2`, which
is not valid `calc()`) is correct but fixed. It gets a `frozen` note because
it won't follow `--primary` at runtime.

Library: `styl.Migrate(src, opts, styl.MigrateOptions{})` or `MigrateFile`
returns `MigrateResult{CSS, Notes, Deps}`. `MigrateOptions.NoNotes` drops
the inline comments (the notes are still returned). `NoVars` inlines every
variable. `Options` supplies import resolution and `Globals`, which inline
as fixed inputs.

Two limits apply. Stylus comments are not carried over, because the parser
drops them. Imports are inlined into one output file.

Every example and fixture sheet is checked in `TestMigrateRoundTrip`. The
migrated CSS, with nesting, `var()` and `calc()` resolved, must declare
exactly what `Compile` declares.

## Formatting (`styl fmt`)

`styl fmt` gives Stylus a gofmt. It rewrites layout and spelling, never
meaning:

- indentation becomes two spaces per nesting level; brace syntax gets its
  level from the braces
- runs of spaces inside a line collapse to one, except in strings and
  comments
- trailing whitespace goes, blank-line runs collapse to one, and the file
  ends in exactly one newline
- comments stay, re-indented with the code around them
- declarations are spelled one way per file: `prop: value` inside braces,
  and in indentation syntax whichever of `prop value` / `prop: value` most
  of the file already uses (a tie leaves each as written); `color:red`
  becomes `color: red`
- assignments and parameter defaults get ` = ` / ` ?= ` (`x=1` → `x = 1`,
  `m(a=1)` → `m(a = 1)`)
- commas in values get one space after and none before (`f(1 ,2)` →
  `f(1, 2)`); selectors, strings and `url()` are left alone

```shell
go run ./cmd/styl fmt app.styl          # formatted source to stdout
go run ./cmd/styl fmt -w styles/*.styl  # rewrite changed files in place
go run ./cmd/styl fmt -l styles/*.styl  # list files that aren't formatted (CI)
go run ./cmd/styl fmt < app.styl        # stdin → stdout
```

Every result is checked before it is returned. It must parse to the same
stylesheet as the input, source positions aside. If a layout confuses the
re-indent, fmt falls back to fixing only trailing whitespace and blank
lines. If even that would change the meaning, it reports an error and
leaves the file alone. A file that doesn't parse is not formatted, and its
parse error is reported instead.

A continuation line (after a trailing comma, or inside a multi-line object
literal) keeps its offset from the first line of its statement, so
hand-aligned value lists stay aligned. In a mixed-syntax file, lines outside
every brace are re-indented by the same depth rule the parser applies to
them (a braced block never adopts indented children, so a line indented
under one lines up with it).

Each respelled line is checked the same way: if a change would alter the
stylesheet, only that line keeps its old spelling.

Library: `styl.Format(src)`.

## Editor support (`styl-lsp`)

`styl-lsp` is a Language Server Protocol server built on the compiler. It
is a single static binary with no dependencies:

```shell
go install github.com/rohanthewiz/go-styl/cmd/styl-lsp@latest
```

It provides:

- **Diagnostics as you type**: the real compile's errors, with did-you-mean
  hints. An error inside an imported file is reported on the `@import`
  line. `warn()` output shows as warnings.
- **Completion** of variables and mixins, including those from imported
  files, plus built-ins and keywords. A root variable's detail is its
  computed value. Where a property name goes (an indented line, or after
  `{`/`;`), CSS property names are offered too.
- **Hover** showing a variable's definition and its computed value
  (`gap = pad * 3` → `12px`), or a mixin's signature.
- **Go to definition** across `@import`/`@require`. Lookup is scope-aware,
  so a local or a parameter shadows a global.
- **Find references** and **rename** across every file of the compile.
  Imported files share the importer's root scope, so a variable a partial
  reads from another partial is found too. A local is its own symbol;
  every root-level assignment of a name is the same variable. A property
  that names a mixin (a transparent mixin call) counts as a use.
- **Signature help** for your mixins and functions while arguments are
  typed.
- **Lint**: unused local variables, unused local functions, unused root
  mixins in an entry sheet (not in a `_partial` or a file another open
  document imports), and a property set twice in one block. The fallback
  idiom (`display -webkit-box` then `display flex`) is allowed. Root
  variables are never flagged, because `styl gen` and custom properties
  make them public.
- **Document symbols** (the outline): selectors, at-rules, mixins and
  variables.
- **Color swatches** for hex literals, with a color picker that writes hex
  or `rgba()`.
- **Formatting**, identical to `styl fmt`.

Each analysis compiles in a [`Sandbox`](#untrusted-themes-sandbox) with a
1s budget, so a half-typed `for` over a huge range is cut off with a
warning instead of hanging the editor. While the text doesn't parse,
completion, hover and definition keep working from the last good parse.
Imports are compiled from the editor's text of any open file, so an
unsaved edit to a partial updates the importer's diagnostics and hover
values right away. Closing it without saving reverts them to the disk text.

Point any LSP client at it for `*.styl`. Neovim:

```lua
vim.api.nvim_create_autocmd("FileType", {
  pattern = "stylus",
  callback = function() vim.lsp.start({ name = "styl", cmd = { "styl-lsp" } }) end,
})
```

Helix (`languages.toml`):

```toml
[language-server.styl-lsp]
command = "styl-lsp"

[[language]]
name = "stylus"
scope = "source.stylus"
file-types = ["styl"]
language-servers = ["styl-lsp"]
```

`-log <file>` sends the server's log to a file (default: stderr).

## Serving over HTTP

Both middleware adapters compile on first request and cache, recompiling when
the source **or any of its `@import`s** change. ETags give you free 304s, and
`SourceMaps: true` serves `<name>.css.map` alongside for DevTools.

With the standard library (`GET /css/app.css` compiles `./styles/app.styl`):

```go
import (
    "github.com/rohanthewiz/go-styl/stylhttp"
    "github.com/rohanthewiz/go-styl/stylserve"
)

mux.Handle("/css/", http.StripPrefix("/css/",
    stylhttp.New(stylserve.Options{Dir: "./styles", SourceMaps: true})))
```

With [rweb](https://github.com/rohanthewiz/rweb) — the adapter lives in the
rweb repo so this module stays serr-only:

```go
import (
    "github.com/rohanthewiz/go-styl/stylserve"
    "github.com/rohanthewiz/rweb/middleware/stylus"
)

s.Get("/css/*path", stylus.Handler(stylserve.Options{Dir: "./styles"}))
```

Or ship the stylesheets inside the binary:

```go
//go:embed styles/*.styl
var styles embed.FS

sub, _ := fs.Sub(styles, "styles")
s.Get("/css/*path", stylus.Handler(stylserve.Options{FS: sub}))
```

`stylserve.Options`: `Dir` or `FS` (source root), `IncludePaths`, `Pretty`
(default compressed), `MergeDuplicates`, `SourceMaps`, plus `Globals` and
`CustomProperties` for [runtime theming](#runtime-theming). Compile errors
return `500` with the positioned message; unknown paths return `404`.

Per-request themes (a tenant's brand color, a user's preference) layer extra
globals over `Options.Globals`. Each distinct variable set compiles once and
is cached, up to `MaxVariants` sets (default 256):

```go
mux.Handle("/css/", http.StripPrefix("/css/", stylhttp.NewWithGlobals(
    stylserve.Options{Dir: "./styles"},
    func(r *http.Request) map[string]any {
        return map[string]any{"brand": tenantFor(r.Host).Brand}
    },
    "Host", // sent as Vary; with no vary list responses are Cache-Control: private
)))
```

Other frameworks call `stylserve.Engine.AssetWith(path, globals)` directly.

In development, `LiveReload: true` swaps in fresh CSS as you edit, with no
page reload: include the script once, next to your stylesheet links.

```go
mux.Handle("/css/", http.StripPrefix("/css/",
    stylhttp.New(stylserve.Options{Dir: "./styles", LiveReload: true})))
```

```html
<link rel="stylesheet" href="/css/app.css">
<script src="/css/_live.js"></script>
```

The script opens an event stream per stylesheet (`/css/_live?css=app.css`).
The server re-checks the sheet twice a second, sends a change when a source
or `@import` changes, and sends compile errors to the browser console while
keeping the last good CSS applied. Keep it off in production.

## CLI

```shell
go run ./cmd/styl input.styl            # pretty CSS to stdout
go run ./cmd/styl -compress input.styl  # minified
go run ./cmd/styl -merge input.styl     # merge duplicate rule bodies
go run ./cmd/styl -o out.css input.styl # write to a file
go run ./cmd/styl -o out.css -sourcemap input.styl  # also writes out.css.map
go run ./cmd/styl -D primary=#0af -D 'pad=2 * 8px' input.styl  # define globals
go run ./cmd/styl -cssvar primary -cssvar pad input.styl       # expose as --vars
go run ./cmd/styl gen -pkg css -o css_gen.go input.styl        # Go constants file
go run ./cmd/styl gen -scoped -css out.css input.styl          # scoped component: hashed names + CSS
go run ./cmd/styl migrate -o out.css input.styl                # Stylus → modern nested CSS
go run ./cmd/styl fmt -w input.styl                            # format in place
```

`-sourcemap` requires `-o`; it writes `<out>.map` next to the CSS and appends a
`/*# sourceMappingURL=… */` comment. `-D` and `-cssvar` are repeatable (see
[Runtime theming](#runtime-theming)). The `gen` subcommand emits typed
class/ID/keyframes/variable constants instead of CSS (see
[Typed class names](#typed-class-names-styl-gen)); it takes `-o`, `-pkg`
(default `css`), and `-D`. `-scoped` compiles the sheet as a scoped
component, so class and keyframes constants hold the hashed names, and
`-css <file>` also writes the scoped CSS (see
[Scoped component styles](#scoped-component-styles-stylcomponent)).
`migrate` converts the sheet to modern nested CSS for leaving Stylus (see
[Migrating off Stylus](#migrating-off-stylus-styl-migrate)). It takes `-o`,
`-no-notes` (omit the inline review comments), `-no-vars` (inline all
variables), `-q` (don't list the notes on stderr) and `-D`. `fmt` formats
source (see [Formatting](#formatting-styl-fmt)). It takes `-w` (rewrite
files in place) and `-l` (list unformatted files).

## Example

```stylus
base = 10px

body
  width base * 2
  color rgba(0, 0, 0, 0.5)

  a
    color blue

    &:hover
      color red
```

compiles to:

```css
body {
	width: 20px;
	color: rgba(0,0,0,0.5);
}

body a {
	color: blue;
}

body a:hover {
	color: red;
}
```

## Examples

The [`examples/`](examples/) directory holds runnable, feature-by-feature samples
(variables, nesting, mixins, control flow, built-ins, interpolation, `@extend`,
at-rules, brace syntax, and `@import`). Compile any of them:

```shell
go run ./cmd/styl examples/08-at-rules.styl
go run ./cmd/styl -compress examples/05-builtins.styl
```

See [`examples/README.md`](examples/README.md) for the full index.

## Playground

**Try it live: <https://rohanthewiz.github.io/go-styl/>**

[`playground/`](playground/) is a browser playground — the compiler built for
WebAssembly behind a two-pane live editor with the examples bundled in:

```sh
./playground/build.sh        # builds playground/styl.wasm (+ wasm_exec.js)
go run ./playground/serve    # http://localhost:8080
```

A GitHub Actions workflow ([pages.yml](.github/workflows/pages.yml)) deploys it
to GitHub Pages on push.

## Limitations

Things to be aware of:

- Source maps map at selector / declaration / at-rule granularity (column-accurate
  for those, including compressed output); they do not yet map inside values.
- Inside `calc(...)`, bare Stylus variables are *not* evaluated (as in
  Stylus) — use interpolation: `calc(100% - {gutter})`, or `s()`/`%`.
  `url(...)` evaluates its contents when they reference a variable
  (`url(base + "x.png")` → `url("/img/x.png")`) and otherwise passes through
  verbatim. (`@media` query values *are* evaluated: `@media (min-width: bp)`.)
- Arithmetic in a `@media` query needs interpolation: `@media (min-width: {bp * 2})`.
- In brace syntax, a stand-alone `{expr}` in value position is not supported —
  use the bare variable (`width x`, not `width {x}`).
- The `MergeDuplicates` pass is a non-standard extra-compression option (off by
  default); standard Stylus does not fold identical rule bodies.
- Function/mixin call depth is capped at 256 and a rule's combined selector
  count at 16384, so unbounded recursion errors out instead of hanging.
- In brace syntax, statements that share a source line with an earlier one
  (one-liner blocks) report approximate positions; multi-line files are exact.

## Compatibility with reference Stylus

`difftest/` differentially tests go-styl against the reference Node.js
[stylus](https://www.npmjs.com/package/stylus) compiler: every corpus file
(`examples/`, `testdata/`, `difftest/corpus/`) is compiled by both, outputs are
normalized (formatting-only differences like `white` vs `#fff` are erased), and
the test reports a compatibility score. Known divergences — go-styl extensions,
missing features, and reference-stylus failures — are cataloged with notes in
[`difftest/known_diffs.txt`](difftest/known_diffs.txt). The test is a ratchet:
it fails when an unlisted file diverges *and* when a listed file starts
matching, so the score only moves up. CI runs it on every push.

```sh
npm install --prefix difftest   # once: fetches the reference compiler
go test -v ./difftest           # prints the compatibility score
```

(The test skips itself when node or the stylus package is absent.)

### Extensions

go-styl accepts a few things reference Stylus does not. Each is syntax
Stylus rejects or leaves literal, so it can't change a sheet that works in
Stylus, with two unlikely exceptions noted below. They are kept on
purpose: they make common tasks one-liners, and several workarounds in
this README rely on them.

- **`{expr}` interpolation in more places**: `@media` preludes
  (`@media (min-width: {bp * 2})`), inside `calc()` and `url()`
  (`calc(100% - {gutter})`), and as a lone value (`width {x}`, indented
  syntax only). Stylus fails to parse these.
- **`{expr}` inside quoted strings** (`content "v{major}"`), substituted
  only when the braces reference a defined variable. Stylus keeps string
  text literal, so `"{nope}"`, `"{1 + 2}"` and the escaped `"\{major}"`
  stay literal in go-styl too; only a string that literally means
  `{major}` while `major` is a variable differs (escape it as `\{major}`).
- **Single-line functions** `double(x) = x * 2`. Stylus doesn't read this
  as a definition and leaves `double(15px)` in the output.
- **`Options.Globals` / `Options.CustomProperties`** (runtime theming),
  `Options.Sandbox` (untrusted themes), and the extra-compression
  `MergeDuplicates` pass.

The difftest pins each one in `difftest/known_diffs.txt`.

### Deliberate differences

Where reference Stylus is silent about a likely mistake, or fails on valid
CSS, go-styl chooses differently on purpose:

- **Calling an undefined mixin is an error.** Stylus silently drops a
  statement-level `sermon()` or `+sermon` when no such mixin exists; go-styl
  reports `undefined mixin "sermon"` (with a did-you-mean hint when a close
  name exists). This catches typos and dead calls; delete the call or define
  the mixin.
- **Backslash-escaped quotes stay inside a string** (`'it\'s'`); Stylus
  fails to parse them.
- **A `;` inside parentheses is kept** in both syntaxes
  (`url(data:image/png;base64,…)`); Stylus fails to parse an unquoted data
  URI.
- **`@extend .x;`** accepts a trailing `;`; Stylus reads it as part of the
  selector and fails to extend.
- **Numbers are always rounded to 15 decimals** when printed. Stylus skips
  this for compressed values between -1 and 1 and prints
  `.30000000000000004`. Very small numbers print in decimal form
  (`.00000001`) where Stylus prints JavaScript's `1e-8`.
- **A bare nested pseudo-class attaches to its parent.** `.btn` over
  `:hover` gives `.btn:hover`, the same as `&:hover`. Stylus reads it as a
  descendant (`.btn :hover`, any hovered element inside `.btn`), which is
  almost never intended. Write `& :hover` for the descendant form.
- **`current-media()` returns the query as written**
  (`'@media screen and (max-width: 100px)'`). Stylus 0.64 wraps each part in
  extra parentheses (`'@media (screen and (max-width: (100px)))'`).
  **`selector-exists()`** only sees rules compiled before the call (0.64
  crashes on nested rules).
- **Lists stay values.** `push(l, x)` (and `pop`/`shift`/`unshift`) rebind the
  variable `l`, as in Stylus, but `push` returns the new list rather than its
  length, and after `b = a`, `push(b, x)` changes only `b` (Stylus mutates
  the one list both names share). Objects, by contrast, are shared by
  reference exactly as in Stylus.
- **An object is not a property value.** `width theme` is an error pointing
  at `theme.key`; Stylus prints a JSON-like blob.
- **`in` keeps CSS meaning in values.** In a property value or function
  arguments, `x in word` where `word` is not a variable stays text, so
  `linear-gradient(to right in oklch, …)` compiles as written; Stylus would
  evaluate it as a membership test.
- **Comment lines never affect structure.** In Stylus, the indentation of a
  `//` line counts: a column-0 comment between `m()` and its body ends the
  definition (the body lands at the root), and a comment indented deeper
  than the line above can re-nest what follows or drop a declaration.
  go-styl ignores comment lines when reading indentation.

## Architecture

```
.styl  →  parser (brace→indent normalize, indentation line-tree + Pratt expr parser)
       →  ast
       →  eval (lexical scope, variable inlining, arithmetic, builtins, at-rules)
       →  css (node tree → position-tracking render, optional merge + source map)
       →  CSS (+ optional .map)
```

Packages live under `internal/`: `token`, `lexer`, `ast`, `parser`, `value`, `eval`,
`builtin`, `css`, and `lsp` (the language server behind `cmd/styl-lsp`).

## Roadmap

- [x] **M1** Vertical slice: lexer, parser, scoped evaluator, arithmetic, nesting, one builtin
- [x] **M2** Control flow (`if`/`else`/`for`) and parametric functions & mixins
- [x] **M3** Built-in function library (color / math / list / string / type)
- [x] **M4** Interpolation (`{expr}`), `@extend` / `$placeholder` selectors, `@import`
- [x] **M5** At-rules (`@media` / `@keyframes` / `@font-face` / …), brace syntax, compress parity
- [x] **M6a** Correctness: `url()`/`calc()`, `!important`, media-query variables,
  bracket-aware selector splitting, whitespace-sensitive `-`/`+`
- [x] **M6b** Source maps (Source Map v3, `CompileMap` / `-sourcemap`)
- [x] **M7** Positioned errors (`file:line:col`, serr attributes) + fuzz hardening
- [x] **M8** `fs.FS`/embed sources, `Build` API (deps), HTTP middleware (`stylserve`/`stylhttp`)
- [x] **M9** Differential testing vs reference Stylus (compatibility score in CI)
- [x] **M10** Stylus parity: for-loop binding order, adjust()/mix color math,
  literal `/` in property values, `**`, ranges (`1..3`), color arithmetic,
  implicit returns, transparent mixins, `spin()`, compressed zero-unit strip
- [x] **M11** WASM playground (`playground/`, deployed via GitHub Pages)
- [x] **M12** Benchmarks: Go bench suite + `bench/` comparison vs reference stylus
- [x] **M13** Runtime theming: `Globals` (Go values as Stylus variables) +
  `CustomProperties` (theme tokens as `:root` `--vars`, direct refs → `var(--name)`)
- [x] **M14** `styl gen` codegen: typed Go constants for classes/IDs/keyframes/variables
  (`styl.Extract` + `Manifest.GoSource`)
- [x] **M15** Critical CSS: `styl.Prune` / `PruneFile` + `UsedFromHTML` — per-response
  stylesheets containing only the rules the rendered page uses
- [x] Scoped component styles: `styl.Component` / `ComponentFile` (hashed class and
  keyframes names, `:global(...)`), `styl gen -scoped`
- [x] `styl migrate`: Stylus → modern CSS (native nesting, `:root` custom properties,
  `calc()`, review notes for everything resolved at migrate time)
- [x] `styl fmt` (`styl.Format`) and `styl-lsp`, a language server: diagnostics, completion,
  hover with computed values, go-to-definition, references and rename across imports,
  signature help, lint, symbols, color swatches
- [ ] Future: value-level source mapping, deeper compress parity, more built-ins

## License

See [LICENSE](LICENSE).
