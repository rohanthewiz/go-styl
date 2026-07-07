# Session: worker runner mirrored into go-styl playground

- **Date:** 2026-07-06 19:29
- **Session ID:** `df41691e-e1c2-4246-ae55-b7216cc91a26` (element-repo session;
  this segment mirrors work done there — see element's
  `2026-0706-1800-go-learn platform build and deploy.md` and
  `2026-0706-1835-go-learn worker runner backport.md`)
- **Repo:** `~/projs/go/go-styl` (branch `main`; **freshly cloned this
  session — no local checkout existed on this machine**)
- **Commit:** `6dec242` playground: run the compiler in a web worker
- **Live & verified:** https://rohanthewiz.github.io/go-styl/

> Third and final playground moved onto the web-worker runner pattern
> (go-learn originated it; element `0878fc5` backported it; this mirrors it
> here). Motivation: `EvalWithContext`-style wasm-side timeouts NEVER fire
> in browser wasm — a spinning goroutine starves Go's timer goroutine — so
> only worker.terminate() gives real preemption.

## 1. What changed (4 files, playground/ only; no Go changes)

- **`worker.js` (new)** — importScripts('wasm_exec.js'), instantiates
  `styl.wasm`; `goStylReady` → postMessage `{type:'ready', version,
  examples}` (examples ride the handshake — the page can no longer touch
  `goStyl` directly). `{id, src, opts}` → `{type:'result', id, r}` where
  `r = goStyl.compile(src, opts)`; compiles are synchronous in the worker
  so results are FIFO. NOTE the API difference vs element/go-learn: it's
  `compile(src, opts)` (two args, opts structured-cloned), not `run(src)`.
- **`runner.js` (new)** — global **`stylRun`**: `compile(src, opts) ->
  Promise`, `isReady()`, `onBoot(cb)` (fires once, first boot only),
  `version()`, `examples()`. 6s watchdog → terminate + silent respawn
  (no auto-recompile — a still-pathological draft would re-wedge in a kill
  loop); compiles queued behind a wedge resolve with a restart error;
  requests made while (re)spawning queue and flush on ready.
- **`index.html`** — dropped wasm_exec.js tag + main-thread wasm boot;
  `compile()` async with `compileSeq` superseded-guard (the opts/globals
  gathering stays synchronous, before dispatch); boot chrome moved into
  `stylRun.onBoot`; data-URI favicon added (kills the console 404).
- **`tutorial.js`** — `compile()` async, guarded by
  `seq !== compileSeq || LESSONS[cur] !== l`; `window.goStyl` readiness
  check → `window.stylRun`.

Deploy needed no workflow change: pages.yml's `cp playground/*.js` glob
stages runner.js + worker.js automatically.

## 2. Finding: go-styl already self-guards ranges

The pathological-input probe (`for i in 1..3000000`) did NOT hit the
watchdog — the compiler errors fast: `range 1..3e+06 exceeds 65536
elements`. So for go-styl the worker is a safety net (unbounded mixin
recursion, compiler bugs) rather than a live bug fix. Element/go-learn
genuinely needed it (interpreted user `for {}`).

## 3. Verification (ALL PASS, locally on :8093 and against live Pages)

11-check playwright drive (`drive_styl.mjs` pattern): worker boot · default
compile → CSS · examples menu · version · pretty-toggle recompile differs ·
pathological input → fast error, page responsive · recovery compile ·
tutorial lesson 1 output · solution completes + tick · no console errors.

## 4. Cross-repo state after this session

All three playgrounds share the worker-runner architecture, **copied not
shared**: go-learn `engine/{worker,runner-go}.js` (goRun, run(src)),
element `playground/{worker,runner}.js` (eleRun, run(src), examples in
handshake), go-styl `playground/{worker,runner}.js` (stylRun,
compile(src,opts), examples in handshake). If a fourth appears, extract a
shared implementation. Element-side memory file
`wasm-eval-timeout-needs-worker.md` records the full picture.
