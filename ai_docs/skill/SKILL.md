---
name: go-styl-stylus-compiler
description: >-
  go-styl is a pure-Go (zero-dependency, no Node.js, no cgo) compiler for the
  Stylus (.styl) CSS preprocessor. Use it to author .styl stylesheets and compile
  them to CSS from Go code or the CLI. This skill covers the Go API
  (Compile/CompileFile/CompileReader + Options), the styl CLI, and the full
  supported Stylus language: variables, unit arithmetic, nesting, mixins/functions,
  control flow, the built-in function library, interpolation, @extend/$placeholder,
  @import, at-rules (@media/@keyframes/@font-face), both the indentation and the
  CSS-like brace syntax, plus url()/calc()/!important. Reach for this whenever you
  need to write .styl, integrate the compiler, or debug its output/errors.
---

# go-styl — pure-Go Stylus compiler

`github.com/rohanthewiz/go-styl` parses Stylus source into an AST, evaluates it
(variables, arithmetic, built-ins, control flow — all at compile time with lexical
scoping), and renders CSS. Variables are **inlined**, matching Stylus semantics
(no `:root{--x}`/`var()` emission).

Pipeline: `.styl → parser (brace→indent normalize, line-tree + Pratt expr) → ast →
eval → css (node tree → render) → CSS`.

Runnable, feature-by-feature samples live in [`examples/`](../../examples/).

---

## 1. Using the compiler

### Go library

```go
import styl "github.com/rohanthewiz/go-styl"

css, err := styl.Compile(src, styl.Options{Pretty: true})
css, err := styl.CompileFile("styles.styl", styl.Options{Pretty: false})
css, err := styl.CompileReader(r, styl.Options{})        // r is an io.Reader
```

All three return `(string, error)`. Compile errors are **positioned** —
`app.styl:7:3: undefined mixin "foo"` (file falls back to `<input>`), with
"did you mean" hints for misspelled mixins — and are wrapped with
[serr](https://github.com/rohanthewiz/serr), carrying `file`/`line`/`col` as
structured attributes (`serr`-aware loggers extract them automatically).

**Build / BuildFile** return a `Result{CSS, Map, Deps}` where `Deps` lists every
file the build read (the input plus inlined `@import`s) — the hook for cache
invalidation. Set `Options.SourceMap` to also populate `Result.Map`.

**`fs.FS` sources** — set `Options.FS` (e.g. an `embed.FS`) and
`CompileFile`/`BuildFile`/`@import` all resolve inside it; a leading `/` on an
import path means the FS root.

**Source maps** (Source Map v3) — `CompileMap`/`CompileFileMap` return the CSS plus
a JSON map (selectors, declarations, and at-rule headers map back to the `.styl`,
column-accurate, including compressed output). The original source is embedded
(`sourcesContent`), so the map is self-contained.

```go
css, mapJSON, err := styl.CompileMap(src, styl.Options{Filename: "app.styl", OutFile: "app.css"})
css, mapJSON, err := styl.CompileFileMap("app.styl", styl.Options{OutFile: "app.css"})
```

`Options`:

| Field | Type | Meaning |
| --- | --- | --- |
| `Pretty` | `bool` | Expanded, human-readable output. `false` ⇒ compressed/minified. |
| `MergeDuplicates` | `bool` | Fold rules with identical declaration bodies into one comma selector group. **Non-standard** extra compression; off by default. |
| `IncludePaths` | `[]string` | Extra directories searched for `@import`. |
| `BaseDir` | `string` | Directory relative `@import` paths resolve against. Defaults to `Filename`'s directory. |
| `Filename` | `string` | Source path; used in errors and to derive `BaseDir`. `CompileFile` sets it automatically. |
| `FS` | `fs.FS` | Filesystem sources/`@import` resolve through (e.g. `embed.FS`) instead of the OS. |
| `SourceMap` | `bool` | Ask `Build`/`BuildFile` to also produce a source map. |

### CLI (`cmd/styl`)

```shell
go run ./cmd/styl input.styl              # pretty CSS to stdout
go run ./cmd/styl -compress input.styl    # minified
go run ./cmd/styl -merge input.styl       # merge duplicate rule bodies
go run ./cmd/styl -o out.css input.styl   # write to a file
go run ./cmd/styl -o out.css -sourcemap input.styl  # also writes out.css.map
```

`-sourcemap` requires `-o`; it writes `<out>.map` and appends a
`/*# sourceMappingURL=… */` comment to the CSS.

### Serving over HTTP

Two adapters compile `.styl` on request and cache (invalidated when the source
or any `@import` changes), with ETag/304 support and optional dev source maps
(`SourceMaps: true` serves `<name>.css.map` and appends `sourceMappingURL`).
Compile errors → `500` with the positioned message; unknown paths → `404`.

```go
// net/http:  GET /css/app.css  compiles  ./styles/app.styl
mux.Handle("/css/", http.StripPrefix("/css/",
    stylhttp.New(stylserve.Options{Dir: "./styles", SourceMaps: true})))

// rweb: adapter ships with rweb (github.com/rohanthewiz/rweb/middleware/stylus)
s.Get("/css/*path", stylus.Handler(stylserve.Options{Dir: "./styles"}))

// Embedded sources (imports resolve inside the FS)
//go:embed styles/*.styl
// var styles embed.FS
sub, _ := fs.Sub(styles, "styles")
s.Get("/css/*path", stylus.Handler(stylserve.Options{FS: sub}))
```

`stylserve.Options`: `Dir` **or** `FS` (source root), `IncludePaths`, `Pretty`
(default compressed), `MergeDuplicates`, `SourceMaps`.

### WASM playground (`playground/`)

The compiler builds for `js/wasm`; `playground/` is a browser playground with
a live two-pane editor, the bundled examples (imports resolve against the
embedded `examples.FS`), positioned errors, and source-map output.

```shell
./playground/build.sh        # playground/styl.wasm + wasm_exec.js
go run ./playground/serve    # http://localhost:8080
```

In the page, `goStyl.compile(src, {pretty, mergeDuplicates, sourcemap})`
returns `{css, map?, ms}` or `{error, file, line, col, msg, ms}`;
`goStyl.examples()` lists the bundled examples. Deployed to GitHub Pages by
`.github/workflows/pages.yml` on push to main.

---

## 2. Two syntaxes

go-styl accepts **both** the indentation syntax and the CSS-like brace/semicolon
syntax, and auto-detects which is in use (brace mode is triggered by a real
`{ … }` block — interpolation braces alone do not trigger it). Pick one per file;
mixing the two in a single file is not supported.

```stylus
// indentation
.btn
  color: white
  &:hover
    color: #eee
```

```stylus
// braces + semicolons (equivalent)
.btn {
  color: white;
  &:hover { color: #eee; }
}
```

The `:` after a property and trailing `;` are always optional in indentation mode.

---

## 3. Variables and assignment

Variables are bare names (no `$` prefix — `$` is reserved for placeholders).

```stylus
base = 16px
font = "Helvetica Neue", Arial, sans-serif
base ?= 12px      // ?= assigns only if `base` is not already defined (no-op here)

body
  font-size base
  font-family font
```

Lexical scoping: a ruleset/mixin/function body sees its enclosing scope and may
shadow names locally.

---

## 4. Arithmetic, units, lists

Unit-aware `+ - * ** / %` and comparisons (`== != < > <= >=`), `&&`, `||`, `!`.
`**` shares the multiplicative precedence level and associates left
(`2 * 3 ** 2` is 36).

```stylus
w = 10px
x = w * 2 + 4px      // 24px   (result unit comes from the left operand)
y = (100% / 3)        // 33.333…%
z = 2 ** 10           // 1024
```

**`/` in property values is literal CSS** (`font 14px/1.5` stays `14px/1.5`,
operands still evaluate) — parenthesize to divide: `margin (h / 2) w`.
Assignments, conditions, and call arguments always divide.

Colors do channel-wise arithmetic with colors and numbers:

```stylus
color #111 + #222     // #333 (clamped at #fff)
color #333 + 10       // #3d3d3d (number applies to each channel)
```

**Whitespace-sensitive `-`/`+`** (important): a sign with a space before and none
after starts a **new list item**; a space on both sides (or none) is a binary op.

```stylus
margin 10px -5px      // a two-item list: "10px -5px"
width  10px - 5px     // subtraction:    "5px"
width  10px-5px       // subtraction:    "5px"
top    -5px           // negative value
```

Lists are space- or comma-separated:

```stylus
padding 1px 2px 3px 4px        // space list
transition color 0.2s, transform 0.3s   // comma list of space lists
```

---

## 5. Nesting and selectors

```stylus
nav
  a
    color #ddd
    &:hover        // & = parent: "nav a:hover"
      color white
    &.active       // "nav a.active"
      font-weight bold
  > .brand         // child combinator: "nav > .brand"
    font-size 20px

h1, h2, h3         // comma groups distribute over nesting
  small
    font-weight normal
```

`&` attaches directly; `:`-leading and `> + ~` combinators are handled. Selector
comma-splitting respects `()`, `[]`, and strings, so `a[href$="a,b"]` and
`:not(.x, .y)` stay intact.

---

## 6. Interpolation `{expr}`

Substitutes an evaluated value into **selectors, property names, strings, and
identifiers**.

```stylus
prefix = icon
side = left
weight = bold

.{prefix}-home          // selector  -> .icon-home
  margin-{side} 8px      // property  -> margin-left
  content "id-{prefix}"  // string    -> "id-icon"
  font Arial-{weight}    // ident     -> Arial-bold
  z-index {prefix}       // lone {expr} yields the value itself
```

A lone `{x}` yields `x`'s value; a mixed form like `a-{x}` yields a substituted
identifier. **In brace syntax, a stand-alone `{expr}` in value position is not
supported** — write the bare variable (`width x`, not `width {x}`).

---

## 7. Control flow

```stylus
if theme == dark
  background #111
else if theme == light
  background white
else
  background gray

unless theme == light    // negation of if
  border-color #333

for i in 1 2 3 4         // for val in list
  .col-{i}
    width (100% / 4) * i

for shade, idx in #eee #ccc #999   // for val, index in list
  .swatch-{idx}
    background shade

for n in 1..3            // ranges: 1..3 inclusive, 0...3 excludes the bound
  .w-{n}                 // (descending like 3..1 also works)
    width unit(n * 10, '%')
```

---

## 8. Functions and mixins

They share definition syntax; the **call site** decides behavior. A callee used in
a value position is a **function** (returns a value); used as a statement it is a
**mixin** (emits declarations/rules into the caller).

```stylus
double(n) = n * 2                 // single-line function

golden(n)                          // block function
  return n * 1.618

triple(n)                          // implicit return: the body's last
  n * 3                            // expression is the value

size(w, h = w)                     // mixin; `h` defaults to `w`
  width w
  height h

clearfix()                         // mixin emitting a nested rule
  &::after
    content ""
    clear both

stack(props...)                    // rest param collects remaining args as a list
  transition props

.box
  size(40px)                       // width:40px; height:40px
  font-size double(8px)            // 16px (function in a value)
  clearfix()                       // emits .box::after
  stack(color 0.2s, transform 0.3s)

.banner
  +size(100%, 200px)               // mixins may also be invoked with a leading +

.card
  size 40px 20px                   // transparent call: `name args` invokes a
                                   // mixin in scope (list items become args)
```

Parameters support defaults (`h = w`) and a single trailing rest param (`props...`).
Inside a mixin's body its own name is a plain property, so the classic
vendor-prefix pattern (`border-radius(n)` emitting `border-radius n`) works.

---

## 9. Built-in function library

Unknown functions pass through as literal CSS (`translateX(10px)`,
`format("woff2")`). CSS named colors (`blue`, `rebeccapurple`, …) are accepted
wherever a color is expected.

**Color**
| Signature | Notes |
| --- | --- |
| `rgb(r, g, b)` / `rgba(r, g, b, a)` | also `rgba(color, a)` to set alpha on a color |
| `hsl(h, s, l)` / `hsla(h, s, l, a)` | |
| `red(c)` `green(c)` `blue(c)` `alpha(c)` | channel getters |
| `hue(c)` `saturation(c)` `lightness(c)` | HSL components |
| `lighten(c, amt%)` `darken(c, amt%)` | |
| `saturate(c, amt%)` `desaturate(c, amt%)` | |
| `mix(c1, c2)` / `mix(c1, c2, weight%)` | weight = amount of `c1` |
| `tint(c, amt%)` `shade(c, amt%)` | mix toward white / black |
| `complement(c)` `invert(c)` | |

**Math** — `abs ceil floor round sqrt sin cos tan` take one number;
`min(a, b, …)` `max(a, b, …)`; `pow(base, exp)`; `percentage(n)` (`0.25 → 25%`).
(Note: `round` takes a single argument — no precision parameter.)

**List** — `length(list)`; `push(list, v…)` (alias `append`);
`unshift(list, v…)` (alias `prepend`); `index(list, v)`; `first(list)`
`last(list)`; `join(sep, list)`.

**String** — `unquote(s)` `quote(s)`; `s(fmt, args…)` (sprintf-style);
`uppercase(s)` `lowercase(s)`; `substr(s, start[, len])`;
`replace(find, repl, s)`; `split(delim, s)`.

**Type** — `typeof(v)` (alias `type`) → `unit|color|string|ident|boolean|list|null`;
`unit(n)` getter / `unit(n, u)` setter; `match(pattern, s)` (regex);
`light(c)` `dark(c)`.

---

## 10. `@extend` and `$placeholder`

`@extend` grafts the current rule's selectors onto every rule matching the target.
A `$placeholder` rule is **only emitted when extended**.

```stylus
$card                 // never emitted on its own
  border 1px solid #ddd
  padding 16px

.message
  padding 12px

.note
  @extend $card       // -> .note gets the $card body
.warning
  @extend .message    // -> ".message, .warning { padding: 12px }"
.error
  @extends .message   // @extends is an accepted alias
```

---

## 11. `@import`

```stylus
@import "reset.css"        // .css / url() / absolute URL -> passthrough, hoisted to top
@import "partials/theme"   // .styl -> inlined, sharing its variables and mixins
```

Inlined imports execute in the current scope (so their `=` vars and mixin defs
become available afterward). Resolution searches `BaseDir` then `IncludePaths`,
trying the path as given, `+ ".styl"`, and `…/index.styl`. Import cycles are
detected and reported.

---

## 12. At-rules

```stylus
@charset "utf-8"                 // leaf at-rules pass through verbatim

@media (min-width: bp)           // bare variables in the query ARE evaluated
  .container
    width 720px

.card                            // a nested @media bubbles the enclosing selector
  color black
  @media print
    color gray                   // -> @media print { .card { color: gray } }

@font-face                       // declaration block
  font-family "Inter"
  src url("/f.woff2") format("woff2")

@keyframes spin                  // frames emitted as-is (no & combining)
  from
    transform rotate(0deg)
  to
    transform rotate(360deg)
```

- `@media` / `@supports` wrap nested rules and **bubble** the enclosing selector
  when nested inside a rule.
- Bare variables inside the parenthesised query are evaluated
  (`@media (min-width: bp)`). **Arithmetic** in a query still needs interpolation:
  `@media (min-width: {bp * 2})`.
- `@keyframes` (and vendor-prefixed `@-webkit-keyframes`) emit frames literally.
- `@font-face` / `@page` are declaration blocks.

---

## 13. `url()`, `calc()`, `!important`

```stylus
a
  background url(/img/bg.png) no-repeat   // url(...) captured literally (slashes ok)
  width calc(100% - 20px)                 // calc(...) operators preserved
  height calc(100% - {gutter})            // {interpolation} inside calc resolves
  color red !important                    // -> color: red !important;
```

Inside `url(...)` and `calc(...)` bare variables are **not** evaluated — use
`{interpolation}` to inject a Stylus value.

---

## 14. Gotchas / limitations

- Source maps map at selector / declaration / at-rule granularity (not yet inside
  individual values).
- Variables inside `url()`/`calc()` need `{interpolation}`; arithmetic in `@media`
  queries needs `{ }`.
- Brace syntax: a stand-alone `{expr}` value isn't supported — use the bare
  variable.
- `MergeDuplicates` is non-standard extra compression (off by default).
- One syntax per file — don't mix indentation and braces in the same source.
- Compressed output strips the final `;`, leading zeros (`0.5 → .5`), and uses
  `#rgb` shorthand where possible.

---

## 15. Recipes

**Compress at build time:**
```go
css, err := styl.CompileFile("src/app.styl", styl.Options{Pretty: false})
if err != nil { log.Fatal(err) }
os.WriteFile("dist/app.css", []byte(css), 0o644)
```

**Compile a string with imports resolved against a directory:**
```go
css, err := styl.Compile(src, styl.Options{
    BaseDir:      "assets/styl",
    IncludePaths: []string{"node_modules"},
})
```

**Generate utility classes:**
```stylus
for i in 0 1 2 3 4
  .m-{i}
    margin i * 4px
```

When in doubt about a feature's exact output, write a minimal snippet to a
`.styl` file and run `go run ./cmd/styl -compress that.styl` (the CLI takes a file
path, not stdin), or compile one of the [`examples/`](../../examples/) files.
