# Session: CLI creates missing output directories (N-051) (2026-10-01)

Session ID: `f6f39913-b1d4-49e4-b91c-87cd121b66d5`

This is the second part of the session that closed N-050. That doc
observed, without raising it, that `styl -o dir/x.css` failed with
`open …/dir/x.css: no such file or directory` when `dir/` didn't exist.
The user asked for the fix, and it is logged here as N-051.

## `writeOutput` (`cmd/styl/main.go`)

```go
func writeOutput(path string, data []byte) error // MkdirAll(dir(path), 0755), then WriteFile(…, 0644)
```

`MkdirAll` is skipped when the directory is `.`. Every output write the
CLI makes now goes through it:

- `main.go`: the plain `-o` compile, and `runWithSourceMap`'s CSS and
  `<out>.map`.
- `gen.go`: `-o` (the Go constants file) and `-scoped -css`.
- `migrate.go`: `-o`, and `writeSplit` for `-split -o dir`, which replaces
  its own `MkdirAll` + `WriteFile` pair.
- `fmt -w` keeps a plain `os.WriteFile`, since it only rewrites files that
  already exist.

## Tests

- `cmd/styl/main_test.go` (the package's first test):
  `TestWriteOutputCreatesDirs` writes `build/css/app.css` into a temp dir,
  then `.map` beside it, and reads the CSS back.
- End to end, a built binary wrote into a clean scratch dir with one fresh
  subdirectory per command:
  - `-o a/app.css`;
  - `-o b/app.css -sourcemap` (CSS and map);
  - `gen -o c/css_gen.go`;
  - `migrate -o d/app.css`;
  - `migrate -split -o e/out` (`main.css`, `tokens.css`).
  All exited 0.
- `go vet ./cmd/styl` is clean; the `cmd/styl` and root-package tests pass.

## Docs

The `-o` line in `cmd/styl/main.go`'s header comment and the README CLI
section both note that missing output directories are created.

## Next

Closed: N-051. Declined: None. Raised: N-051 (logged and closed this session).
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
