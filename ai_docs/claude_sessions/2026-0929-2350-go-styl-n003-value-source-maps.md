# Session: value-level source mapping (N-003) (2026-09-29)

Session ID: `95d7311a-f8c4-4175-bf13-5af3cfa51f52`

Closed N-003, which had been on the Roadmap since M6. Source maps used to
have one segment per selector, declaration and at-rule. Each declaration
value now gets a segment of its own. Getting that right meant fixing where
positions come from, since a value column is only useful if the columns
around it are correct.

## What a map looks like now

```
c = red
body
	color c
```
```
gen 1:1  "color: red;"  <- src 2:1  "color c"
gen 1:8  "red;"         <- src 2:7  "c"
```

- The value maps to the value expression's first character: the use site.
  A variable is not followed back to its definition, which is how Sass and
  Stylus map values too.
- `!important` and the `;` share the value segment, which runs until the
  next segment starts.
- A declaration inside a mixin body maps into the mixin body, the same as
  its property.
- Statements that have no source expression (`add-property()`, the
  `CustomProperties` `:root` rule) have a zero `ValuePos`, so they get no
  value segment and the property segment covers the whole declaration.

Plumbing: `ast.Declaration.ValueLine/ValueCol` (set by the parser from the
first value token) → `css.Statement.ValuePos` (in `eval.go`) →
`p.mark(st.ValuePos)` in `Rule.Render` just before the value is written.
Expression nodes still carry no positions. The value's start is the only
expression position the parser records.

## True source positions (`internal/parser`)

Before this session a statement's `Col` was `ln.indent + 1`. That is wrong
in three cases, and a value column built on top of it inherits the error:

1. **Tabs.** `indent` is a width: a tab advances to the next multiple of
   `tabWidth` (4). So `\t\twidth` reported column 9 when the real column is 3.
2. **Brace syntax.** `bracesToIndent` re-indents each statement to
   `2 * depth`, so the column was the rewrite's column, not the source's.
3. **One-line blocks.** In `.a { width: 1px; height: 2px }` each statement
   is emitted on its own output line, so the later ones drifted onto source
   lines below (sometimes past the end of the file). The README listed this
   as a limitation.

Fix:

- `line` gains `srcLine, srcCol`: the true 1-based position of `text`'s
  first character, counted in characters. In the indentation path it is
  `i+1` and the length of the leading whitespace plus 1. `expandSemicolons`
  pieces add their rune offset (`sg.col`). Merged selector groups copy it.
- `bracesToIndent` now returns `(string, []srcPos)`: `pos[k]` is the source
  position of the statement on output line k+1. `scanHandlers` gets an
  optional `at func(i int)` hook, which `scanStructural` calls with the rune
  index just before each `text`/`interp` run. `buffer` uses it to find the
  column where a statement starts. `colOf` scans forward only, so the cost
  stays linear on a single-line (minified) file; counting back from each
  statement would be quadratic. Columns are measured against the original
  runes, so a `/* comment */` earlier on the line still counts.
- Every `Line: ln.lineNo, Col: ln.indent + 1` and every `diag` position in
  `parser.go`/`stmt.go` now uses `srcLine/srcCol`, and so do the line
  numbers passed to `lexLine`/`parseExpr`. This fixes error positions and
  LSP ranges too, not only maps. `lineNo` is unchanged: it is the
  structural line, and comment attachment still keys on it.
- The value column is `ln.srcCol + valToks[0].Col - 1`. Token columns are
  runes from the start of `ln.text`, and the brace rewrite copies statement
  text verbatim (dropping only comments), so the offset carries over.
- `sameAST` (the formatter's guard) now skips `ValueLine`/`ValueCol` as
  well as `Line`/`Col`. Without that, every reformat that moved a value
  looked like a change in meaning and was refused.

## Tests (`sourcemap_value_test.go`)

- `decodeSegments` decodes a mappings string into absolute segments.
  `mappedSource` finds the segment that starts exactly at a piece of output
  and returns the source text it points at.
- `TestSourceMapValues` has 14 cases: variable use site, computed value,
  `!important`, tabs (value and property), brace syntax, a one-line block
  (value and property), a comment before a rule, `;` in indentation syntax,
  a mixin body, compressed output, and a string after non-ASCII text.
- `TestSourceMapNoValueSegment` covers `add-property()`.
  `TestBraceOneLinerErrorPosition` checks that an error inside a one-line
  block reports `2:20`.
- All of them except the property-only cases failed with the parser/eval
  changes stashed. The full suite passes. `FuzzCompile` ran for 30s and
  `FuzzFormat` for 20s with no failures.
- Cost: the `sourcemap` variant benchmark went from about 1.57ms to about
  1.59ms per op (one more segment per declaration). Brace syntax is about
  2% slower.

## Found along the way → N-049

A source map lists only the entry file. `css.Pos` has no file, so rules from
an `@import`ed file are mapped onto the entry file's line numbers. A probe
showed `.lib` from line 4 of `_lib.styl` mapped to line 4 of a 3-line
`app.styl`. This bug predates the session and was left as its own item,
because fixing it needs a file on `Pos`, a `sources` index, paths relative
to the map, and the imported files' text for `sourcesContent`. It is now
listed under README "Limitations".

## README

- The features bullet now says maps cover declaration values.
- The Limitations entry about one-line brace blocks having approximate
  positions is removed (no longer true) and replaced by the import
  limitation.
- The roadmap line is split: value-level mapping is checked off, and
  "deeper compress parity, more built-ins" stays under Future.

## Next

Closed: N-003. Declined: None. Raised: N-049.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
