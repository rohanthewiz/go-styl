# Session: user block mixins (N-041) (2026-09-27)

Session ID: `8f2b7a50-2611-43dd-a08e-44874f40a02d`

Implemented next-list item N-041: user mixins accept a block. `+m(args)`
followed by an indented body passes the body to `m`, which emits it wherever
it writes `{block}`. Previously only the built-in `+prefix-classes` took a
block; a user mixin given one was an error.

```styl
mobile(w = 600px)
  @media (max-width: w)
    {block}
.btn
  +mobile()
    color c        // → @media (max-width:600px) { .btn { color: … } }
```

## Parser

- **`ast.BlockSlot`** — new statement for a line that is exactly `{block}`
  (a trailing `;` allowed). Matched as whole-line text in `parseLine`
  before tokenizing, since the lexer reads a leading `{` as interpolation.
  Added to `ast.Pos` and the `stmtNode` set.
- **`braces.go`** — `scanStructural` copies `{block}` verbatim (like
  interpolation/object braces) instead of treating it as a block brace, so
  brace syntax (`m() { .z { {block} } }`) works. `usesBraces` was already
  unaffected (body has no `\n;:`).
- `parseBlockMixinCall` (already existed) produces `MixinCall.Block`.

## Evaluator

**Key decision: the block keeps its call site's lexical context but emits
at the slot.** `passedBlock` (eval.go) captures body, call-site scope, file,
import dir, enclosing mixin name, and `outer` (the call site's own block).
`blockSlotCtx` (context.go) builds the run context:

- lexical side from the call site: `scope.Child()` (assignments stay local,
  as in a ruleset body), `file`, `dir`, `mixin`, `block: outer`;
- emission side from the slot: `rule`, `parents`, `sink`, `stack`, `media`,
  `prefix`, `propRule`.

`outer` means a `{block}` inside a passed body refers to the enclosing
mixin's block — nested block mixins (`outer()` passing its block into
`+inner()`) work.

- `execCtx.block` carries the block; every child context construction
  propagates it (evalRuleSet, the three m5.go at-rule children, and the
  migrate.go equivalents), so `{block}` works at any depth in the body.
- `invoke` gained a `blk *passedBlock` parameter (two callers pass nil).
- `evalBlockMixinCall`: a user mixin of that name now receives the block via
  `invoke(…, newPassedBlock(s.Block, ctx))`; a user mixin shadows the
  built-in `prefix-classes`, matching ordinary call precedence.
- A `return` inside the passed body ends only the body (not propagated),
  matching Stylus.

**Semantics vs Stylus (checked against stylus 0.64 CLI):**
- Mixin called without a block → `{block}` is empty (Stylus same). Allows
  optional blocks.
- `{block}` outside any mixin → error `{block} is only valid inside a mixin
  body`. Stylus silently drops it; deliberate difference (can only be a
  mistake).

## Migrate

`migrator.mixinCall` routes to `blockMixinCall` only when no user mixin
exists (built-in prefix-classes); otherwise it sets `fctx.block`. A
`*ast.BlockSlot` case in `execInner` walks the body with `m.walk` in the
slot's node, so rules nest in the migrated output like the mixin body.

## Tests / docs

- `block_mixin_test.go`: media wrapper, slot twice with args, nested block
  mixins, inside `+prefix-classes`, optional block, `return` in block,
  brace syntax, user shadowing built-in, outside-mixin error, migrate.
- `difftest/corpus/block-mixins.styl` matches reference stylus (score
  24/36 → 25/36).
- README bullet under functions/mixins; next-list N-041 closed.
- `styl fmt` leaves `{block}` lines unchanged (verified).

## Next

Closed: N-041. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
