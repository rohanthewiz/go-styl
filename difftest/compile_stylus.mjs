// Compiles a batch of .styl files with the reference (npm) Stylus compiler.
// Reads a JSON array [{id, path}] on stdin; writes {id: {css} | {error}} on
// stdout. One process for the whole corpus keeps the differential test fast.
import { readFileSync } from 'node:fs'
import stylus from 'stylus'

const files = JSON.parse(readFileSync(0, 'utf8'))
const out = {}
let pending = files.length
if (pending === 0) {
  process.stdout.write('{}')
  process.exit(0)
}
const done = () => {
  if (--pending === 0) process.stdout.write(JSON.stringify(out))
}
for (const f of files) {
  let src
  try {
    src = readFileSync(f.path, 'utf8')
  } catch (e) {
    out[f.id] = { error: String(e.message || e) }
    done()
    continue
  }
  try {
    stylus(src)
      .set('filename', f.path)
      .set('compress', true)
      .render((err, css) => {
        out[f.id] = err ? { error: String(err.message || err) } : { css }
        done()
      })
  } catch (e) {
    out[f.id] = { error: String(e.message || e) }
    done()
  }
}
