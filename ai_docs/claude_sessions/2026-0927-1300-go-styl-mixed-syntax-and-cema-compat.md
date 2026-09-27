# Session: cema compatibility probe, next-list seed, mixed-syntax fix (2026-09-27)

Session ID: `fc4af2a6-9084-41cc-b869-9797596573c8`

Started in cema (`~/projs/go/church/cema`) with the question "what would it
take to use go-styl for cema's Stylus compilation?" The answer came from
compiling cema's real sheets and diffing against stylus 0.64. That produced
a list of go-styl gaps. The go-styl living next list was then seeded, and the
first item (N-002, mixed syntax) was fixed. The cema side of the story is in
cema's `ai_docs/claude_sessions/2026-0927-1300-go-styl-compat-probe.md`.

## 1. cema compatibility probe

cema compiles `styles/styl/master.styl` → `dist/css/app.css` and six
`styles/styl/theme_masters/*.styl` → `dist/css/themes/`. It uses the npm
`stylus` 0.64 CLI.

Method (scratchpad, session-scoped; recreate if needed):

- `flatten.py` imitated `@require` (import once) and glob imports by inlining
  each entry into one file.
- `run.sh` patched each go-styl gap in the flat copy, then compiled with a
  go-styl CLI built from this repo.
- `cmp.py` compared rules semantically. It normalized whitespace, hex, and
  leading zeros, and expanded selector groups to per-selector declaration maps.

Go-styl gaps found (all now items in `ai_docs/todo/next-list.md`):

| Item | Gap |
|---|---|
| N-023 | `@require` passes through verbatim; no glob imports |
| N-024 | Multi-line selector groups: a trailing-comma continuation fails even at the top level. Stacked selector lines fail, **or silently misparse**: `td:nth-child(1)` becomes a declaration. |
| N-025 | A trailing `&` (`.checkbox &`) is emitted literally |
| N-026 | `"\2022"` loses its backslash |
| N-027 | A trailing `;` and multiple declarations per line fail in indented syntax (337 lines in cema) |
| N-028 | The `"…%s…" % x` sprintf operator is missing |
| N-029 | Numbers print as `1.7999999999999998rem` |
| N-030 | Calling an undefined mixin errors (Stylus silently drops it). Keep the error, but document it. |
| N-002 | Mixed indentation + brace files. **Fixed this session.** |

Harmless differences: named colors are not converted to hex, and quote style
in `url()` differs.

cema source quirks that Stylus hides:

- **`_material_form.styl`:** `.form-help {` sits at column 0 inside a mixin.
  Stylus emits the whole brace section, and even the mixin body, at the root.
- **`_banner.styl`:** a `/* */` inside `//` comments un-nests
  `#banner-wrapper` in Stylus's output.
- **Dead calls:** `sermon()`, `fullcalendar()` and a bodiless
  `payment_form()`.

## 2. Next list seeded

`/next-list seed` over all 14 prior docs, plus the probe. Most docs carry
their follow-ups under Roadmap and Carry-forward, not a Next section.

- **Lapsed items restored:** N-001 and N-002 (dropped after m6a-m6b), N-003,
  N-004 and N-006 to N-008 (dropped after m11), and N-012 (dropped after
  playground-globals).
- **Closed while seeding:** the unquoted `url(/x.png)` lexer issue (N-032,
  verified fixed), and a nested module for the rweb middleware (N-033),
  overtaken when the middleware moved into rweb.
- **Non-goals:** CLI watch mode (N-031) and vendor-prefixed `@keyframes`
  (N-034).
- **N-001's premise corrected:** Stylus leaves `calc()` args literal too, so
  only `url()` evaluation is a gap.

## 3. N-002: mixed indentation + brace syntax

Before this fix, one block brace anywhere made `Parse` send the whole file
through `bracesToIndent`. That derived depth purely from brace nesting and
dropped the source indentation, so indented mixins and rules in the same file
broke.

Changes in `internal/parser/braces.go` (`bracesToIndent`):

- **Inside braces:** depth is the innermost open header's depth plus 1
  (`braceDepths` stack).
- **Outside braces:** depth comes from the source indentation, through an
  `indentStack` that pops until it finds a shallower entry, the same way
  `buildTree` works.
- **Closed braced headers:** a header whose `{…}` block has closed is marked
  `braced` and never adopts indentation children. A deeper line after it
  becomes its sibling. This keeps pure brace files unchanged, for example a
  stray indent after `}`.
- **Same-line statements** (`a; b`, `} .b {`) are siblings of the previous
  statement.
- **A line after a statement ending in `,`** keeps the same depth. This
  prepares for N-024.
- **`{` on its own line** opens the previous statement.

Stylus's reference behaviour was probed first:

- It is indentation-sensitive outside braces.
- A deeper line after a closed braced block is a sibling at the root, and an
  error when nested. go-styl is lenient here and makes it a sibling.
- Indented lines inside braces are an error in Stylus. go-styl is lenient and
  treats them as brace content.

Verification:

- **Unit tests:** `mixed_syntax_test.go` has 7 cases, with expected values
  checked against stylus 0.64.
- **Full suite:** `go test ./...` passes, and gofmt is clean.
- **Differential test:** it had been skipping because its npm deps weren't
  installed (`npm install --prefix difftest`). Once installed, the score is
  unchanged at 23/32.
- **cema:** the flattened sheets compile with `_material_form.styl`'s braces
  intact. The output is identical, selector for selector, to the earlier run
  where I stripped the braces by hand, for all 7 sheets.

Found while testing:

- `.a,` then `.b {` still fails. It failed before this change too, so it
  stays with N-024.
- A standalone `{expr}` in any brace-using file is still unsupported, so
  `calc({x} - 8px)` fails in a flattened mixed file. `s("calc(%s - 8px)", x)`
  works in both compilers. Noted under N-007.

## Gotchas

- The editor diagnostics (`undefined: splitIndent`) came from cema's gopls
  workspace not including go-styl. They are not real errors.
- gofmt reflows a `//` diagram written as a list continuation. Put diagrams in
  a tab-indented code block after a blank `//` line.

## Next

Seeded `ai_docs/todo/next-list.md` this session: N-001–N-030 Open,
N-031 and N-034 Non-goals, N-032 and N-033 Closed while seeding.
Closed: N-002. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: N-007, N-022. Full list: `ai_docs/todo/next-list.md`.
