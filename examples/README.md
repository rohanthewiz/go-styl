# go-styl examples

Each `.styl` file here is a self-contained, runnable demonstration of one feature
area. Compile any of them with the CLI:

```shell
go run ./cmd/styl examples/01-variables.styl            # pretty CSS
go run ./cmd/styl -compress examples/01-variables.styl  # minified
```

| File | Demonstrates |
| --- | --- |
| [`01-variables.styl`](01-variables.styl) | Variables, `?=` conditional assignment, unit-aware arithmetic |
| [`02-nesting.styl`](02-nesting.styl) | Nesting, `&` parent references, pseudo-classes, combinators |
| [`03-mixins-functions.styl`](03-mixins-functions.styl) | Functions vs mixins, default & rest params, `+mixin()` |
| [`04-control-flow.styl`](04-control-flow.styl) | `if`/`else if`/`else`/`unless`, `for … in` loops |
| [`05-builtins.styl`](05-builtins.styl) | Color / math / list / string / type built-ins |
| [`06-interpolation.styl`](06-interpolation.styl) | `{expr}` in selectors, properties, strings, identifiers |
| [`07-extend-placeholders.styl`](07-extend-placeholders.styl) | `@extend`, `@extends`, `$placeholder` selectors |
| [`08-at-rules.styl`](08-at-rules.styl) | `@media` (bubbling + variables), `@keyframes`, `@font-face`, `@charset` |
| [`09-brace-syntax.styl`](09-brace-syntax.styl) | The CSS-like brace/semicolon syntax |
| [`10-imports.styl`](10-imports.styl) | `@import` — inlined `.styl` partials and passthrough `.css` |

`imports/_theme.styl` is the partial pulled in by `10-imports.styl`.

To compile the import example you can run it from the repo root so the relative
path (`imports/_theme`) resolves against the file's directory:

```shell
go run ./cmd/styl examples/10-imports.styl
```
