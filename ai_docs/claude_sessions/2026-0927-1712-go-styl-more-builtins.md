# Session: more Stylus built-ins (N-004) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Closed next-list item N-004 ("more built-ins and deeper compress parity").
The item was open-ended, so it was made concrete: list stylus 0.64's
built-ins (`lib/functions/*.js` plus the functions defined in
`index.styl`), diff them against go-styl's registry, and implement every
one that needs neither a hash type nor evaluator context. The rest went to
N-040.

Same session as the items from `2026-0927-1625` to `-1706`.

## Added (each checked against stylus 0.64)

- **math** (`internal/builtin/math.go`): `asin`, `acos`, `atan` (angle in
  deg by default, or rad/turn/grad; rounded to 9 places),
  `radians-to-degrees`, `degrees-to-radians`, `sum` (via `value.Arith`,
  so the first unit wins), `avg`, `odd`, `even`, `remove-unit`,
  `percent-to-decimal`, `base-convert(num, base, [width=2])`.
- **color** (`color.go`): `fade-in`/`fade-out` (alpha ± fraction,
  clamped), `grayscale`, `luminosity` (WCAG relative luminance),
  `blend(top, [bottom=white])`, `transparentify(top, [bottom], [alpha])`
  (ported from the JS, with 0/0 treated as 0), `component(color, name)`.
- **list** (`list.go`): `pop`, `shift` (return the item, no mutation),
  `range(start, stop, [step])` (capped at 100000 items; step must be
  positive), `list-separator` (quoted `' '`/`','`), `keys`/`values` over
  a list of pairs, `clone` (identity; values are immutable).
- **string/path** (`string.go`): `basename(p, [ext])`, `dirname`,
  `extname`, `pathjoin`, which return `'…'` strings like Stylus, via
  `path` (slash paths); `convert(str)` for single values (number, hex
  color, otherwise an ident).
- **type** (`type.go`): `opposite-position`, `error(msg)`.

## Parity fixes found by the side-by-side run

- `sin`/`cos`/`tan` kept the argument's unit and ignored `deg`. They now
  convert `deg` to radians, return unitless values and round to 9 places
  (`sin(90deg)` is 1, `sin(180deg)` is 0).
- `filter: saturate(2)` errored ("expects 2 arguments"). `saturate()`,
  `invert()` and the new `grayscale()` render as a literal CSS call
  (`literalCall`) when the first argument isn't a color, like Stylus's
  `unquote("saturate(" + color + ")")`.
- **A built-in called as a statement** was "undefined mixin". So a function
  ending in `round(n)` failed (Stylus returns 1), and `error('…')` guards
  couldn't work. `evalMixinCall` now evaluates a builtin statement like an
  expression: its value is the implicit return, and its error propagates.
  In a rule body the value is dropped, as in Stylus.

## Tests

- `builtins_stylus_test.go`: `TestStylusBuiltins`, 45 cases whose
  expectations were generated from `stylus -c` output. Six rgba cases use
  go-styl's `.8` alpha where Stylus compresses to `0.8`; that's an existing
  formatting difference, erased by the difftest normalizer.
  `TestBuiltinAsStatement` and `TestErrorBuiltin` cover the statement fix.
- A 36-property sheet exercising all of the above diffed **byte for byte**
  against `stylus -p`.
- `go test ./...` passes; difftest 24/35 unchanged; `FuzzCompile` ran 15s
  clean.

README Status: the built-ins bullet now lists the new functions and the
filter pass-through.

## Not done → N-040

Hash-based (`merge`, object `keys`/`values`, `contrast`, `json`) and
evaluator-context built-ins (`selector()`, `selectors()`,
`selector-exists()`, `current-media()`, `define()`, `lookup()`, `use()`,
`prefix-classes`, `add-property`, `warn`), plus list mutation by
`push`/`pop`/`shift`.

"Deeper compress parity": nothing concrete was found beyond the rgba
leading zero, and both forms are valid CSS.

## Next

Closed: N-004. Declined: None. Raised: N-040.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
