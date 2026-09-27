# Session: cema parity verified (N-022) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Closed next-list item N-022 on the go-styl side. With the go-styl blockers
N-027…N-030 closed earlier in this session, I re-ran the cema comparison.
cema's stylesheets now compile with go-styl to the same CSS as stylus 0.64,
once cema's own source quirks are patched in a scratch copy. One more
go-styl fix came out of it: the selector order inside nested groups.

Same session as N-027 (`2026-0927-1625-go-styl-semicolons`), N-028
(`…-1628-go-styl-sprintf-operator`), N-029 (`…-1630-go-styl-number-formatting`)
and N-030 (`…-1631-go-styl-undefined-mixin-doc`).

## Method

`ai_docs/tools/cema-parity/` (new, committed so the next run needn't
rebuild it):

- `run.sh <workdir>` builds the CLI, compiles `master.styl` and the six
  `theme_masters/*.styl` from `<workdir>/styl` with both compilers, and
  checks each pair two ways:
  - **`cmp.py`, semantic:** each sheet becomes
    `{(at-rule context, selector): {prop: value}}`, with selector groups
    expanded. Whitespace, hex case/shorthand, named colors (table read from
    `internal/value/names.go`), leading zeros, quote style and `url()`
    quotes are normalized.
  - **Order:** the sequence of rule headers, selector lists included.

`@require` and globs work natively now (N-023), so the old `flatten.py`
step is gone.

## cema-side quirks (patched in the scratch copy only)

cema was not modified. Each of these is a source problem that Stylus's
output hides:

1. **`_material_form.styl:138`:** `.form-help {` at column 0 inside the
   `mat-form()` mixin. Fix: indent it to 4 spaces (its `}` is at 4).
2. **`_material_form.styl:4`:** `// @import url(...)` at column 0 between
   `mat-form()` and its body. In Stylus, that comment **ends the mixin**:
   the whole body is emitted once at the root, and the `mat-form()` call
   in `#main` (`_mid_layout.styl:87`) is silently dropped. So cema
   currently ships `.wrapper-material-form …`, not
   `body #main .wrapper-material-form …`. Indenting the comment makes both
   compilers nest it, which **changes cema's CSS** (higher specificity,
   scoped to `#main`). The earlier probe blamed the root emission on quirk 1.
3. **`_banner.styl:25–40`:** commented-out lines indented deeper (6 spaces)
   than the rule around them (4). Stylus lets comment indentation shape
   the tree, so `#banner-wrapper` and `#banner-extension` come out as
   `body #…` instead of `body #banner #…`. Fix: align the comments to 4.
   This also **changes cema's CSS** (a descendant of `#banner` either way,
   so it likely renders the same). The earlier probe blamed the `/* */`
   fragments on those lines; stripping them alone changes nothing.
4. **Dead calls:** `sermon()` and `fullcalendar()` in `base.styl`
   (undefined mixins; go-styl errors, see N-030).
5. **`_styl/payment_form.styl:1`:** a bodiless `payment_form()` call over a
   column-0 `#payment-form` block. Deleting the line keeps Stylus's output.

Minimal repros of Stylus's comment sensitivity:

    m()                  body              m()
    // c                   #a                #a
      .w                     top 0             top 0
        color red              //y               //y
    body                                     ...
      #main              → `top 0` is dropped entirely
        m()
    → `.w{color:red}` at the root; call dropped

## go-styl fix: selector order in nested groups

The one real diff left after the patches: `.a, .b` over `.x, .y` gave
`.a .x,.a .y,.b .x,.b .y` (parent-major). Stylus gives
`.a .x,.b .x,.a .y,.b .y` (child-major): `utils.compileSelectors` recurses
from the innermost level outward. `combineSelectors`
(`internal/eval/selector.go`) now loops over the child selectors first. The
parent list already carries that order from the levels above, so it matches
at any depth. Checked at three levels and with trailing `&`. This doesn't
change what matches; it's for byte parity.

Test: `selector_order_test.go` (`TestNestedGroupOrder`, 4 cases, checked
against stylus 0.64). No existing test pinned the old order.

## Result

All 7 sheets: 290 selectors each side, 230 rule blocks in identical order
with identical selector lists. There are 2 semantic diffs per sheet, both
`opacity: .00000001` (go-styl) vs `1e-8` (Stylus, JS exponent notation).
That's the same number, and the decimal form is the more portable CSS.

README "Deliberate differences" gained two bullets: comment lines never
affect structure, and small numbers print in decimal form.

`go test ./...` passes; difftest 23/33, unchanged.

## For cema (handed to the user, not done here)

To drop the `stylus` npm dependency on `roh/use-go-styl`:

- Apply quirk fixes 1, 4 and 5. They don't change the CSS.
- Decide on 2 and 3. They do change the CSS (material form scoped under
  `#main`; banner children under `#banner`). To keep today's output
  instead, move the material-form rules out of the mixin and dedent the
  banner children.
- Switch the build to `styl` (or go-styl as a library) and diff
  `dist/css` once.

## Next

Closed: N-022. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
