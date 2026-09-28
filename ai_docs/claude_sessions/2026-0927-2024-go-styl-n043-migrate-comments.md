# Session: `styl migrate` keeps source comments (N-043) (2026-09-27)

Session ID: `584be964-081a-458d-87c6-1306959d9b55`

Implemented next-list item N-043. `styl migrate` used to drop every source
comment, because `parser.stripComments` removed them before the parser ran.
Comments are now carried into the migrated CSS:

```styl
// Card styles                 /* Card styles */

.card                          .card {
  color red // why red    →      /* why red */
  // closing                     color: red;
                                 /* closing */
                               }
```

## Parser (`internal/parser`)

- **Opt-in path.** `ParseWithComments(src)` is `Parse` plus comments. `Parse`
  is unchanged (it calls `parse(src, false)`), so compile, `styl fmt` and
  the LSP get the same AST as before. With no comments, the compile path
  does no extra work.
- **`scanComments`** (`comments.go`) finds every comment that
  `stripComments` removes, using the same rules: a `//` only after
  whitespace, strings skipped, an unterminated `/*` running to EOF. It
  records the text, block vs line comment, line, end line, column,
  indentation, and whether a blank line follows (`gap`).
- **`commentAttacher`.** Comments are never lines in the parse tree. They
  attach to real `line` nodes as `lead` (before) or `trail` (after), so a
  comment can't turn a leaf into a block or split an `if`/`else` chain.
  - An inline comment (code on its start line, or after its `*/`) leads
    that code line.
  - An own-line comment leads the next code line, unless it is indented
    deeper than that line. Then it closes the block it sits in and trails
    the block's last child (`trailInto`).
  - A comment on a line folded into a trailing-comma continuation goes to
    the continued statement. Whatever is left at EOF trails the deepest
    open block, or the root.
  - Comment positions come from the original source. `joinObjectLiterals`
    and `bracesToIndent` keep lines in place, so the positions still match
    the line tree, except for statements of one-line brace blocks (they may
    land on a neighboring statement, never get lost).
- **Line rewrites carry the comments along.**
  - `expandSemicolons`: the first piece of a split line gets `lead`, the
    last gets `trail`. A dropped `;`-only line hands its comments to the
    previous statement, else the next one, else to a text-less placeholder
    line that `parseBlock` turns into comments only. It copies lines rather
    than mutating them.
  - `groupSelectorLines`: the group's comments go above the merged rule.
- **`parseBlock`** emits `commentStmts(lead)`, then the statement, then
  `trail`. For an if chain, the first line's lead goes before the `If`, and
  the comments of the `else` lines follow it.

## AST

`ast.Comment{Text, Block, Line, Col, EndLine, Inline, BlankAfter}`, added to
`ast.Pos`. The evaluator treats it as a no-op (`execStmtInner`), which
matters for comments in function bodies.

## Migrator (`internal/eval/migrate.go`)

- **New node kind `mComment`**, with `src` and `file` for adjacency checks.
  A `moved` comment isn't rendered in place.
- **`comment`** writes a comment in place. Output comes from `cssComment`:
  `//` becomes `/* … */` through `commentSafe`, and a block comment keeps
  its text, with continuation lines stripped of the start column's
  indentation. It writes nothing when `NoComments` is set, or while `muted`
  is on (a loop's second and later iterations).
- **`docRun`** finds the run of comment nodes just before a statement in
  the same file: first inline comments on the statement's own line, then
  own-line comments on consecutive lines. A blank line or another
  statement's inline comment ends the run.
  - **`adoptDocs`:** if a root assignment created a custom property, those
    comments are recorded in `varDocs`. `rootRule` copies them into
    `:root` and marks the originals moved.
  - **`dropDocs`:** a `FuncDef`'s run is removed and noted (kind
    `comment`), since the definition is expanded away.
- **`:root` placement.** `firstVarKid` records the root child count when
  the first custom property is created. Leading root comments before that
  index (a file header, including one before an `@require` that defines the
  variables) stay above `:root`.
- **`renderKids`.** A comment is spaced like the node it precedes
  (`nextIsBlock`), with no blank line between them. It keeps a blank line
  after it when the source had one (`BlankAfter`). `mRaw` output is
  byte-identical to before.
- **Mixin and loop bodies.** A comment in a mixin body is written at each
  expansion; a loop body's comment only once.
- **Imports.** Inlined `.styl` imports are parsed with comments too.

## API and CLI

`MigrateOptions.NoComments` (in both `styl` and `eval`), plus
`styl migrate -no-comments`. The docs in `Migrate` and `cmd/styl/main.go`
were updated.

## Tests

- `internal/parser/comments_test.go`:
  - `checkCommentParse`: `ParseWithComments` fails exactly when `Parse`
    does; with comments removed (a reflect walk over `[]ast.Stmt`) the
    tree is `sameAST` to Parse's; and the comment count equals
    `len(scanComments)`.
  - Run over the whole corpus and hand-written edge cases, plus
    `TestCommentPlacement`.
  - `FuzzParseWithComments`: its first run found the lone `; /*` loss. That
    input is kept as a seed in `testdata/fuzz/`; after the fix, about 11M
    executions ran clean.
- `migrate_test.go`:
  - Golden output for in-place and `:root` doc placement, brace syntax,
    mixin and loop behavior, and `NoComments`.
  - `parseCSS` (used by the round-trip test) now skips multi-line comments.
    `TestMigrateRoundTrip` still passes with comments in the output.
- Full suite, `go vet`, and 60s of `FuzzCompile` (which also runs
  `Migrate`) all pass.

## Known limits

- An imported partial's header directly above its first variable is taken
  as that variable's doc (seen in `difftest/corpus/cema-shaped.styl`).
- Comments in a branch that isn't taken are dropped along with the branch.
- Consecutive `//` lines become separate `/* */` lines; they aren't merged.

## Next

Closed: N-043. Declined: None. Raised: None. Deferred: None.
Promoted: None. Updated: None. Full list: `ai_docs/todo/next-list.md`.
