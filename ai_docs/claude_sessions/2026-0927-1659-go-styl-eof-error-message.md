# Session: "unexpected end of expression" (N-039) (2026-09-27)

Session ID: `851f02c0-9693-4a4f-9e10-f398cc824bd3`

Fixed next-list item N-039, raised in the previous item (N-016). An
expression that ended where an operand was due (`width 1px +`,
`x = (1 +`) reported `unexpected "" in expression`, because the EOF
token's text is empty.

Same session as the items from `2026-0927-1625` to `-1658`.

## Change

- `internal/parser/expr.go` (`parseOperand`): a `case token.EOF` before
  the default now returns `unexpected end of expression`. The other
  `unexpected %q` site (leftover tokens after a full expression) can't
  see EOF, so it's unchanged.
- `m7_test.go`: new `TestM7ErrorPositions` case: `body` / `width 1px +`
  gives `<input>:2:3: unexpected end of expression`.

No test pinned the old text (grepped). `go test ./...` passes. The
playground shows the new message with its line mark (N-016).

## Next

Closed: N-039. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
