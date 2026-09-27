# Session: multi-line selector groups (N-024) (2026-09-27)

Session ID: `6e41c92e-5a0b-4cdf-b02a-b5b9e0524fcc`

Implemented next-list item N-024: selector groups that span lines. This was
one of the blockers for N-022 (compile cema with go-styl). It covers two
forms:

- **(a)** a trailing-comma continuation: `.a,` with `.b` on the next line.
- **(b)** stacked selector lines that share the block of the last one:
  `&:after` over `&:before`, or `td:nth-child(1)` over `td:nth-child(2)`. The
  second pair used to be misread as the declaration `td: nth-child(1)`.

## Stylus behavior (probed against stylus 0.64)

The rules come from `Parser#looksLikeSelector` in stylus's
`lib/parser.js`:

- **Selector lines absorb what follows.** Once a line looks like a selector,
  Stylus absorbs the lines after it, verbatim, until one has an indented
  body. The absorbed lines include declarations: `.a` / `color red` / `.b`
  gives the selector `.x color red`. When no block follows, it's a
  ParseError.
- **Trailing comma.** A trailing comma continues the selector onto the next
  line, whatever that line's indentation. Blank lines and comments in
  between are skipped. The same goes for values:
  `transition opacity 1s,` / `transform 2s` is one declaration.
- **Pseudo-class list.** A glued `ident:name` is taken as a selector only
  when `name` is in Stylus's `pseudoSelectors` list. `display:block` and
  `cursor:default` stay declarations. A `::` always makes a selector.
- **Bare identifiers.** A bare identifier directly above a selector block is
  a type selector, even when a mixin of that name exists:
  `m` over `.y` → `.x m, .x .y`. A bare `m` followed by a declaration does
  nothing in Stylus, because a mixin call needs parens. In go-styl, calling
  a mixin without parens is an extension.

## What changed

- **Trailing-comma join** (`internal/parser/parser.go`, `buildTree`): a line
  that ends in `,` takes the next non-blank line's content, whatever its
  indentation. That line doesn't enter the tree on its own. The join happens
  after `bracesToIndent`, so brace syntax is fixed too. Before this change,
  `.a,\n.b {` failed with `unexpected character "."`.
- **Stacked selector lines** (new file `internal/parser/selgroup.go`):
  - `groupSelectorLines` runs at the top of `parseBlock`.
  - It merges a run of selector-shaped leaf lines (lines with no children)
    with the block line that ends the run. The merged text is joined with
    `", "`. The merged line keeps the first leaf's line number and indent,
    and takes the block line's children.
  - `looksLikeSelectorLine` checks one line at a time:
    - a leading `& > ~ [ :`
    - a leading `.` or `#` that is not followed by a digit
    - a leading `*` or `+` that is not followed by a letter. This rules out
      `+mixin` calls and the `*zoom` hack.
    - a bare identifier, other than a keyword
    - an identifier glued to `.`, `#`, `[`, `::`, or `:` followed by a name
      in the pseudo-class list
  - The pseudo-class list is copied from Stylus.
  - `isRuleSetHeader` makes sure the block line would parse as a ruleset:
    not an `@` rule, not control flow, and not a `name(params)` definition.
  - Deliberate divergence: a run that contains a declaration-shaped line is
    not merged. Stylus would glue it into the selector and output nonsense.

## Tests

- `multiline_selector_test.go` has 23 cases:
  - trailing commas: top level, nested, a continuation indented deeper, a
    blank line or comment inside the group, brace syntax, and a multi-line
    value list
  - stacks: pseudo-classes after `&`, nth-child, bare type selectors,
    combinators, attribute selectors, universal, brace syntax, and a bare
    identifier over a block
  - negatives: a declaration or colon declaration followed by a block,
    `cursor:default`, `m()` followed by a block, and a bare `m` followed by a
    declaration
- Each case was checked against stylus, except the bare `m` followed by a
  declaration, which is a go-styl extension and says so in the test.
  Stylus's compressed output keeps spaces around combinators and go-styl's
  doesn't. The tests expect go-styl's form.
- `go test ./...` passes. Difftest is unchanged at 23/32.

## Notes

- Differences that remain (these weren't regressions):
  - `.x` over `:hover` joins as `.x:hover`, while Stylus gives `.x :hover`.
    Raised as N-035.
  - `%ph` placeholders and the `*zoom 1` hack were already errors before
    this change, and still are.
- `m(1),` over `m(2)` now joins into the declaration `m: 1, m(2)`. Stylus
  errors on it. Left as an edge case.
- The untracked `.cats-todo/` folder isn't from this work and is left out of
  commits.

## Next

Closed: N-024. Declined: None. Raised: N-035.
Deferred: None. Promoted: None.
Updated: N-022 (blockers now N-025…N-030). Full list: `ai_docs/todo/next-list.md`.
