// Times the reference (npm) Stylus compiler for the bench runner. Reads a
// JSON array [{id, path, iters}] on stdin; writes {id: {ns} | {error}} on
// stdout. One process for the whole corpus keeps node startup out of the
// per-file numbers. The stylus package is resolved from difftest/node_modules
// (shared with the differential test).
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'

const require = createRequire(new URL('../difftest/package.json', import.meta.url))
const stylus = require('stylus')

const jobs = JSON.parse(readFileSync(0, 'utf8'))
const out = {}

// stylus .render(cb) is synchronous for local sources (file reads use
// readFileSync); renderOnce turns the callback style into call-or-throw and
// guards the assumption.
function renderOnce(src, path) {
  let css, error
  stylus(src).set('filename', path).set('compress', true).render((e, c) => { error = e; css = c })
  if (error) throw error
  if (css === undefined) throw new Error('stylus rendered asynchronously; cannot time')
  return css
}

for (const j of jobs) {
  try {
    const src = readFileSync(j.path, 'utf8')
    renderOnce(src, j.path) // warmup + validity check
    renderOnce(src, j.path)
    const t0 = performance.now()
    for (let i = 0; i < j.iters; i++) renderOnce(src, j.path)
    out[j.id] = { ns: Math.round(((performance.now() - t0) * 1e6) / j.iters) }
  } catch (e) {
    out[j.id] = { error: String(e.message || e) }
  }
}
process.stdout.write(JSON.stringify(out))
