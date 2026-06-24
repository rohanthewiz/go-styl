# go-styl

A pure-Go compiler for the [Stylus](https://stylus-lang.com/) (`.styl`) CSS preprocessor — no Node.js, no cgo, zero external dependencies.

> Forked from [aerogo/scarlet](https://github.com/aerogo/scarlet) and rebuilt around a real
> lexer → AST → evaluator pipeline so it can target the full Stylus language rather than a
> Stylus-inspired subset.

## Status

Under active development. The compiler currently supports:

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
- Pretty and **compressed** output, plus an optional duplicate-rule **merge** pass

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
```

`Options`:

| Field | Meaning |
| --- | --- |
| `Pretty` | Expanded, human-readable output (otherwise compressed). |
| `MergeDuplicates` | Fold rules with identical bodies into one selector group. |
| `IncludePaths` | Extra directories searched for `@import`. |
| `BaseDir` | Directory relative `@import` paths resolve against (defaults to `Filename`'s dir). |
| `Filename` | Source path, used in errors and to derive `BaseDir`. |

## CLI

```shell
go run ./cmd/styl input.styl            # pretty CSS to stdout
go run ./cmd/styl -compress input.styl  # minified
go run ./cmd/styl -merge input.styl     # merge duplicate rule bodies
go run ./cmd/styl -o out.css input.styl # write to a file
```

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

## Architecture

```
.styl  →  parser (indentation line-tree + Pratt expression parser)
       →  ast
       →  eval (lexical scope, variable inlining, arithmetic, builtins)
       →  css (rule tree → render, optional duplicate merge)
       →  CSS
```

Packages live under `internal/`: `token`, `lexer`, `ast`, `parser`, `value`, `eval`,
`builtin`, `css`.

## Roadmap

- [x] **M1** Vertical slice: lexer, parser, scoped evaluator, arithmetic, nesting, one builtin
- [x] **M2** Control flow (`if`/`else`/`for`) and parametric functions & mixins
- [x] **M3** Built-in function library (color / math / list / string / type)
- [x] **M4** Interpolation (`{expr}`), `@extend` / `$placeholder` selectors, `@import`
- [ ] **M5** At-rules (`@media`, `@keyframes`, …), compress parity, brace syntax, sourcemaps

## License

See [LICENSE](LICENSE).
