# Session: source names relative to the map file (N-050) (2026-10-01)

Session ID: `f6f39913-b1d4-49e4-b91c-87cd121b66d5`

The N-049 session noted, without raising it, that `sources` names were
relative to the entry path as given, not to where the `.map` is written.
`styl -o out/app.css -sourcemap styles/app.styl` listed `styles/app.styl`.
The v3 spec resolves each `sources` entry against the map's own URL, so
the right name is `../styles/app.styl`. This session fixed that and logged
it as N-050.

## Result

```
styl -o out/main.css -sourcemap testdata/imports/main.styl
  sources: ["../testdata/imports/main.styl", "../testdata/imports/_vars.styl"]
styl -o testdata/imports/main.css -sourcemap testdata/imports/main.styl
  sources: ["main.styl", "_vars.styl"]
```

stylserve with an OS `Dir` used to list the absolute server path of the
stylesheet. It now lists `app.styl`.

## `Options.MapFile` (`styl.go`, `internal/eval/eval.go`)

- The new public option is the path the map is written to or served from.
  It is in `Filename`'s terms: an OS path, or an fs path under `FS`.
- It is passed through `Build` and `CompileMap` to `eval.Options.MapFile`.
- When empty, naming is unchanged: the entry is named by `Filename`, and
  each import in its terms (`sourceName`, from N-049).

## Naming (`internal/eval/sourcemap.go`)

- `entryLoc()` gives the entry's location in the same form as import keys:
  - `Filename`, made absolute in OS mode or `path.Clean`ed under FS;
  - for a bare-string compile, `BaseDir/input.styl`, matching where
    `sourceName` takes such an entry to sit.
- `mapRelName(loc)` takes `Rel(dir(MapFile), loc)`:
  - in OS mode the map dir is made absolute first, so a relative `MapFile`
    and an absolute import key compare correctly;
  - under FS the comparison is lexical (fs paths share the FS root).
  - ok=false (no `MapFile`, or no relative path, e.g. different Windows
    volumes) keeps the default name.
- `newSourceMap` applies it to the entry name (passed to `NewSourceMap`)
  and to every `AddSource` name.
- The file's header comment gains a "Relative to the map" section.

## Callers

- CLI `runWithSourceMap`: `MapFile: mapPath` (`<out>.map`).
- `stylserve.Engine.build`: `MapFile` is `<Dir>/<cssPath>.map`
  (`path.Join(rootOr(Dir), …)` under FS, `filepath.Join` on the OS).
  - The served URL tree mirrors the source tree, so in source terms the map
    sits beside the `.styl` file.
- The playground (`Build` with `Filename: "playground.styl"`) is unchanged.

## Tests

- `TestSourceMapMapFile` (`sourcemap_value_test.go`) has subtests:
  - relative entry with the map in `out/`;
  - map beside the entry;
  - absolute temp-dir entry and map, giving relative `../styles/…` names;
  - fs paths (`dist/` and beside);
  - bare-string `Build` with `BaseDir` (`../styles/input.styl`).
- `stylserve`:
  - `TestEngineSourceMaps` now asserts `"sources":["app.styl"]`.
  - The new `TestEngineSourceMapNames` covers an fs `Dir` with a nested
    sheet that imports `../shared/_x`, giving
    `["page.styl","../shared/_x.styl"]`.
- With the implementation reverted, all of these fail. The full suite and
  `go vet` pass.
- Process slip: the revert step's `git checkout styl.go internal/eval/eval.go`
  also undid those two files' edits, since they weren't stashed. They were
  re-applied and re-verified.

## README

- The Source maps feature bullet mentions `MapFile`.
- `MapFile` is added to the options table.
- The middleware and `-sourcemap` paragraphs describe the naming.
- An out-of-date Limitations line ("do not yet map inside values", wrong
  since N-003) now says values are mapped but individual tokens inside a
  value are not.

Observed, not raised: `styl -o dir/x.css` fails when `dir/` doesn't exist.
The CLI doesn't create output directories, which is unchanged behaviour.

## Next

Closed: N-050. Declined: None. Raised: N-050 (logged and closed this session).
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
