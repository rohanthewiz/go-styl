# Session: file-for-file `styl migrate -split` (N-044) (2026-09-27)

Session ID: `584be964-081a-458d-87c6-1306959d9b55`

Implemented next-list item N-044. Until now, `styl migrate` inlined every
`.styl` import into one stylesheet. With `-split` (`MigrateOptions.Split`),
the project's file layout is kept:

```
main.styl                        main.css
  @import "_vars"          →       @import "tokens.css";
  @import "buttons"                @import "buttons.css";
  .page { … }                      .page { … }
_vars.styl (vars/mixins only)    (no file; import dropped, noted)
buttons.styl (rules)             buttons.css
                                 tokens.css  (:root { --… })
```

## Design (`internal/eval/migrate_split.go` + hooks in `migrate.go`)

- **Same walk, different destination.** An imported file still runs in the
  importer's scope, so its variables and mixins stay visible to everything
  after it. Under Split, an import at a file root (`mc.node.isRoot`) gets a
  new `mfile` whose root receives the output. The importer keeps an
  `mImport` node that records the import's position.
- **`isRoot` on `mnode`** marks each file root. `walk` flushes hoisted rules
  into whichever file root it is walking (`flush(root)`).
- **Inlined anyway, with a note:**
  - an import inside a rule or at-rule, because its output depends on the
    enclosing context;
  - a plain second `@import` of a file already split out, because its CSS
    could differ (it reads the variables in scope at that point).
- **`renderSplit`** runs after `applyExtends`, since a partial's
  `$placeholder` can gain selectors late:
  - `hasCSS` decides, recursively along imports, whether a file emits
    anything. A file with no CSS vanishes (`import` note).
  - `placeImports` turns each `mImport` into `@import "<relURL>";` and
    moves all `@import` lines, generated and literal, up after the
    `@charset` and header comments. A `hoisted` note is recorded if an
    import had to pass a rule (the cascade order changes).
  - Comments directly above an import travel with it, and are dropped with
    a vanished import. The header is the run of leading comments that ends
    at a blank line in the source.
  - `:root` is built from the combined text of all files (`rootRule`) and
    written to the tokens file, which the entry sheet imports first.
- **Paths.** Output paths mirror the sources relative to the entry sheet's
  directory (`relSource`, which handles both absolute OS paths and FS
  paths), with `.styl` → `.css`. Sources outside that directory go under
  `_external/`. Name collisions are numbered (`b-2.css`); the entry and
  tokens names are reserved first. `tokensOut` keeps any tokens name inside
  the output root.
- **Render tweak (all modes):** consecutive one-line `@import`s have no
  blank line between them.

## API and CLI

- **`MigrateOptions`:** `Split` and `TokensFile`.
- **`MigrateResult.Files []MigratedFile{Path, Source, CSS}`:** the entry
  first (the same text as `CSS`), then tokens, then the imported files that
  emit CSS, in import order.
- **CLI:** `styl migrate -split -o <dir> [-tokens name]`. `-o` is required
  with `-split`. `writeSplit` creates the subdirectories and prints `wrote
  …` unless `-q`.

## Tests (`migrate_test.go`)

- **`TestMigrateSplitRoundTrip`:** migrates all 38 example and fixture
  sheets with Split, splices the files back together (`linkSplit` replaces
  each `@import` naming another output file with that file), and compares
  the result with `Compile` through `flatDecls`.
- **`TestMigrateSplitProject`** (fstest): relative imports from a
  subdirectory, a vanishing partial, a partial kept alive by `@extend`, an
  import inside a rule (inlined), and a late import (hoisted).
- **`TestMigrateSplitNames`:** `_external/`, collision numbering, and
  `TokensFile`.
- **`TestMigrateSplitRepeatImport`:** a repeated plain import is inlined.

## Known limits

- None of the repo's fixtures has an imported file that emits rules. On
  real files, `-split` produces only the entry sheet plus `tokens.css`; the
  rule-emitting cases are covered by the fstest tests only.
- The comments of a vanished partial go with it (variable doc comments
  still move to `:root`).

## Next

Closed: N-044. Declined: None. Raised: None. Deferred: None.
Promoted: None. Updated: None. Full list: `ai_docs/todo/next-list.md`.
