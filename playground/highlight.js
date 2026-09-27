// highlight.js — tiny, dependency-free syntax highlighters for the go-styl
// playground: Stylus source (indent or brace syntax) and generated CSS.
//
//   stylHi.styl(src)          -> HTML string (token <span>s)
//   stylHi.css(css)           -> HTML string (adds color swatches)
//   stylHi.escape(s)          -> HTML-escaped string
//   stylHi.editor(ta, code)   -> wires a <textarea> to its overlay <code>;
//                                returns a repaint function
//   stylHi.errorMark(ta)      -> marks a compile error's line in that editor
//   stylHi.go(src)            -> HTML for a Go snippet (tutorial prose)
//
// The Stylus tokenizer is a per-line scanner with one piece of context: a line
// whose next non-blank line is indented deeper (or that ends in `{` or `,`)
// opens a block and is highlighted as a selector; other lines are declarations,
// assignments, control flow, or at-rules. That heuristic is what lets
// `body a` (selector) and `color red` (declaration) read differently without
// a real parse.
(() => {
'use strict';

const ESC = { '&': '&amp;', '<': '&lt;', '>': '&gt;' };
const esc = s => s.replace(/[&<>]/g, c => ESC[c]);
const span = (cls, s) => s ? '<span class="' + cls + '">' + esc(s) + '</span>' : '';

const KW = /^(?:true|false|null|and|or|not|is|isnt|in|if|unless|else|for|return|arguments|is-defined|defined)$/;
const IDENT = /^-{0,2}[A-Za-z_$][-\w$]*/;

// --- expression / declaration-value scanner --------------------------------
// `st.comment` carries an open /* */ across lines. `swatch` adds an inline
// color chip before hex colors (used only where there is no textarea overlay
// that must stay column-aligned).
function value(text, st, swatch) {
  let out = '', i = 0;
  const n = text.length;
  while (i < n) {
    const rest = text.slice(i);
    let m;
    if (st.comment) {
      const end = rest.indexOf('*/');
      if (end < 0) { out += span('t-com', rest); break; }
      out += span('t-com', rest.slice(0, end + 2));
      st.comment = false; i += end + 2; continue;
    }
    if (rest.startsWith('/*')) { st.comment = true; continue; }
    if ((m = /^\/\/.*/.exec(rest))) { out += span('t-com', m[0]); i += m[0].length; continue; }
    if ((m = /^(['"])(?:\\.|(?!\1).)*\1?/.exec(rest))) { out += span('t-str', m[0]); i += m[0].length; continue; }
    if ((m = /^#[0-9a-fA-F]{3,8}\b/.exec(rest))) {
      out += swatch
        ? '<span class="t-col"><i style="background:' + m[0] + '"></i>' + m[0] + '</span>'
        : '<span class="t-col" style="text-decoration-color:' + m[0] + '">' + m[0] + '</span>';
      i += m[0].length; continue;
    }
    if ((m = /^(\d*\.)?\d+[a-zA-Z%]*/.exec(rest))) { out += span('t-num', m[0]); i += m[0].length; continue; }
    if ((m = /^![a-zA-Z]+/.exec(rest))) { out += span('t-kw', m[0]); i += m[0].length; continue; }
    if (rest[0] === '{' || rest[0] === '}') { out += span('t-int', rest[0]); i++; continue; }
    if ((m = IDENT.exec(rest))) {
      const w = m[0];
      out += span(rest[w.length] === '(' ? 't-fn' : KW.test(w) ? 't-kw' : 't-id', w);
      i += w.length; continue;
    }
    if ((m = /^\s+/.exec(rest))) { out += m[0]; i += m[0].length; continue; }
    out += span('t-op', rest[0]); i++;
  }
  return out;
}

// --- selector scanner -------------------------------------------------------
function selector(text, st) {
  let out = '', i = 0;
  const n = text.length;
  while (i < n) {
    const rest = text.slice(i);
    let m;
    if (st.comment || rest.startsWith('/*') || rest.startsWith('//'))
      return out + value(rest, st);
    if ((m = /^(['"])(?:\\.|(?!\1).)*\1?/.exec(rest))) { out += span('t-str', m[0]); i += m[0].length; continue; }
    if ((m = /^[.#$][-\w]+/.exec(rest))) { out += span('t-cls', m[0]); i += m[0].length; continue; }
    if ((m = /^::?[-\w]+/.exec(rest))) { out += span('t-pse', m[0]); i += m[0].length; continue; }
    if (rest[0] === '&') { out += span('t-amp', '&'); i++; continue; }
    if (rest[0] === '{' || rest[0] === '}') { out += span('t-int', rest[0]); i++; continue; }
    if ((m = /^(\d*\.)?\d+%?/.exec(rest))) { out += span('t-num', m[0]); i += m[0].length; continue; }
    if ((m = IDENT.exec(rest))) { out += span('t-sel', m[0]); i += m[0].length; continue; }
    if ((m = /^\s+/.exec(rest))) { out += m[0]; i += m[0].length; continue; }
    out += span('t-op', rest[0]); i++;
  }
  return out;
}

// --- property-name scanner (handles {interp} inside the name) --------------
function property(text) {
  let out = '', i = 0;
  while (i < text.length) {
    const m = /^\{[^}]*\}/.exec(text.slice(i));
    if (m) {
      out += span('t-int', '{') + span('t-id', m[0].slice(1, -1)) + span('t-int', '}');
      i += m[0].length;
    } else {
      const j = text.indexOf('{', i);
      const chunk = j < 0 ? text.slice(i) : text.slice(i, j);
      out += span('t-prop', chunk);
      i += chunk.length || 1;
    }
  }
  return out;
}

// --- one Stylus line, role already decided ----------------------------------
function stylLine(t, opens, st) {
  let m;
  if (t.startsWith('//') || t.startsWith('/*')) return value(t, st);
  if ((m = /^@[-\w]+/.exec(t))) {
    const rest = t.slice(m[0].length);
    const asSel = /^@extends?$/.test(m[0]);
    return span('t-at', m[0]) + (asSel ? selector(rest, st) : value(rest, st));
  }
  if ((m = /^(else if|if|else|unless|for|while|return)(?![-\w])/.exec(t)))
    return span('t-kw', m[0]) + value(t.slice(m[0].length), st);
  if ((m = /^(-{0,2}[A-Za-z_$][-\w$]*)(\s*)(\?=|:=|\+=|-=|\*=|\/=|=(?!=))/.exec(t)))
    return span('t-var', m[1]) + m[2] + span('t-op', m[3]) + value(t.slice(m[0].length), st);
  if (opens || /\{\s*$/.test(t) || /,\s*$/.test(t)) {
    if ((m = /^(-{0,2}[A-Za-z_][-\w]*)\(/.exec(t)))
      return span('t-fn', m[1]) + value(t.slice(m[1].length), st);
    return selector(t, st);
  }
  if (/^[.#&>+~*\[:$}]/.test(t)) return selector(t, st);
  if ((m = /^(-{0,2}[A-Za-z_][-\w]*)\(/.exec(t)))       // mixin call
    return span('t-fn', m[1]) + value(t.slice(m[1].length), st);
  if ((m = /^((?:\{[^}]*\}|[-\w$])+)(\s*:?\s*)/.exec(t)))
    return property(m[1]) + m[2] + value(t.slice(m[0].length), st);
  return value(t, st);
}

// --- whole Stylus source ----------------------------------------------------
function styl(src) {
  const lines = src.split('\n');
  const indents = lines.map(l =>
    /^\s*$/.test(l) ? -1 : l.match(/^[ \t]*/)[0].replace(/\t/g, '  ').length);
  const st = { comment: false };
  const out = [];
  for (let li = 0; li < lines.length; li++) {
    const line = lines[li];
    if (st.comment) { out.push(value(line, st)); continue; }
    const lead = line.match(/^[ \t]*/)[0];
    const t = line.slice(lead.length);
    if (!t) { out.push(esc(line)); continue; }
    let next = -1;
    for (let j = li + 1; j < lines.length; j++)
      if (indents[j] >= 0) { next = indents[j]; break; }
    out.push(lead + stylLine(t, next > indents[li], st));
  }
  return out.join('\n');
}

// --- generated CSS ----------------------------------------------------------
function css(text) {
  let out = '', i = 0;
  const stack = ['sel'];
  const st = { comment: false };
  while (i < text.length) {
    const rest = text.slice(i);
    let m;
    if ((m = /^\s+/.exec(rest))) { out += m[0]; i += m[0].length; continue; }
    if ((m = /^\/\*[^]*?(?:\*\/|$)/.exec(rest))) { out += span('t-com', m[0]); i += m[0].length; continue; }
    if (rest[0] === '}') {
      if (stack.length > 1) stack.pop();
      out += span('t-op', '}'); i++; continue;
    }
    if (stack[stack.length - 1] === 'sel') {
      if (rest[0] === '@') {
        m = /^@[-\w]+/.exec(rest);
        out += span('t-at', m[0]); i += m[0].length;
        const p = /^[^{;]*/.exec(text.slice(i))[0];
        out += value(p, st, true); i += p.length;
        if (text[i] === '{') {
          stack.push(/^@(media|supports|keyframes|-[-\w]+-keyframes|document|layer|container)$/.test(m[0]) ? 'sel' : 'body');
          out += span('t-op', '{'); i++;
        } else if (text[i] === ';') { out += span('t-op', ';'); i++; }
        continue;
      }
      m = /^[^{}@]+/.exec(rest);
      if (m) {
        out += selector(m[0], st); i += m[0].length;
        if (text[i] === '{') { stack.push('body'); out += span('t-op', '{'); i++; }
        continue;
      }
      if (rest[0] === '{') { stack.push('body'); out += span('t-op', '{'); i++; continue; }
      out += span('t-op', rest[0]); i++; continue;
    }
    // rule body: `prop: value;`
    if ((m = /^(--[\w-]+|\*?[-\w$]+)(\s*)(:)/.exec(rest))) {
      out += span('t-prop', m[1]) + m[2] + span('t-op', ':');
      i += m[0].length;
      const v = /^[^;}]*/.exec(text.slice(i))[0];
      out += value(v, st, true); i += v.length;
      if (text[i] === ';') { out += span('t-op', ';'); i++; }
      continue;
    }
    out += span('t-op', rest[0]); i++;
  }
  return out;
}

// --- overlay editor wiring ---------------------------------------------------
// The <textarea> sits on top with transparent text (its wrapper's CSS handles
// that); the highlighted copy lives in `code` inside an overflow-hidden <pre>
// behind it. `enabled()` lets the host toggle highlighting off cheaply.
function editor(ta, code, enabled) {
  const pre = code.parentElement;
  const paint = () => {
    code.innerHTML = (enabled ? enabled() : true) ? styl(ta.value) + '\n' : esc(ta.value) + '\n';
  };
  const sync = () => { pre.scrollTop = ta.scrollTop; pre.scrollLeft = ta.scrollLeft; };
  ta.addEventListener('input', paint);
  ta.addEventListener('scroll', sync);
  paint();
  return () => { paint(); sync(); };
}

// --- Go highlighter ------------------------------------------------------------
// go(src) highlights Go snippets in tutorial prose. A single left-to-right
// scan (not per line, since raw `strings` can span lines), reusing the
// token classes of the Stylus/CSS highlighters:
//
//   t-com  // and /* */ comments (including //go:embed directives)
//   t-str  "…", '…' and `…` literals
//   t-num  numbers, true/false/nil/iota
//   t-kw   keywords
//   t-cls  predeclared types, and exported names after a dot (styl.Options)
//   t-fn   an identifier directly followed by "(" (a call or declaration)
//   t-op   operators
const GO_KW = /^(?:break|case|chan|const|continue|default|defer|else|fallthrough|for|func|go|goto|if|import|interface|map|package|range|return|select|struct|switch|type|var)$/;
const GO_TYPE = /^(?:any|bool|byte|comparable|complex64|complex128|error|float32|float64|int|int8|int16|int32|int64|rune|string|uint|uint8|uint16|uint32|uint64|uintptr)$/;
const GO_CONST = /^(?:true|false|nil|iota)$/;

function go(src) {
  let out = '', i = 0;
  const n = src.length;
  let afterDot = false; // the previous token was a "." selector
  while (i < n) {
    const rest = src.slice(i);
    let m;
    if ((m = /^\/\/[^\n]*/.exec(rest)) || (m = /^\/\*[\s\S]*?(?:\*\/|$)/.exec(rest))) {
      out += span('t-com', m[0]);
    } else if ((m = /^"(?:\\.|[^"\\\n])*"?/.exec(rest)) ||
               (m = /^'(?:\\.|[^'\\\n])*'?/.exec(rest)) ||
               (m = /^`[^`]*`?/.exec(rest))) {
      out += span('t-str', m[0]);
    } else if ((m = /^(?:0[xX][\da-fA-F_]+|\d[\d_]*(?:\.\d*)?(?:[eE][+-]?\d+)?|\.\d+)/.exec(rest))) {
      out += span('t-num', m[0]);
    } else if ((m = /^[A-Za-z_]\w*/.exec(rest))) {
      const w = m[0];
      const call = src[i + w.length] === '(';
      const cls = GO_KW.test(w) ? 't-kw'
        : GO_CONST.test(w) ? 't-num'
        : call ? 't-fn'
        : GO_TYPE.test(w) ? 't-cls'
        : afterDot && /^[A-Z]/.test(w) ? 't-cls'
        : 't-id';
      out += span(cls, w);
    } else if ((m = /^(?::=|\.\.\.|&&|\|\||<-|[-+*\/%&|^<>!=]=?)/.exec(rest))) {
      out += span('t-op', m[0]);
    } else {
      m = [src[i]];
      out += esc(m[0]);
    }
    if (m[0].trim()) afterDot = m[0] === '.';
    i += m[0].length;
  }
  return out;
}

// --- error marker ------------------------------------------------------------
// errorMark(ta) marks a compile error's line in an overlay editor: a tinted
// band across the line with a bar at the left edge (the host styles
// `.errband`), kept aligned as the textarea scrolls. The band is inserted
// just before the textarea, so it paints above the highlighted <pre> and
// below the (transparent) textarea text.
//
//   const mark = stylHi.errorMark(ta);
//   mark.show(line, col)  // 1-based; col may be 0/undefined
//   mark.clear()
//   mark.fromResult(r)    // show() for an error in this source, else clear()
//   mark.jump()           // focus the textarea with the caret at line:col
function errorMark(ta) {
  const band = document.createElement('div');
  band.className = 'errband';
  band.hidden = true;
  ta.parentElement.insertBefore(band, ta);
  let line = 0, col = 0;

  const place = () => {
    if (!line) return;
    const cs = getComputedStyle(ta);
    const lh = parseFloat(cs.lineHeight) || parseFloat(cs.fontSize) * 1.5;
    band.style.top = (parseFloat(cs.paddingTop) + (line - 1) * lh - ta.scrollTop) + 'px';
    band.style.height = lh + 'px';
  };
  ta.addEventListener('scroll', place);

  return {
    show(l, c) {
      line = l > 0 ? l : 0;
      col = c > 0 ? c : 0;
      band.hidden = !line;
      place();
    },
    clear() { line = 0; band.hidden = true; },
    // fromResult marks a compile result's error when it points into the
    // editor's own source (the WASM side compiles it as playground.styl; an
    // error inside another file has no line here). Returns whether it did.
    fromResult(r) {
      const here = r.error !== undefined && r.line > 0 &&
        (!r.file || r.file === '<input>' || /(^|\/)playground\.styl$/.test(r.file));
      if (here) this.show(r.line, r.col); else this.clear();
      return here;
    },
    jump() {
      if (!line) return;
      // Offset of line:col in the text; clamp to the line's end.
      const lines = ta.value.split('\n');
      let pos = 0;
      for (let i = 0; i < line - 1 && i < lines.length; i++) pos += lines[i].length + 1;
      const len = (lines[line - 1] || '').length;
      pos += Math.min(Math.max(col - 1, 0), len);
      ta.focus();
      ta.setSelectionRange(pos, pos);
      // Scroll the error line to about a third of the way down.
      const lh = parseFloat(getComputedStyle(ta).lineHeight) || 19.5;
      ta.scrollTop = Math.max(0, (line - 1) * lh - ta.clientHeight / 3);
      place();
    },
  };
}

const api = { styl, css, go, escape: esc, editor, errorMark };
if (typeof window !== 'undefined') window.stylHi = api;
if (typeof module !== 'undefined') module.exports = api;
})();
