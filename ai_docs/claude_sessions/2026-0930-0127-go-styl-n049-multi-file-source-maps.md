# Session: multi-file source maps (N-049) (2026-09-30)

Session ID: `95d7311a-f8c4-4175-bf13-5af3cfa51f52`

This session closed N-049, which the N-003 work raised earlier in the same
session. Until now a source map listed only the entry file. `css.Pos` had no
file, so anything from an `@import`ed file was mapped onto the entry file's
line numbers. A probe had shown `.lib` from line 4 of `_lib.styl` mapped to
line 4 of a 3-line `app.styl`.

## Result

`styl -o out.css -sourcemap testdata/imports/main.styl`:

```json
"sources": ["testdata/imports/main.styl", "testdata/imports/_vars.styl"],
"sourcesContent": ["@import \"reset.css\"\n…", "brand = #3498db\n…"],
"mappings": ";;AAGA;CCCE,YAAW;CACX,eAAc;CDAd,OAAM"
```

`background` and `border-radius` come from the `button()` mixin, so they
point into `_vars.styl`. `color white` points back to `main.styl`.

## Positions carry a file (`internal/css/print.go`, `internal/eval`)

- `css.Pos` gains `File`, the evaluator's key for the file:
  - the entry uses `Options.Filename`, which may be `""`;
  - an import uses the path `resolveImport` returned (absolute OS path, or
    an fs path under `Options.FS`).
- Four places set it from `ctx.file`: declarations and values and rulesets
  (`eval.go`), and at-rules (`m5.go`).
- Mixin and function bodies already ran with `Closure.File`, and passed
  blocks with their own file. So a mixin's declarations map to the file that
  defines the mixin, with no extra work.
- The synthesized `:root` custom-properties rule and `add-property()` keep
  an empty File, which means the entry file.

## `css.SourceMap` with several sources (`internal/css/sourcemap.go`)

- `sources`/`contents` are parallel slices, and index 0 is always the entry.
  `SetEntry(key)` says which `Pos.File` value, besides `""`, means the entry.
- `AddSource(key, name, content)` registers an imported file. It joins
  `sources` on the first segment that points into it (`sourceIndex`). A
  partial with no output of its own (variables only) never appears.
- A key that was never registered returns -1, and that segment is dropped:
  no mapping is better than a wrong one.
- `segment` gains `src`. The encoder now writes real source-index deltas.
  Line and column deltas stay relative to the previous segment even when the
  source changes, as the v3 spec requires.
- `sourcesContent` is `[]*string`: a null stands for a source with no text,
  and the field is omitted when no source has text (the old behaviour).
- `sort.Slice` became `sort.SliceStable`, which gives a deterministic order
  for two segments at the same generated position.

## Naming imports (`internal/eval/sourcemap.go`)

Evaluator keys make poor `sources` names: in OS mode they are absolute,
while the entry is named however the caller wrote it. `sourceName` writes
each import in the entry name's terms. It takes the path relative to the
directory the entry is in, then joins that onto the entry name's directory:

- entry `styles/app.styl`, import `/cwd/styles/partials/_btn.styl` gives
  `styles/partials/_btn.styl`. An absolute entry gives absolute names.
- The entry's directory is `dir(Filename)`, or `BaseDir` for a compile of
  a bare string (named `input.styl`, so its imports come out as
  `partials/_a.styl`). If `BaseDir` differs from the entry's own directory,
  the result is still right: `a/app.styl` importing `b/_x.styl` becomes
  `b/_x.styl`, by way of `../b`.
- Under `Options.FS`, fs paths are already relative to the FS root, as the
  entry's is, so a lexical `filepath.Rel` is exact.
- `importFile` calls `noteSource`, which records each imported file's text
  once (only when `SourceMap` is set). `newSourceMap` registers them all
  before rendering.

Observed but not raised: names are relative to the entry path as given, not
to where the `.map` is written. `styl -o out/app.css -sourcemap
styles/app.styl` lists `styles/app.styl`, not `../styles/app.styl`. The
entry file already behaved this way, and DevTools shows the embedded text
either way.

## Tests (`sourcemap_value_test.go`)

- The helper `segmentAt` now reads the source index and returns the
  `sources` name plus the text from `sourcesContent`. `mappedSource` wraps
  it. `checkMap` asserts the `sources` list and a set of gen→(source, text)
  cases.
- `TestSourceMapImports` compiles an absolute temp-dir entry. It covers
  `_lib`'s rule on line 4 (past the end of the entry file) and a mixin in
  `partials/_btn.styl` that maps into that partial.
- `TestSourceMapImportsRelative` compiles `testdata/imports/main.styl`, so
  all names are relative.
- `TestSourceMapImportsFS` uses a `fstest.MapFS`. A variables-only partial
  is left out, a file imported twice is listed once, and a bare-string
  `Build` with `BaseDir` names imports next to `input.styl`.
- With `internal/eval` and `internal/css` stashed, all three fail. The full
  suite and `go vet` pass.
- `internal/css/sourcemap_test.go` was updated for the new `add(…, Pos)`
  signature.

README: the Source maps feature bullet now describes multi-file maps, and
the Limitations entry added for N-049 is removed.

## Next

Closed: N-049. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
