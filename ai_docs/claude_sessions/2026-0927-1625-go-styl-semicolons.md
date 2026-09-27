# Session: semicolons in the indentation syntax (N-027) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Implemented next-list item N-027. In the indentation syntax, a trailing `;`
(`margin: 0 auto;`) or several declarations on one line
(`color:black; background: #72962d`) failed with `unexpected ";" in
expression`. cema has 337 such lines across 15 files, so this blocked N-022
(compile cema with go-styl).

## Stylus behavior (probed against stylus 0.64)

- A top-level `;` separates statements in indented source too. Trailing and
  doubled `;` are dropped. Declarations, assignments (`x = 5px;`) and mixin
  calls (`m();`) all accept it.
- A `;` inside a string (`content "a;b"`) is text.
- Split pieces are siblings. If the line has an indented body, the body goes
  to the last piece: `.a` / `color red; .b` / (indented) `top 0` gives
  `.a{color:red}` and `.a .b{top:0}`.
- A multi-line value joined by a trailing comma (`transition a 1s,` /
  `b 2s;`) ends at the `;` as expected.
- Stylus **fails** on an unquoted data URI with a `;`
  (`url(data:image/png;base64,…)`: `expected ")", got ";"`) and on
  `@extend .c;` (the `;` becomes part of the selector). go-styl accepts both,
  like the earlier lenient choices (N-026's escaped quotes).

## Implementation

- `internal/parser/semis.go` (new):
  - `splitSemicolons(text)` scans for `;` outside strings (with backslash
    escapes) and outside `()`, `[]`, `{}` nesting. It returns nil when there
    is no top-level `;`, so the common case costs one scan and no
    allocation. Each piece carries its rune offset in the line.
  - `expandSemicolons(lines)` replaces each such line with one `line` per
    non-empty piece, all at the same level and source line. The last piece
    keeps the original children. A piece's `indent` is the original indent
    plus its offset, which makes the error column (`indent+1`) point at
    that statement. `indent` is no longer used for structure once the tree
    is built, so this is safe. A line of only semicolons is dropped if
    childless, kept whole otherwise, so the parser reports it instead of
    silently losing a body.
- `internal/parser/parser.go`: `parseBlock` calls `expandSemicolons` first,
  before `groupSelectorLines`, so selector runs and if/else chains see each
  statement as its own line.

Why the tree and not the raw text: by the time `parseBlock` runs, comma
continuations are already joined, and splitting whole lines can't disturb
the indentation structure. Brace-syntax sources never reach this with a
top-level `;`, because `bracesToIndent` already splits on it.

## Tests

- `semicolon_test.go`: 15 cases (trailing, multiple, doubled, lone `;`,
  strings, escaped quote, data URI, `!important`, `//` comment, assignment,
  mixin call, comma continuation, body to last piece, if/else) plus
  `TestSemicolonErrorColumn` (`color red; width 1px +` reports `2:14`).
  Expected values match stylus 0.64 except the data URI case.
- `go test ./...` passes; difftest unchanged. `FuzzCompile` ran 20s clean.

## cema check

Compiled every cema `.styl` file containing `;` (28 files) with the new
binary. No semicolon errors remain. What's left is known: N-030
(`payment_form` undefined mixin) and the cema-side column-0 `.form-help {` in
`_material_form.styl` (ends the `mat-form()` scope, so `mf-spacer` is
undefined after line 138).

## Found along the way

- In brace syntax, `scanStructural` treats a `;` inside parentheses as a
  statement end, so `.a { background: url(data:image/png;base64,AAA=) }`
  fails with `unexpected ","`. Stylus fails there too. Raised as N-037.

## Next

Closed: N-027. Declined: None. Raised: N-037.
Deferred: None. Promoted: None.
Updated: N-022. Full list: `ai_docs/todo/next-list.md`.
