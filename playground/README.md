# go-styl WASM playground

A browser playground for go-styl: the compiler built for `js/wasm`, driving a
two-pane editor (Stylus in, CSS out) with live recompilation, positioned
errors, the bundled `examples/`, and optional source-map output.

## Build & run locally

```sh
./playground/build.sh        # produces playground/styl.wasm + wasm_exec.js
go run ./playground/serve    # http://localhost:8080
```

Any static file server works — the page is a single `index.html` with no
dependencies beyond the two build artifacts (which are gitignored).

## Pieces

- `wasm/main.go` — `js/wasm` entry point; installs a global `goStyl` object:
  `goStyl.compile(src, {pretty, mergeDuplicates, sourcemap})`,
  `goStyl.examples()`, `goStyl.version`. `@import` resolves against the
  embedded `examples/` filesystem.
- `index.html` — the whole UI (vanilla JS/CSS, dark/light via
  `prefers-color-scheme`).
- `serve/` — tiny dev file server.
- `../.github/workflows/pages.yml` — builds and deploys to GitHub Pages on
  push to main (needs Pages enabled with source "GitHub Actions").
