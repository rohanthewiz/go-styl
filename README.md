# go-styl

A pure-Go compiler for the [Stylus](https://stylus-lang.com/) (`.styl`) CSS preprocessor — no Node.js, no cgo. The only dependency is [serr](https://github.com/rohanthewiz/serr), a zero-dependency structured-error wrapper.

> Forked from [aerogo/scarlet](https://github.com/aerogo/scarlet) and rebuilt around a real
> lexer → AST → evaluator pipeline so it can target the full Stylus language rather than a
> Stylus-inspired subset.

## Status

Under active development. The compiler currently supports:

- Both the **indentation syntax** and the CSS-like **brace/semicolon syntax**
- Indentation-based nesting with `&` parent references and pseudo-class attachment
- Compile-time **variables** (inlined, with lexical scoping) and `?=` conditional assignment
- Unit-aware **arithmetic** (`+ - * / %`) and comparisons
- Comma and space **value lists**
- **Control flow**: `if` / `else if` / `else` / `unless`, and `for … in` loops
- User-defined **functions** (return values) and **mixins** (emit declarations/rules),
  with default and rest (`args…`) parameters, in single-line and block forms
- **Built-in functions** across color (`rgb`/`rgba`/`hsl`/`hsla`/`lighten`/`darken`/
  `saturate`/`mix`/`tint`/`shade`/`complement`/`invert`/`hue`/`alpha`/…), math
  (`abs`/`ceil`/`floor`/`round`/`min`/`max`/`pow`/`percentage`/…), list
  (`length`/`push`/`index`/`last`/`join`/…), string (`unquote`/`quote`/`s`/`substr`/
  `replace`/`split`/`uppercase`/…), and type (`typeof`/`unit`/`match`/`light`/`dark`),
  with CSS named-color support
- Unknown functions pass through as literal CSS (`translateX(10px)`, `url(...)`)
- **Interpolation** (`{expr}`) in selectors, property names, strings, and identifiers
- **`@extend`** (and `@extends`) plus **`$placeholder`** selectors
- **`@import`**: `.styl` files are inlined (sharing variables/mixins); `.css` and
  `url(...)` imports pass through verbatim
- **At-rules**: `@media` / `@supports` (with selector bubbling and variables in
  queries), `@keyframes`, `@font-face`, and verbatim passthrough for leaf at-rules
  (`@charset`, …)
- Literal **`url()`** and **`calc()`** (operators/paths preserved; `{interp}` still
  resolves), **`!important`**, and whitespace-sensitive unary `-`/`+`
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

See [the roadmap](#roadmap) for what's next.

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
(default compressed), `MergeDuplicates`, `SourceMaps`. Compile errors return
`500` with the positioned message; unknown paths return `404`.

## CLI

```shell
go run ./cmd/styl input.styl            # pretty CSS to stdout
go run ./cmd/styl -compress input.styl  # minified
go run ./cmd/styl -merge input.styl     # merge duplicate rule bodies
go run ./cmd/styl -o out.css input.styl # write to a file
go run ./cmd/styl -o out.css -sourcemap input.styl  # also writes out.css.map
```

`-sourcemap` requires `-o`; it writes `<out>.map` next to the CSS and appends a
`/*# sourceMappingURL=… */` comment.

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

## Limitations

Things to be aware of:

- Source maps map at selector / declaration / at-rule granularity (column-accurate
  for those, including compressed output); they do not yet map inside values.
- Inside `url(...)` and `calc(...)`, bare Stylus variables are *not* evaluated —
  use interpolation: `calc(100% - {gutter})`. (`@media` query values *are*
  evaluated: `@media (min-width: bp)`.)
- Arithmetic in a `@media` query needs interpolation: `@media (min-width: {bp * 2})`.
- In brace syntax, a stand-alone `{expr}` in value position is not supported —
  use the bare variable (`width x`, not `width {x}`).
- The `MergeDuplicates` pass is a non-standard extra-compression option (off by
  default); standard Stylus does not fold identical rule bodies.
- Function/mixin call depth is capped at 256 and a rule's combined selector
  count at 16384, so unbounded recursion errors out instead of hanging.
- In brace syntax, statements that share a source line with an earlier one
  (one-liner blocks) report approximate positions; multi-line files are exact.

## Architecture

```
.styl  →  parser (brace→indent normalize, indentation line-tree + Pratt expr parser)
       →  ast
       →  eval (lexical scope, variable inlining, arithmetic, builtins, at-rules)
       →  css (node tree → position-tracking render, optional merge + source map)
       →  CSS (+ optional .map)
```

Packages live under `internal/`: `token`, `lexer`, `ast`, `parser`, `value`, `eval`,
`builtin`, `css`.

## Roadmap

- [x] **M1** Vertical slice: lexer, parser, scoped evaluator, arithmetic, nesting, one builtin
- [x] **M2** Control flow (`if`/`else`/`for`) and parametric functions & mixins
- [x] **M3** Built-in function library (color / math / list / string / type)
- [x] **M4** Interpolation (`{expr}`), `@extend` / `$placeholder` selectors, `@import`
- [x] **M5** At-rules (`@media` / `@keyframes` / `@font-face` / …), brace syntax, compress parity
- [x] **M6a** Correctness: `url()`/`calc()`, `!important`, media-query variables,
  bracket-aware selector splitting, whitespace-sensitive `-`/`+`
- [x] **M6b** Source maps (Source Map v3, `CompileMap` / `-sourcemap`)
- [ ] Future: value-level source mapping, deeper compress parity, more built-ins

## License

See [LICENSE](LICENSE).
