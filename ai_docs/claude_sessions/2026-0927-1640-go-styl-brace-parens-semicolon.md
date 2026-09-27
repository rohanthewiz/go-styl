# Session: `;` inside parentheses in brace syntax (N-037) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Implemented next-list item N-037, raised earlier this session while doing
N-027. In brace syntax, a `;` inside parentheses ended the statement, so
`.a { background: url(data:image/png;base64,AAA=) }` failed with
`unexpected ","`. The indentation syntax already kept it (N-027's
`splitSemicolons` skips parens).

Same session as N-027…N-030, N-022, N-035 and N-036
(`2026-0927-1625` … `-1639`).

## Implementation

`internal/parser/braces.go` (`scanStructural`):

- New `parens` counter. `(` and `)` are emitted as text (as before, via the
  default branch) and adjust the depth.
- `case c == ';' && parens > 0` emits the `;` as text instead of firing
  `semi`.
- The depth resets to 0 on newline, `{` (block open) and `}`. An unbalanced
  `(` (a typo) can then only swallow the `;` of its own line, and the
  closing `}` still closes the block. Verified: `.c { x: f(1; 2 }` gives a
  parse error for `.c`, and the next rule still parses.

`usesBraces` shares the scanner and is unaffected. Its body check looks
for `\n`, `;` or `:` anywhere.

## Tests

- `semicolon_test.go` gained `TestSemicolonInParensBraceSyntax`: a
  one-liner data URI, a multi-line block, and `calc(...)` followed by
  another statement on the same line.
- `go test ./...` passes; `FuzzCompile` ran 15s clean.
- README "Deliberate differences": the `url(...)` bullet now says both
  syntaxes.

Gotcha: an error inside a one-liner block reports a drifted line number
(8 for a 5-line file). That's the existing, documented limitation for
statements that share a source line in brace syntax, not new.

## Next

Closed: N-037. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
