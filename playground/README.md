# go-styl WASM playground

A browser playground for go-styl: the compiler built for `js/wasm`, driving a
two-pane editor (Stylus in, CSS out) with live recompilation, syntax
highlighting (toggleable via the header checkbox), positioned errors, the
bundled `examples/`, optional source-map output, and a Go-side options panel
for `Options.Globals` / `Options.CustomProperties` (runtime theming: seed
Stylus variables from "Go", expose them as `:root` custom properties).

A second tab hosts an **interactive tutorial**: 20 lessons from first
selectors through mixins, control flow, `@extend`, at-rules, `@import`, and
go-styl's runtime theming, each with prose, a live editor, an auto-checked
exercise, and a loadable solution. Progress is stored in `localStorage`;
`index.html#tutorial` deep-links to it.

## Build & run locally

```sh
./playground/build.sh        # produces playground/styl.wasm + wasm_exec.js
go run ./playground/serve    # http://localhost:8080
```

Any static file server works — the page is `index.html` plus two small local
scripts (`highlight.js`, `tutorial.js`) with no dependencies beyond the two
build artifacts (which are gitignored).

## Pieces

- `wasm/main.go` — `js/wasm` entry point; installs a global `goStyl` object:
  `goStyl.compile(src, {pretty, mergeDuplicates, sourcemap, globals,
  customProperties})`, `goStyl.examples()`, `goStyl.version`. `globals` is an
  object (string values are parsed as Stylus expressions; numbers, booleans,
  and null map directly), `customProperties` an array of variable names.
  `@import` resolves against the embedded `examples/` filesystem.
- `index.html` — the UI shell: tabs, panes, options, wasm loading (vanilla
  JS/CSS, dark/light via `prefers-color-scheme`).
- `highlight.js` — dependency-free syntax highlighters (Stylus + CSS) and the
  overlay-editor wiring (highlighted `<pre>` behind a transparent-text
  `<textarea>`; the two must stay character-identical, which is why color
  swatches only appear in the output pane).
- `tutorial.js` — lesson data + tutorial UI. The lesson array is plain data,
  `require`-able from Node, so snippets can be batch-verified against the real
  compiler via `cmd/styl`.
- `serve/` — tiny dev file server.
- `../.github/workflows/pages.yml` — builds and deploys to GitHub Pages on
  push to main (needs Pages enabled with source "GitHub Actions").
