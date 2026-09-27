# Session: variables inside url() (N-001) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Implemented next-list item N-001: with `p = "a.png"`, `url(p)` printed
`url(p)`. Stylus prints `url("a.png")`.

Same session as N-027…N-030, N-022, N-035…N-037 and N-008
(`2026-0927-1625` … `-1643`).

## Stylus behavior (stylus 0.64 source and probes)

- `Parser#url` parses `url(` arguments as ordinary expressions, with the
  lexer adding `urlchars` literals (`/:@.;?&=*!,<>#%0-9`) for bare paths.
- The evaluator treats it as an undefined function call, so the arguments
  are evaluated. `ignoreColors` stops `url(red)` becoming a color.
- The compiler (`visitCall` with `isURL`) prints strings without quotes and
  expression nodes without spaces, then wraps everything in `"…"`. So:
  - `url(p)` → `url("a.png")`, `url(base + "x.png")` → `url("/img/x.png")`
  - `url(p p)` → `url("a.pnga.png")`, `url(nothere)` → `url("nothere")`
  - `url(/x/y.png)` → `url("/x/y.png")`
- Unquoted data URIs with `;` fail to parse in Stylus.

## Design choice

Stylus quotes **every** url(). go-styl has always kept url() contents
verbatim (the lexer's `rawCallNames` captures `url(...)` as one token), and
the difftest normalizer already erases the quoting difference. Quoting
everything would churn every existing go-styl output for no semantic gain.
So the evaluation is targeted:

- evaluate only when the contents parse as an expression **and** reference
  a variable defined in scope;
- otherwise, or when evaluation fails, keep the verbatim text.

That keeps `url(/a.png)`, `url(data:…)`, `url(http://…)` and
`url(p/b.png)` (which doesn't parse, so it isn't mistaken for
`p / b.png`) exactly as before.

## Implementation

`internal/eval/eval.go`:

- `evalURL(name, scope)` is called from the `*ast.Ident` case after
  interpolation (a `{…}` url is still handled by the interpolation path)
  and before the variable lookup. It checks the `url(`…`)` shape
  (case-insensitive), runs `parser.ParseExpr` on the inner text, checks
  `refsVariable`, evaluates, and returns an Ident `url("` + `urlText(v)` +
  `")`.
- `refsVariable` walks Ident/Unary/Binary/List/Call/Index.
- `urlText`: a `Str` gives its raw value, a `List` runs its items together,
  anything else its CSS form (custom-property wrappers deref'd).

README Limitations: the url()/calc() bullet now says calc() leaves
variables literal (as Stylus does) and url() evaluates them.

## Tests

`url_vars_test.go`: 12 cases (bare variable, concatenation, two variables,
list, undefined name, path, quoted, absolute URL, data URI,
`p/b.png`, interpolation, inside a mixin). `go test ./...` passes.

## Found along the way

Interpolating a **quoted string** keeps its quotes: `s = "x"` makes
`.a-{s}` into `.a-"x"` (Stylus gives `.a-x`), and `url({base}x.png)` into
`url("/img/"x.png)`. This predates this session. Raised as **N-038**
(value medium: it's a real Stylus-compat bug in selectors).

## Next

Closed: N-001. Declined: None. Raised: N-038.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
