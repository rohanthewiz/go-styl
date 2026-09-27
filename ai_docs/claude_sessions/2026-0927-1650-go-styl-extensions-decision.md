# Session: go-styl extensions kept and documented (N-007) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Closed next-list item N-007: "decide the fate of the go-styl extensions
(`{expr}` in `@media`, `calc()` and strings; single-line `f(x) = expr`):
gate them or keep them." Decided on my recommendation, under the user's
instruction to use my best judgment: **keep them, ungated**, and narrow the
one extension that could change a valid Stylus sheet.

Same session as the items from `2026-0927-1625` to `-1648`.

## Reasoning

- Each extension is syntax Stylus rejects (a parse error: `{x}` in
  `@media`/`calc()`/`url()`, a lone `{x}` value) or leaves literal
  (`f(x) = expr` → `double(15px)` stays in the output). Accepting them
  can't break a sheet that compiles in Stylus.
- Several recommended workarounds depend on them: `calc(100% - {gutter})`
  (README Limitations), `url({dir}/x.png)`, and the N-028 alternative.
- A gate (option or strict mode) would add surface for no current user.
  The difftest already pins each extension in `known_diffs.txt`, so any
  change is visible.

The exception was `{expr}` inside quoted strings. Stylus keeps string text
literal, and go-styl substituted every group, even `"{nope}"` → `"nope"`
(an undefined name interpolates as itself), with no way to escape
(`"\{name}"` → `"\x"`). A string like a JSON snippet or a template
placeholder changed silently.

## Changes

- `internal/eval/m4.go`: new `interpolateString`, used for
  `*ast.StringLit` (selectors, property names etc. still use
  `interpolate`):
  - a backslash escape is copied with its next rune, so `\{` never opens a
    group (and prints `\{` like Stylus);
  - a group is substituted only if it parses as an expression and
    `refsVariable` (added for N-001) finds a defined variable;
  - otherwise the group is copied verbatim.
- `README.md`:
  - new "Extensions" subsection (before "Deliberate differences") listing
    `{expr}` in `@media`/`calc()`/`url()`/lone values, `{expr}` in strings
    with the new rule, single-line functions, and Globals/CustomProperties
    plus MergeDuplicates;
  - the Status list now covers `@require` + globs, list indexing, string
    `+` and `%`, and `url()` variables.

## Tests

- `interp_string_test.go`: `TestStringInterpolationScope`. A variable and
  an expression with a variable are substituted; an undefined name, a
  constant expression, an escaped brace, unparsable contents and a lone
  `{` stay literal. The literal cases match stylus 0.64 byte for byte.
- Existing `m4_test.go` string case (`"x-{p}"` with `p` defined) is
  unchanged. `go test ./...` passes.

## Next

Closed: N-007. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
