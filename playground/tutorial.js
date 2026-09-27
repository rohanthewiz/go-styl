// tutorial.js — the interactive go-styl tutorial (the "Tutorial" tab of the
// playground). Lesson data + the tutorial UI.
//
// Each lesson: { id, title, nav, prose, code, task, check, solution, opts }
//   prose    — array of segments: raw-HTML strings, or {code, lang} blocks
//              that render as highlighted snippets (lang: styl|css|go|plain)
//   code     — starter source for the lesson editor
//   task     — one-line exercise shown above the editor
//   check    — (css, flat) => bool; flat is css with whitespace collapsed to
//              single spaces. Passing marks the lesson complete.
//   solution — source the "solution" button loads
//   opts     — extra compile options (used by the theming lesson)
//
// The data half is plain JS so ../playground/verify_tutorial.js can compile
// every snippet with the real compiler in CI/dev.
(() => {
'use strict';

const code = (src, lang) => ({ code: src, lang: lang || 'styl' });

const LESSONS = [

// ---------------------------------------------------------------------------
{
  id: 'hello',
  title: 'Hello, Stylus',
  nav: 'Hello, Stylus',
  prose: [
`<h2>Hello, Stylus</h2>
<p><a href="https://stylus-lang.com/" target="_blank" rel="noopener">Stylus</a> is a CSS preprocessor: you write terse, expressive source and it compiles to plain CSS. <strong>go-styl</strong> is a pure-Go compiler for it — the same compiler running in this page as WebAssembly, recompiling on every keystroke.</p>
<p>The signature feature is the <em>indentation syntax</em>: braces, colons, and semicolons are all optional. A selector on its own line starts a rule; indented <code>property value</code> lines fill it:</p>`,
    code(`h1
  font-size 2rem
  color #333`),
`<p>compiles to:</p>`,
    code(`h1 {
  font-size: 2rem;
  color: #333;
}`, 'css'),
`<p>You can still write <code>property: value;</code> — the punctuation is simply optional. (A whole CSS-like brace syntax also works; lesson 17 covers it.)</p>
<div class="tip">Everything in this tutorial is live: edit the code on the right and the CSS pane updates instantly. Errors appear below the editor with a <code>line:col</code> position.</div>
<p>The editor on the right has a small stylesheet started for you. Complete the task above it to earn a check mark — progress is saved in your browser.</p>`,
  ],
  code: `// your first stylesheet — edit me!
body
  font 16px/1.6 sans-serif
  color #333

h1
  font-size 2rem
`,
  task: 'Give the h1 a color of tomato.',
  check: (css, flat) => /h1 \{[^}]*color: tomato/.test(flat),
  solution: `body
  font 16px/1.6 sans-serif
  color #333

h1
  font-size 2rem
  color tomato
`,
},

// ---------------------------------------------------------------------------
{
  id: 'nesting',
  title: 'Nesting & the parent reference',
  nav: 'Nesting & parent',
  prose: [
`<h2>Nesting &amp; the parent reference</h2>
<p>Rules nest. An inner selector compiles to a <em>descendant</em> selector of its parent — the structure of your source mirrors the structure of your markup:</p>`,
    code(`.card
  padding 16px

  .title
    font-weight 600`),
    code(`.card { padding: 16px; }
.card .title { font-weight: 600; }`, 'css'),
`<p>The <code>&amp;</code> character references the <em>parent selector itself</em> rather than nesting under it. That is how you attach pseudo-classes, sibling combinators, and modifier classes:</p>`,
    code(`.card
  &:hover          // -> .card:hover
    box-shadow 0 4px 12px rgba(0,0,0,0.15)
  &.active         // -> .card.active   (no space!)
  & + .card        // -> .card + .card`),
`<p>As a convenience, a bare pseudo-class line (<code>:hover</code>) attaches to the parent too — go-styl treats it like <code>&amp;:hover</code>.</p>
<div class="tip">Nesting is powerful but deep chains produce heavy selectors. Two or three levels is usually plenty.</div>`,
  ],
  code: `.card
  padding 16px
  border-radius 8px
  background white

  .title
    font-weight 600
    font-size 1.1rem

  &:hover
    box-shadow 0 4px 12px rgba(0,0,0,0.15)
`,
  task: 'Add an &.active variant on .card that sets border-color to #e91e63.',
  check: (css, flat) => /\.card\.active \{[^}]*border-color: #e91e63/.test(flat),
  solution: `.card
  padding 16px
  border-radius 8px
  background white

  .title
    font-weight 600
    font-size 1.1rem

  &:hover
    box-shadow 0 4px 12px rgba(0,0,0,0.15)

  &.active
    border-color #e91e63
`,
},

// ---------------------------------------------------------------------------
{
  id: 'variables',
  title: 'Variables',
  nav: 'Variables',
  prose: [
`<h2>Variables</h2>
<p>Assign with <code>=</code>; use anywhere a value goes. Variables are resolved at <em>compile time</em> — the generated CSS contains only final values:</p>`,
    code(`primary = #e91e63
pad = 12px

.btn
  background primary
  padding pad`),
`<p>Two refinements:</p>
<ul>
<li><code>?=</code> assigns <em>only if the variable is not already set</em>. It is how a file declares an overridable default — an earlier assignment, an <code>@import</code>-ing file, or a Go-side <code>Options.Globals</code> entry (lesson 19) wins over it.</li>
<li>Scoping is <em>lexical</em>: a variable assigned inside a rule shadows the outer one for that block only.</li>
</ul>`,
    code(`accent = #06c
.sidebar
  accent = #f60      // shadows only inside .sidebar
  border-color accent
.footer
  border-color accent  // still #06c`),
`<div class="tip">Variable names may contain <code>-</code> and <code>_</code>. A <code>$</code> prefix (<code>$primary</code>) is allowed and common in Stylus code you will find in the wild.</div>`,
  ],
  code: `primary = #e91e63
pad = 12px
accent ?= #06c    // ?= assigns only when not already set

.btn
  background primary
  padding pad
  color white

.badge
  border 1px solid accent
  padding pad
`,
  task: 'Add a radius = 6px variable and use it for border-radius on both .btn and .badge.',
  check: (css, flat) => (flat.match(/border-radius: 6px/g) || []).length >= 2,
  solution: `primary = #e91e63
pad = 12px
radius = 6px
accent ?= #06c

.btn
  background primary
  padding pad
  color white
  border-radius radius

.badge
  border 1px solid accent
  padding pad
  border-radius radius
`,
},

// ---------------------------------------------------------------------------
{
  id: 'arithmetic',
  title: 'Arithmetic & units',
  nav: 'Arithmetic',
  prose: [
`<h2>Arithmetic &amp; units</h2>
<p>Numbers carry their units through arithmetic — <code>+ - * / %</code> and <code>**</code> (power) all work, and comparisons (<code>&lt; &gt; == !=</code>) return booleans for control flow later:</p>`,
    code(`base = 8px

.box
  padding base * 2      // 16px
  width 100% - 25%      // 75%
  z-index 2 ** 3        // 8`),
`<h3>Two gotchas worth learning early</h3>
<p><strong>1. Division in property values.</strong> CSS itself uses <code>/</code> literally (<code>font: 16px/1.5</code>), so in a property value a bare <code>/</code> is kept as-is. Wrap the expression in parentheses to force division:</p>`,
    code(`.box
  font 16px/1.5 sans-serif   // literal: font: 16px/1.5
  margin (base / 2)          // computed: 4px`),
`<p><strong>2. Whitespace-sensitive minus.</strong> <code>margin 10px -5px</code> is a two-item list (that is valid CSS!), while <code>10px - 5px</code> subtracts. The space on <em>both</em> sides makes it an operator.</p>
<div class="tip">Mixed units follow the left operand: <code>10px + 1</code> is <code>11px</code>. Percentages combine with percentages.</div>`,
  ],
  code: `base = 8px

.box
  padding base * 2
  margin (base / 2)
  width 100% - 25%
  z-index 2 ** 3

.list-item
  // a list, not subtraction (note the missing space):
  margin 10px -5px
`,
  task: 'Add a gutter variable equal to base * 3 and use it for gap on a .grid rule.',
  check: (css, flat) => /\.grid \{[^}]*gap: 24px/.test(flat),
  solution: `base = 8px
gutter = base * 3

.box
  padding base * 2
  margin (base / 2)
  width 100% - 25%
  z-index 2 ** 3

.grid
  display grid
  gap gutter
`,
},

// ---------------------------------------------------------------------------
{
  id: 'colors',
  title: 'Color functions',
  nav: 'Colors',
  prose: [
`<h2>Color functions</h2>
<p>Colors are first-class values: hex literals, <code>rgb()/rgba()/hsl()/hsla()</code>, and CSS named colors all parse into a color you can compute with. go-styl ships the Stylus color library:</p>
<ul>
<li><code>lighten(c, 20%)</code> / <code>darken(c, 20%)</code> — move lightness</li>
<li><code>saturate(c, 20%)</code> / <code>desaturate(c, 20%)</code></li>
<li><code>mix(a, b, 50%)</code> — blend two colors</li>
<li><code>tint(c, 60%)</code> / <code>shade(c, 60%)</code> — mix with white / black</li>
<li><code>alpha(c, 0.5)</code> — set opacity; <code>rgba(c, 0.5)</code> works too</li>
<li><code>spin(c, 180deg)</code>, <code>complement(c)</code>, <code>invert(c)</code>, <code>hue(c)</code> …</li>
</ul>`,
    code(`brand = #3b82f6

.button
  background brand
  &:hover
    background darken(brand, 15%)

.button-outline
  color brand
  border 1px solid rgba(brand, 0.4)`),
`<p>Because everything computes at compile time, a one-line change to <code>brand</code> re-derives every hover shade and tint in the sheet.</p>
<div class="tip">Color arithmetic also works: <code>#222 + #111</code> adds channelwise. It is occasionally handy, but the named functions read better.</div>`,
  ],
  code: `brand = #3b82f6

.button
  background brand
  color white
  &:hover
    background darken(brand, 15%)

.tint-box
  background tint(brand, 70%)

.overlay
  background rgba(brand, 0.25)
`,
  task: 'Add a .link rule whose color is lighten(brand, 20%).',
  check: (css, flat) => {
    const m = /\.link \{[^}]*color: (#[0-9a-fA-F]+|rgba?\([^)]*\))/.exec(flat);
    return !!m && !/#3b82f6$/i.test(m[1]);
  },
  solution: `brand = #3b82f6

.button
  background brand
  color white
  &:hover
    background darken(brand, 15%)

.tint-box
  background tint(brand, 70%)

.overlay
  background rgba(brand, 0.25)

.link
  color lighten(brand, 20%)
`,
},

// ---------------------------------------------------------------------------
{
  id: 'lists',
  title: 'Lists',
  nav: 'Lists',
  prose: [
`<h2>Lists</h2>
<p>Values separated by spaces or commas form <em>lists</em>. A space list emits as-is (perfect for shorthands); a comma list keeps its commas (perfect for <code>box-shadow</code>, <code>font-family</code>, transitions):</p>`,
    code(`sizes = 4px 8px 16px 32px
shadows = 0 1px 2px rgba(0,0,0,0.2), 0 8px 24px rgba(0,0,0,0.08)

.card
  box-shadow shadows   // both shadows, comma kept`),
`<p>List built-ins let you compute with them:</p>
<ul>
<li><code>length(list)</code> — item count</li>
<li><code>last(list)</code> — final item</li>
<li><code>index(list, item)</code> — zero-based position (or null)</li>
<li><code>push(list, item)</code> / <code>unshift(list, item)</code> — grow a list</li>
<li><code>join(sep, list)</code> — stringify with a separator</li>
</ul>
<p>Lists really shine with <code>for … in</code> loops (lesson 11), where each item stamps out a rule.</p>`,
  ],
  code: `sizes = 4px 8px 16px 32px
shadows = 0 1px 2px rgba(0,0,0,0.2), 0 8px 24px rgba(0,0,0,0.08)

.card
  box-shadow shadows
  padding last(sizes)      // 32px

.meta
  // length() counts items
  z-index length(sizes)    // 4
`,
  task: 'Add a .chip rule with margin set to the whole sizes list (space lists emit as-is).',
  check: (css, flat) => /\.chip \{[^}]*margin: 4px 8px 16px 32px/.test(flat),
  solution: `sizes = 4px 8px 16px 32px
shadows = 0 1px 2px rgba(0,0,0,0.2), 0 8px 24px rgba(0,0,0,0.08)

.card
  box-shadow shadows
  padding last(sizes)

.meta
  z-index length(sizes)

.chip
  margin sizes
`,
},

// ---------------------------------------------------------------------------
{
  id: 'mixins',
  title: 'Mixins',
  nav: 'Mixins',
  prose: [
`<h2>Mixins</h2>
<p>A mixin is a reusable block of declarations. Define it like a rule with a parameter list; <em>call</em> it inside another rule and its declarations are stamped in:</p>`,
    code(`button(bg = #555, fg = white)
  display inline-block
  padding 8px 16px
  border-radius 4px
  background bg
  color fg

.primary
  button(#e91e63)     // fg defaults to white

.ghost
  button(transparent, #e91e63)`),
    code(`.primary {
  display: inline-block;
  padding: 8px 16px;
  border-radius: 4px;
  background: #e91e63;
  color: white;
}`, 'css'),
`<p>Parameters take <em>defaults</em> (<code>bg = #555</code>), so callers pass only what differs. A default can even reference an earlier parameter — <code>size(w, h = w)</code> makes squares from one argument.</p>
<p>Mixins can nest selectors too: a mixin body may contain <code>&amp;:hover</code> blocks, media queries, or child rules, and they land relative to the caller.</p>
<div class="tip">Mixin vs. function: a <strong>mixin</strong> emits declarations into the calling rule; a <strong>function</strong> (next lesson) returns a value you place into a single declaration. Same definition syntax — usage decides.</div>`,
  ],
  code: `button(bg = #555, fg = white)
  display inline-block
  padding 8px 16px
  border-radius 4px
  border none
  background bg
  color fg

.primary
  button(#e91e63)

.ghost
  button(transparent, #e91e63)
`,
  task: 'Add a .success rule that calls button() with background #22c55e.',
  check: (css, flat) => /\.success \{[^}]*background: #22c55e/.test(flat),
  solution: `button(bg = #555, fg = white)
  display inline-block
  padding 8px 16px
  border-radius 4px
  border none
  background bg
  color fg

.primary
  button(#e91e63)

.ghost
  button(transparent, #e91e63)

.success
  button(#22c55e)
`,
},

// ---------------------------------------------------------------------------
{
  id: 'functions',
  title: 'Functions',
  nav: 'Functions',
  prose: [
`<h2>Functions</h2>
<p>The same <code>name(params)</code> definition, used <em>in value position</em>, is a function. The last expression is returned implicitly (or use <code>return</code>):</p>`,
    code(`gutter(n)
  n * 8px            // implicit return

.grid
  gap gutter(2)      // -> gap: 16px`),
`<p>Functions compose with everything: arithmetic, conditionals, built-ins. A classic is a px→em converter:</p>`,
    code(`em(px, base = 16px)
  return unit(px / base, 'em')

.lead
  font-size em(20px)   // -> 1.25em`),
`<p><code>unit(n, 'em')</code> is a built-in that swaps the unit of a number — dividing px by px left a unitless <code>1.25</code>, and <code>unit()</code> stamps <code>em</code> on it.</p>
<div class="tip">Unknown function names simply pass through to the CSS — <code>translateX(10px)</code>, <code>var(--x)</code>, <code>url(...)</code> come out untouched. Only names you define (or built-ins) are evaluated.</div>`,
  ],
  code: `gutter(n)
  n * 8px

em(px, base = 16px)
  return unit(px / base, 'em')

.grid
  display grid
  gap gutter(2)

.lead
  font-size em(20px)
`,
  task: 'Write half(n) returning n / 2, then add .half { width: half(300px) } — expect 150px.',
  check: (css, flat) => /\.half \{[^}]*width: 150px/.test(flat),
  solution: `gutter(n)
  n * 8px

em(px, base = 16px)
  return unit(px / base, 'em')

half(n)
  n / 2

.grid
  display grid
  gap gutter(2)

.lead
  font-size em(20px)

.half
  width half(300px)
`,
},

// ---------------------------------------------------------------------------
{
  id: 'advanced-mixins',
  title: 'Rest arguments & friends',
  nav: 'Rest args',
  prose: [
`<h2>Rest arguments &amp; friends</h2>
<p>A trailing <code>args...</code> parameter collects <em>all remaining arguments</em> — commas included. That is the idiom for shorthand-property helpers and vendor prefixing:</p>`,
    code(`shadow(args...)
  -webkit-box-shadow args
  box-shadow args

.modal
  shadow(0 2px 8px rgba(0,0,0,0.3), 0 0 1px rgba(0,0,0,0.4))`),
    code(`.modal {
  -webkit-box-shadow: 0 2px 8px rgba(0,0,0,0.3), 0 0 1px rgba(0,0,0,0.4);
  box-shadow: 0 2px 8px rgba(0,0,0,0.3), 0 0 1px rgba(0,0,0,0.4);
}`, 'css'),
`<p>Other parameter tricks:</p>
<ul>
<li><strong>Defaults referencing earlier params</strong>: <code>size(w, h = w)</code> — one argument makes a square.</li>
<li><strong><code>arguments</code></strong>: inside any mixin/function, the implicit <code>arguments</code> holds exactly what the caller wrote.</li>
</ul>
<div class="tip">go-styl also supports Stylus <em>transparent mixins</em>: an unknown property-looking call like <code>border-radius 4px</code> inside a mixin body can forward to a mixin of that name if you define one.</div>`,
  ],
  code: `shadow(args...)
  -webkit-box-shadow args
  box-shadow args

size(w, h = w)
  width w
  height h

.modal
  shadow(0 2px 8px rgba(0,0,0,0.3), 0 0 1px rgba(0,0,0,0.4))
  size(400px, 300px)

.avatar
  size(40px)
`,
  task: 'Add a .thumb rule that is 96px square, via size().',
  check: (css, flat) => /\.thumb \{[^}]*width: 96px;[^}]*height: 96px/.test(flat),
  solution: `shadow(args...)
  -webkit-box-shadow args
  box-shadow args

size(w, h = w)
  width w
  height h

.modal
  shadow(0 2px 8px rgba(0,0,0,0.3), 0 0 1px rgba(0,0,0,0.4))
  size(400px, 300px)

.avatar
  size(40px)

.thumb
  size(96px)
`,
},

// ---------------------------------------------------------------------------
{
  id: 'conditionals',
  title: 'Conditionals',
  nav: 'Conditionals',
  prose: [
`<h2>Conditionals</h2>
<p><code>if</code> / <code>else if</code> / <code>else</code> work at compile time, both inside functions and directly inside rules. <code>unless</code> is sugar for <code>if not</code>. Operators read like English: <code>and</code>, <code>or</code>, <code>not</code>, <code>==</code>, <code>is</code>, <code>isnt</code>.</p>
<p>A favorite pattern — pick readable text for any background using the <code>dark()</code> / <code>light()</code> built-ins (when a function branches, use an explicit <code>return</code>):</p>`,
    code(`readable(bg)
  if dark(bg)
    return white
  return #222

.hero
  background #1e293b
  color readable(#1e293b)   // -> white`),
`<p>Conditionals also guard whole declaration blocks inside a rule:</p>`,
    code(`compact = false

.row
  padding 12px
  if compact
    padding 4px
    font-size 0.9rem`),
`<div class="tip">Truthiness: <code>false</code>, <code>null</code>, and <code>0</code> are falsy; everything else (including strings and colors) is truthy.</div>`,
  ],
  code: `readable(bg)
  if dark(bg)
    return white
  return #222

.hero
  background #1e293b
  color readable(#1e293b)

.note
  background #fef9c3
  color readable(#fef9c3)

loud ?= false
`,
  task: 'Add an .alert rule that uses "unless loud" to apply border 1px dashed #999.',
  check: (css, flat) => /\.alert \{[^}]*border: 1px dashed #999/.test(flat),
  solution: `readable(bg)
  if dark(bg)
    return white
  return #222

.hero
  background #1e293b
  color readable(#1e293b)

.note
  background #fef9c3
  color readable(#fef9c3)

loud ?= false

.alert
  unless loud
    border 1px dashed #999
`,
},

// ---------------------------------------------------------------------------
{
  id: 'loops',
  title: 'Loops & ranges',
  nav: 'Loops',
  prose: [
`<h2>Loops &amp; ranges</h2>
<p><code>for item in list</code> repeats its block once per item. Combined with interpolation in the selector (<code>{i}</code> — full story next lesson), it stamps out utility classes:</p>`,
    code(`for i in 1..4
  .m-{i}
    margin i * 4px`),
    code(`.m-1 { margin: 4px; }
.m-2 { margin: 8px; }
.m-3 { margin: 12px; }
.m-4 { margin: 16px; }`, 'css'),
`<p><code>1..4</code> is an inclusive range; <code>1...4</code> excludes the end. Any list works as the source, including names:</p>`,
    code(`for c in coral teal rebeccapurple
  .text-{c}
    color c`),
`<p>Loops nest, and the loop variable participates in any expression — arithmetic, function calls, conditionals.</p>
<div class="tip">Generated selectors are a compile-time expansion: the CSS output contains no loop, just the finished rules. Keep an eye on the output pane size when generating utilities.</div>`,
  ],
  code: `for i in 1..4
  .m-{i}
    margin i * 4px

for c in coral teal rebeccapurple
  .text-{c}
    color c
`,
  task: 'Generate .p-1 through .p-4 padding utilities in steps of 4px (like the .m- classes).',
  check: (css, flat) => /\.p-1 \{[^}]*padding: 4px/.test(flat) && /\.p-3 \{[^}]*padding: 12px/.test(flat),
  solution: `for i in 1..4
  .m-{i}
    margin i * 4px

for i in 1..4
  .p-{i}
    padding i * 4px

for c in coral teal rebeccapurple
  .text-{c}
    color c
`,
},

// ---------------------------------------------------------------------------
{
  id: 'interpolation',
  title: 'Interpolation',
  nav: 'Interpolation',
  prose: [
`<h2>Interpolation</h2>
<p><code>{expr}</code> evaluates an expression and splices the result into places where a bare variable would not be understood — selectors, property <em>names</em>, strings, and media queries:</p>`,
    code(`side = left
n = 4

.pull
  margin-{side} auto      // property name

.col
  width (100% / n)        // 25%

@media (min-width: {320px * 2})   // arithmetic in a query
  .col
    width 50%`),
`<p>Where interpolation is <em>required</em> (these come from real-world stumbles, straight from the go-styl README):</p>
<ul>
<li>Inside <code>calc()</code> and <code>url()</code> bare variables are left alone — write <code>calc(100% - {gutter})</code>.</li>
<li>Arithmetic in a media query needs it: <code>@media (min-width: {bp * 2})</code>. (A plain variable is fine: <code>@media (min-width: bp)</code>.)</li>
</ul>`,
    code(`gutter = 24px
.main
  width calc(100% - {gutter})`),
`<div class="tip">In selectors, interpolation pairs with loops (previous lesson) and with mixins that build BEM-ish names: <code>.icon-{name}</code>.</div>`,
  ],
  code: `side = left
gutter = 24px

.pull
  margin-{side} auto

.main
  width calc(100% - {gutter})

@media (min-width: {320px * 2})
  .main
    max-width 60ch
`,
  task: 'Add a .border-{side} rule (compiling to .border-left) that sets border-{side} 2px solid #333.',
  check: (css, flat) => /\.border-left \{[^}]*border-left: 2px solid #333/.test(flat),
  solution: `side = left
gutter = 24px

.pull
  margin-{side} auto

.main
  width calc(100% - {gutter})

@media (min-width: {320px * 2})
  .main
    max-width 60ch

.border-{side}
  border-{side} 2px solid #333
`,
},

// ---------------------------------------------------------------------------
{
  id: 'builtins',
  title: 'Built-ins tour',
  nav: 'Built-ins',
  prose: [
`<h2>Built-ins tour</h2>
<p>Beyond colors, go-styl implements the Stylus built-in library across four families. A sampler:</p>
<h3>Math</h3>
<ul>
<li><code>round(7.4px)</code> / <code>ceil</code> / <code>floor</code> / <code>abs</code></li>
<li><code>min(a, b)</code> / <code>max(a, b)</code></li>
<li><code>pow(2, 10)</code>, <code>sqrt(x)</code></li>
<li><code>percentage(0.42)</code> → <code>42%</code></li>
</ul>
<h3>String</h3>
<ul>
<li><code>quote(x)</code> / <code>unquote(s)</code> — add or strip quotes (unquote is the classic escape hatch for odd CSS)</li>
<li><code>uppercase(s)</code>, <code>lowercase(s)</code>, <code>substr(s, 0, 3)</code>, <code>replace(pattern, replacement, s)</code>, <code>split(sep, s)</code></li>
<li><code>s('%s %s', a, b)</code> — sprintf-style formatting</li>
</ul>
<h3>List</h3>
<p><code>length</code>, <code>last</code>, <code>index</code>, <code>push</code>, <code>join</code> — lesson 6.</p>
<h3>Type &amp; unit</h3>
<ul>
<li><code>typeof(x)</code> — 'unit', 'color', 'string', 'boolean', …</li>
<li><code>unit(x)</code> — read a number's unit; <code>unit(x, 'rem')</code> — replace it</li>
<li><code>light(c)</code> / <code>dark(c)</code> — lesson 10's readable-text trick</li>
</ul>
<div class="tip">Anything go-styl does not recognize passes through as literal CSS — new platform functions like <code>clamp()</code> and <code>color-mix()</code> keep working. But <code>min()</code>/<code>max()</code> <em>are</em> built-ins, so they compute at compile time; to emit a literal CSS <code>min()</code>, wrap it: <code>unquote('min(8px, 1rem)')</code>.</div>`,
  ],
  code: `w = 0.42

.stats
  width percentage(w)
  margin round(7.4px)
  z-index max(3, 7)

.label
  content quote(hello)
  letter-spacing unit(2, 'px')

.debug
  // typeof() in action — check the output
  content typeof(12px)
`,
  task: 'Use percentage() to give .half a width computed from 1 / 2 (expect 50%).',
  check: (css, flat) => /\.half \{[^}]*width: 50%/.test(flat),
  solution: `w = 0.42

.stats
  width percentage(w)
  margin round(7.4px)
  z-index max(3, 7)

.label
  content quote(hello)
  letter-spacing unit(2, 'px')

.half
  width percentage(1 / 2)
`,
},

// ---------------------------------------------------------------------------
{
  id: 'extend',
  title: '@extend & placeholders',
  nav: '@extend',
  prose: [
`<h2>@extend &amp; placeholders</h2>
<p>Where a mixin <em>copies</em> declarations to each caller, <code>@extend</code> <em>merges selectors</em>: the extending selector is appended to the extended rule, so the declarations exist once:</p>`,
    code(`$reset-list
  margin 0
  padding 0
  list-style none

.menu
  @extend $reset-list
  display flex

.crumbs
  @extend $reset-list`),
    code(`.menu, .crumbs {
  margin: 0;
  padding: 0;
  list-style: none;
}
.menu { display: flex; }`, 'css'),
`<p><code>$reset-list</code> is a <em>placeholder</em> selector: it exists only to be extended and never appears in the output on its own. You can also <code>@extend .real-class</code> — the extender joins that rule's selector list.</p>
<p><strong>Mixin or extend?</strong> Extend produces smaller CSS (one shared rule) but couples selectors together and cannot take parameters. Mixins are parameterized and self-contained but repeat declarations. Utility resets → extend; anything configurable → mixin.</p>`,
  ],
  code: `$reset-list
  margin 0
  padding 0
  list-style none

.menu
  @extend $reset-list
  display flex
  gap 12px

.breadcrumbs
  @extend $reset-list
  font-size 0.9rem
`,
  task: 'Add a .tags rule that also extends $reset-list.',
  check: (css, flat) => /\.menu,\s*\.breadcrumbs,\s*\.tags \{/.test(flat) || /\.tags[^{]*\{[^}]*list-style: none/.test(flat),
  solution: `$reset-list
  margin 0
  padding 0
  list-style none

.menu
  @extend $reset-list
  display flex
  gap 12px

.breadcrumbs
  @extend $reset-list
  font-size 0.9rem

.tags
  @extend $reset-list
`,
},

// ---------------------------------------------------------------------------
{
  id: 'media',
  title: 'Media queries & at-rules',
  nav: '@media & co.',
  prose: [
`<h2>Media queries &amp; at-rules</h2>
<p><code>@media</code> nests <em>inside</em> rules and bubbles out — you keep all of a component's behavior in one place, and the compiler hoists the query around it:</p>`,
    code(`bp = 768px

.sidebar
  width 280px
  @media (max-width: bp)
    display none`),
    code(`.sidebar { width: 280px; }
@media (max-width: 768px) {
  .sidebar { display: none; }
}`, 'css'),
`<p>Variables work directly in query values (<code>max-width: bp</code>); arithmetic needs interpolation (<code>{bp * 2}</code> — lesson 12). <code>@supports</code> behaves the same way, and leaf at-rules (<code>@charset</code>, <code>@namespace</code>, …) pass through verbatim.</p>`,
    code(`@supports (display: grid)
  .layout
    display grid`),
`<div class="tip">Define your breakpoints once as variables (or in an imported theme partial) and every query stays in sync.</div>`,
  ],
  code: `bp = 768px

.sidebar
  width 280px
  @media (max-width: bp)
    display none

@supports (display: grid)
  .layout
    display grid
    grid-template-columns 240px 1fr
`,
  task: 'Add a .content rule with a nested @media (min-width: {bp * 2}) block setting max-width 60ch.',
  check: (css, flat) => /@media \(min-width: 1536px\)/.test(flat) && /max-width: 60ch/.test(flat),
  solution: `bp = 768px

.sidebar
  width 280px
  @media (max-width: bp)
    display none

@supports (display: grid)
  .layout
    display grid
    grid-template-columns 240px 1fr

.content
  @media (min-width: {bp * 2})
    max-width 60ch
`,
},

// ---------------------------------------------------------------------------
{
  id: 'keyframes',
  title: 'Keyframes & fonts',
  nav: '@keyframes',
  prose: [
`<h2>Keyframes &amp; fonts</h2>
<p><code>@keyframes</code> uses the same indentation style — each step (<code>from</code>/<code>to</code> or percentages) is a nested block:</p>`,
    code(`@keyframes pulse
  0%
    opacity 1
  50%
    opacity 0.35
  100%
    opacity 1

.live-dot
  animation pulse 2s ease-in-out infinite`),
`<p>Variables and functions work inside steps like anywhere else — compute your keyframe values from theme tokens.</p>
<p><code>@font-face</code> is a plain at-rule block:</p>`,
    code(`@font-face
  font-family 'Inter'
  src url('/fonts/inter.woff2') format('woff2')
  font-display swap`),
`<div class="tip">Inside <code>url(...)</code> everything is literal (no variable evaluation) — use interpolation if you need a computed path: <code>url('/fonts/{name}.woff2')</code>.</div>`,
  ],
  code: `@keyframes pulse
  0%
    opacity 1
  50%
    opacity 0.35
  100%
    opacity 1

.live-dot
  animation pulse 2s ease-in-out infinite

@font-face
  font-family 'Inter'
  src url('/fonts/inter.woff2') format('woff2')
  font-display swap
`,
  task: 'Add a fade-in keyframes (from opacity 0, to opacity 1) and animate .toast with it.',
  check: (css, flat) => /@keyframes fade-in/.test(flat) && /\.toast \{[^}]*animation:[^;}]*fade-in/.test(flat),
  solution: `@keyframes pulse
  0%
    opacity 1
  50%
    opacity 0.35
  100%
    opacity 1

@keyframes fade-in
  from
    opacity 0
  to
    opacity 1

.live-dot
  animation pulse 2s ease-in-out infinite

.toast
  animation fade-in 0.3s ease-out

@font-face
  font-family 'Inter'
  src url('/fonts/inter.woff2') format('woff2')
  font-display swap
`,
},

// ---------------------------------------------------------------------------
{
  id: 'braces',
  title: 'Brace syntax',
  nav: 'Brace syntax',
  prose: [
`<h2>Brace syntax</h2>
<p>Everything you have learned also works in a CSS-like syntax with braces and semicolons — handy when pasting existing CSS, or when a team prefers explicit delimiters:</p>`,
    code(`.btn {
  padding: 8px 16px;
  border-radius: 4px;

  &:hover { background: #eee; }
}`),
`<p>Variables, mixins, loops, <code>@media</code> — all identical. The syntax is detected <em>per file</em>: a sheet is either indentation-style or brace-style, so pick one per file (an <code>@import</code>-ed partial can use the other style).</p>
<p>One brace-syntax limitation to know (from the go-styl README): a stand-alone <code>{expr}</code> interpolation in <em>value</em> position is not supported — write the bare variable instead (<code>width x</code>, not <code>width {x}</code>). Selector and property-name interpolation work fine.</p>
<div class="tip">Migrating a plain <code>.css</code> file? Rename it <code>.styl</code>, and it compiles as-is — then start deleting braces at your leisure.</div>`,
  ],
  code: `muted = #667085

.btn {
  padding: 8px 16px;
  border-radius: 4px;

  &:hover { background: #eee; }
}

.hint {
  color: muted;
}
`,
  task: 'Add a .btn.primary rule in brace syntax with background: #e91e63;',
  check: (css, flat) => /\.btn\.primary \{[^}]*background: #e91e63/.test(flat),
  solution: `muted = #667085

.btn {
  padding: 8px 16px;
  border-radius: 4px;

  &:hover { background: #eee; }

  &.primary {
    background: #e91e63;
    color: white;
  }
}

.hint {
  color: muted;
}
`,
},

// ---------------------------------------------------------------------------
{
  id: 'imports',
  title: '@import',
  nav: '@import',
  prose: [
`<h2>@import</h2>
<p><code>@import "file"</code> with a <code>.styl</code> target <em>inlines</em> that file at the import site — its variables and mixins become available, which is how you structure a real project: a <code>_theme.styl</code> partial with tokens and helpers, imported everywhere.</p>
<p>This playground bundles the repo's <code>examples/</code> folder as its filesystem, so you can import from it right here:</p>`,
    code(`@import "imports/_theme"   // primary, danger, radius + button()

.btn-primary
  button(primary)`),
`<p>Two other import forms pass through untouched: <code>.css</code> targets and <code>url(...)</code> imports become literal CSS <code>@import</code> statements, hoisted to the top of the output where CSS requires them.</p>
<p>On the Go side the same mechanics work against any directory — or an <code>embed.FS</code>, so your <code>.styl</code> sources ship inside the binary:</p>`,
    code(`//go:embed styles/*.styl
var styles embed.FS

css, err := styl.CompileFile("styles/app.styl",
    styl.Options{FS: styles})`, 'go'),
`<div class="tip">The leading underscore in <code>_theme.styl</code> is just a convention marking a partial; go-styl does not require it. The import path may omit the extension.</div>`,
  ],
  code: `// this import resolves inside the bundled examples/ folder
@import "imports/_theme"    // defines primary, danger, radius + button(bg)

.btn-primary
  button(primary)

@import "reset.css"          // .css imports pass through, hoisted to the top
`,
  task: 'Add a .btn-danger rule that calls button(danger) — danger comes from the imported theme.',
  check: (css, flat) => /\.btn-danger \{[^}]*background: #ed4014/.test(flat),
  solution: `@import "imports/_theme"

.btn-primary
  button(primary)

.btn-danger
  button(danger)

@import "reset.css"
`,
},

// ---------------------------------------------------------------------------
{
  id: 'theming',
  title: 'Runtime theming (go-styl)',
  nav: 'Runtime theming',
  prose: [
`<h2>Runtime theming <span class="ext">go-styl extension</span></h2>
<p>go-styl compiles in microseconds, in-process — fast enough to compile a stylesheet <em>per tenant, per user, per request</em>. Two options make that useful:</p>
<h3>Options.Globals</h3>
<p>Seeds Go values as root-scope variables before the sheet runs. In the sheet, declare overridable defaults with <code>?=</code>:</p>`,
    code(`brand ?= #06c      // a Global override wins
radius ?= 4px

.chip
  background brand
  border-radius radius`),
    code(`css, _ := styl.Compile(src, styl.Options{
    Globals: map[string]any{
        "brand":  "#7c3aed",
        "radius": "10px",
    },
})`, 'go'),
`<h3>Options.CustomProperties</h3>
<p>Lists tokens to expose as CSS custom properties. The output gains a <code>:root</code> block, and <em>direct references</em> compile to <code>var(--name)</code> — so the shipped CSS can be re-themed in the browser (dark mode, user themes) with no recompile. Computed uses (like <code>darken(brand, 20%)</code>) still use the compile-time value, because a function cannot run on a <code>var()</code>.</p>
<p><strong>This lesson's editor compiles with both options set</strong> — <code>brand: #7c3aed</code>, <code>radius: 10px</code>, both listed as custom properties. Watch the output: a <code>:root</code> block appears, and direct references become <code>var(--brand)</code>.</p>
<div class="tip">In the Playground tab, the "globals &amp; custom properties" panel under the editor does the same thing interactively: <code>name = value</code> per line, <code>--name</code> to expose one.</div>`,
  ],
  opts: {
    globals: { brand: '#7c3aed', radius: '10px' },
    customProperties: ['brand', 'radius'],
  },
  code: `// compiled with Options.Globals{brand: #7c3aed, radius: 10px}
// and Options.CustomProperties[brand, radius]
brand ?= #06c
radius ?= 4px

.chip
  background brand
  border-radius radius
  border 1px solid darken(brand, 20%)   // computed -> compile-time value
`,
  task: 'Add a .pill rule using brand for its color — it should compile to color: var(--brand).',
  check: (css, flat) => /\.pill \{[^}]*color: var\(--brand\)/.test(flat),
  solution: `brand ?= #06c
radius ?= 4px

.chip
  background brand
  border-radius radius
  border 1px solid darken(brand, 20%)

.pill
  color var(--brand)   // or just: color brand
`,
},

// ---------------------------------------------------------------------------
{
  id: 'next',
  title: 'Where to next',
  nav: 'Where to next',
  prose: [
`<h2>Where to next</h2>
<p>You have covered the language: nesting, variables, arithmetic, colors, lists, mixins, functions, control flow, interpolation, <code>@extend</code>, at-rules, both syntaxes, imports, and go-styl's runtime theming. The editor holds a small showcase combining most of it — tinker freely.</p>
<p>Beyond the browser, the Go side of go-styl offers:</p>
<ul>
<li><strong>Library</strong> — <code>styl.Compile / CompileFile / Build</code>, source maps, <code>embed.FS</code> sources.</li>
<li><strong>HTTP middleware</strong> — serve <code>.styl</code> compiled on the fly with caching + ETags: <code>stylhttp</code> for <code>net/http</code>, or rweb's <code>middleware/stylus</code>.</li>
<li><strong>Typed class names</strong> — <code>styl gen</code> emits Go constants for every class/ID/keyframes name, so a typo in markup is a compile error.</li>
<li><strong>Critical CSS</strong> — <code>styl.Prune</code> + <code>UsedFromHTML</code> compile only the rules a rendered page uses; <code>stylcrit</code> caches it per layout for middleware.</li>
<li><strong>CLI</strong> — <code>go run github.com/rohanthewiz/go-styl/cmd/styl input.styl</code>, with <code>-D</code> globals, <code>-cssvar</code>, <code>-sourcemap</code>, <code>-compress</code>.</li>
</ul>
<p>All of it is documented in the <a href="https://github.com/rohanthewiz/go-styl#readme" target="_blank" rel="noopener">README</a>. The bundled examples (the picker in the Playground tab) are runnable feature tours, too.</p>
<p>Building the HTML in Go too? <a href="https://rohanthewiz.github.io/element/#tutorial" target="_blank" rel="noopener">element's tutorial</a> covers its zero-dependency HTML builder — the natural companion to go-styl's typed class names.</p>
<div class="tip">Found a divergence from reference Stylus? The repo's <code>difftest/</code> harness scores go-styl against the Node compiler — issues and PRs welcome.</div>`,
  ],
  code: `// a little of everything — tinker away
bp = 640px
brand = #7c3aed
pad = 12px

button(bg = brand)
  display inline-block
  padding (pad / 2) pad
  border-radius 6px
  background bg
  color white
  &:hover
    background darken(bg, 12%)

nav.top
  display flex
  gap pad
  padding pad

  .logo
    font-weight 700
    color brand

  a
    button(#5b21b6)

  @media (max-width: bp)
    flex-direction column

for i in 1..3
  .stack-{i}
    margin-top i * 8px
`,
  task: 'No task here — you made it. Edit freely, or head to the Playground tab.',
  check: null,
  solution: null,
},
];

// ---------------------------------------------------------------------------
// Tutorial UI
// ---------------------------------------------------------------------------
function init() {
  const $ = id => document.getElementById(id);
  const doc = $('tut-doc'), navEl = $('tut-navlist');
  const ta = $('tsrc'), hlCode = $('tsrcHl');
  const outEl = $('tout'), errEl = $('terr');
  const taskEl = $('tut-task'), statEl = $('tut-check');
  const posEl = $('tut-pos');

  const store = {
    read(k, dflt) { try { return localStorage.getItem(k) ?? dflt; } catch (_) { return dflt; } },
    write(k, v) { try { localStorage.setItem(k, v); } catch (_) {} },
    del(k) { try { localStorage.removeItem(k); } catch (_) {} },
  };

  let done;
  try { done = new Set(JSON.parse(store.read('go-styl-tut-done', '[]'))); }
  catch (_) { done = new Set(); }
  let cur = Math.min(LESSONS.length - 1,
    Math.max(0, parseInt(store.read('go-styl-tut-cur', '0'), 10) || 0));
  let lastCSS = '';

  const hlOn = () => !document.body.classList.contains('nohl');
  const repaint = stylHi.editor(ta, hlCode, hlOn);

  function saveDone() { store.write('go-styl-tut-done', JSON.stringify([...done])); }

  function block(seg) {
    if (typeof seg === 'string') return seg;
    const lang = seg.lang || 'styl';
    const body = lang === 'styl' ? stylHi.styl(seg.code)
               : lang === 'css' ? stylHi.css(seg.code)
               : stylHi.escape(seg.code);
    return '<pre class="snip lang-' + lang + '"><code>' + body + '</code></pre>';
  }

  function renderNav() {
    navEl.innerHTML = '';
    LESSONS.forEach((l, i) => {
      const li = document.createElement('li');
      if (i === cur) li.className = 'cur';
      li.innerHTML = '<span class="n">' + (i + 1) + '</span>' + stylHi.escape(l.nav) +
        (done.has(l.id) ? '<span class="tick">✓</span>' : '');
      li.addEventListener('click', () => open(i));
      navEl.appendChild(li);
    });
    posEl.textContent = (cur + 1) + ' / ' + LESSONS.length;
    $('tut-prev').disabled = cur === 0;
    $('tut-next').disabled = cur === LESSONS.length - 1;
    $('tut-solution').style.display = LESSONS[cur].solution ? '' : 'none';
  }

  function open(i) {
    cur = i;
    store.write('go-styl-tut-cur', String(i));
    const l = LESSONS[i];
    doc.innerHTML = l.prose.map(block).join('');
    doc.scrollTop = 0;
    ta.value = store.read('go-styl-tut-draft-' + l.id, null) ?? l.code;
    taskEl.textContent = l.task || '';
    if (!l.check && !done.has(l.id)) { done.add(l.id); saveDone(); }
    renderNav();
    repaint();
    compile();
  }

  function renderOut() {
    outEl.innerHTML = '';
    if (hlOn()) outEl.innerHTML = stylHi.css(lastCSS);
    else outEl.textContent = lastCSS;
  }

  // Compiles are async (the compiler lives in a web worker — see runner.js);
  // compileSeq drops superseded results, and a lesson switch mid-flight
  // makes the old result stale.
  let compileSeq = 0;
  function compile() {
    if (!window.stylRun) return;
    const l = LESSONS[cur];
    const opts = Object.assign({ pretty: true }, l.opts || {});
    const seq = ++compileSeq;
    stylRun.compile(ta.value, opts).then(r => {
      if (seq !== compileSeq || LESSONS[cur] !== l) return;
      if (r.error !== undefined) {
        errEl.textContent = r.error;
        errEl.style.display = 'block';
        outEl.style.opacity = '0.45';
      } else {
        errEl.style.display = 'none';
        outEl.style.opacity = '';
        lastCSS = r.css;
        renderOut();
        if (l.check) {
          const flat = r.css.replace(/\s+/g, ' ');
          if (l.check(r.css, flat)) {
            if (!done.has(l.id)) { done.add(l.id); saveDone(); renderNav(); }
            statEl.textContent = '✓ task complete';
            statEl.className = 'stat ok';
            return;
          }
        }
      }
      if (l.check) {
        statEl.textContent = done.has(l.id) ? '✓ solved earlier' : '○ not yet';
        statEl.className = done.has(l.id) ? 'stat ok' : 'stat';
      } else {
        statEl.textContent = '';
        statEl.className = 'stat';
      }
    });
  }

  let timer = 0;
  ta.addEventListener('input', () => {
    store.write('go-styl-tut-draft-' + LESSONS[cur].id, ta.value);
    clearTimeout(timer);
    timer = setTimeout(compile, 120);
  });
  // Tab inserts two spaces, like the playground editor.
  ta.addEventListener('keydown', e => {
    if (e.key !== 'Tab') return;
    e.preventDefault();
    const { selectionStart: s, selectionEnd: t, value } = ta;
    ta.value = value.slice(0, s) + '  ' + value.slice(t);
    ta.selectionStart = ta.selectionEnd = s + 2;
    ta.dispatchEvent(new Event('input'));
  });

  $('tut-prev').addEventListener('click', () => cur > 0 && open(cur - 1));
  $('tut-next').addEventListener('click', () => cur < LESSONS.length - 1 && open(cur + 1));
  $('tut-reset').addEventListener('click', () => {
    store.del('go-styl-tut-draft-' + LESSONS[cur].id);
    ta.value = LESSONS[cur].code;
    repaint();
    compile();
  });
  $('tut-solution').addEventListener('click', () => {
    const l = LESSONS[cur];
    if (!l.solution) return;
    ta.value = l.solution;
    store.write('go-styl-tut-draft-' + l.id, l.solution);
    repaint();
    compile();
  });
  $('tut-toplay').addEventListener('click', () => {
    if (window.playHooks) window.playHooks.openInPlayground(ta.value);
  });

  open(cur);
  return { compile, repaint: () => { repaint(); renderOut(); } };
}

const api = { LESSONS, init };
if (typeof window !== 'undefined') window.gsTutorial = api;
if (typeof module !== 'undefined') module.exports = api;
})();
