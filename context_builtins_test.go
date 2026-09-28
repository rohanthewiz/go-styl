package styl_test

import (
	"strings"
	"testing"

	"github.com/rohanthewiz/go-styl"
)

// TestSelectorBuiltins covers selector() and selectors(). Expected values are
// reference stylus 0.64's output, apart from ^[N] partial references, which
// go-styl does not support.
func TestSelectorBuiltins(t *testing.T) {
	src := `.a, .b
  w0 selector()
  .c &:hover
    w1 selector()
    w2 selector('.x')
    w3 selector('.x', '.y')
    w4 selector('&:hover')
    w5 selector('.x' '.y')
    w6 length(selectors())
    w7 selectors()[1]
    .d
      w8 selectors()[2]
.top
  w0 selector()
  w1 selectors()[0]
v = selector()
.root
  x v
f()
  selector()
.fn .q
  z f()
m()
  w selector()
.x
  m()
`
	want := ".a,.b{w0:'.a,.b'}" +
		".c .a:hover,.c .b:hover{w1:'.c .a:hover,.c .b:hover';w2:'.x';" +
		"w3:'.c .a:hover .x .y,.c .b:hover .x .y';w4:'.c .a:hover:hover,.c .b:hover:hover';" +
		"w5:'.c .a:hover .x .y,.c .b:hover .x .y';w6:2;w7:'.c &:hover'}" +
		".c .a:hover .d,.c .b:hover .d{w8:'& .d'}" +
		".top{w0:'.top';w1:'.top'}" +
		".root{x:'&'}" +
		".fn .q{z:'.fn .q'}" +
		".x{w:'.x'}"
	if got := compileMin(t, src); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// TestSelectorExists: selector-exists() sees rules compiled before the call,
// including nested ones (which crash stylus 0.64's implementation).
func TestSelectorExists(t *testing.T) {
	src := `.a
  color #f00
  > .c
    color #00f
.b
  if selector-exists('.a')
    w1 1
  if selector-exists('.a>.c')
    w2 1
  if selector-exists('.zz')
    w3 1
`
	if got, want := compileMin(t, src), ".a{color:#f00}.a>.c{color:#00f}.b{w1:1;w2:1}"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestCurrentMedia: current-media() is the innermost @media query as
// written (” outside one); variables in it are resolved. (stylus 0.64 adds
// stray parentheses here; see biCurrentMedia.)
func TestCurrentMedia(t *testing.T) {
	src := `bp = 100px
@media screen and (max-width: bp)
  .m
    w current-media()
.nm
  w current-media()
.outer
  @media print
    w current-media()
@supports (display: grid)
  .s
    w current-media()
`
	want := "@media screen and (max-width:100px){.m{w:'@media screen and (max-width: 100px)'}}" +
		".nm{w:''}" +
		"@media print{.outer{w:'@media print'}}" +
		"@supports (display:grid){.s{w:''}}"
	if got := compileMin(t, src); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// TestDefineLookup: define() binds a computed name in the calling scope (or
// the root with global = true); lookup() reads one, null when undefined.
func TestDefineLookup(t *testing.T) {
	src := `define('dv', 10px)
col = 'x'
define(col, 1)
foo()
  define('inner', 5)
  inner
g()
  define('gl', 7, true)
  1
.a
  w dv lookup('dv') x
  n lookup('nope')
  i foo()
  j lookup('inner')
  k g() gl
`
	if got, want := compileMin(t, src), ".a{w:10px 10px 1;n:;i:5;j:;k:1 7}"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestAddProperty: add-property() appends to the enclosing rule, also from a
// function used in a declaration value (landing before that declaration),
// and from a mixin.
func TestAddProperty(t *testing.T) {
	src := `k()
  add-property('bar', 1)
  10
pm()
  add-property('q', 2)
.ap
  a 0
  width k()
  z 2
.ap2
  add-property('baz', 3px)
  y 1
.ap3
  pm()
`
	want := ".ap{a:0;bar:1;width:10;z:2}.ap2{baz:3px;y:1}.ap3{q:2}"
	if got := compileMin(t, src); got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
	if _, err := styl.Compile("add-property('x', 1)\n", styl.Options{}); err == nil {
		t.Error("add-property() outside a selector: want an error")
	}
}

// TestWarn: warn() reports through Options.Warn and compilation continues.
func TestWarn(t *testing.T) {
	var got []string
	css, err := styl.Compile("warn('careful')\n.a\n  color #f00\n", styl.Options{
		Warn: func(msg string) { got = append(got, msg) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "careful" {
		t.Errorf("warnings = %q, want [careful]", got)
	}
	if !strings.Contains(css, "color") {
		t.Errorf("compilation should continue, got %q", css)
	}
}

// TestUseUnsupported: use() (a JavaScript plugin loader) is a clear error.
func TestUseUnsupported(t *testing.T) {
	_, err := styl.Compile("use('plugin.js')\n", styl.Options{})
	if err == nil || !strings.Contains(err.Error(), "JavaScript plugin") {
		t.Fatalf("want the use() error, got %v", err)
	}
}

// TestPrefixClasses: +prefix-classes(p) prefixes class names in the rules of
// its block, but not in enclosing selectors (stylus 0.64 output).
func TestPrefixClasses(t *testing.T) {
	src := `+prefix-classes('foo-')
  .bar
    color #f00
    .baz &.q, #id .x
      color #00f
.outer
  +prefix-classes('p-')
    .in
      color #f00
    &.self
      color #f00
`
	want := ".foo-bar{color:#f00}" +
		".foo-baz .foo-bar.foo-q,.foo-bar #id .foo-x{color:#00f}" +
		".outer .p-in{color:#f00}" +
		".outer.p-self{color:#f00}"
	if got := compileMin(t, src); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	// A nested `+ li` is still an adjacent-sibling selector.
	if got, want := compileMin(t, ".a\n  + li\n    color #f00\n"), ".a+li{color:#f00}"; got != want {
		t.Errorf("sibling selector: got %q, want %q", got, want)
	}
}

// TestListMutation: push/unshift/pop/shift update the list variable they are
// given, as in Stylus, while returning go-styl's values (push returns the
// new list, so `l = push(l, x)` keeps working).
func TestListMutation(t *testing.T) {
	src := `l = 1 2 3
push(l, 4)
x = pop(l)
unshift(l, 0)
y = shift(l)
c = 1, 2
append(c, 3)
f()
  push(l, 7)
  1
.a
  w l
  v x y
  n length(push(l, 9)) l
  c c
  g f() l
  r = push(l, 5)
  s r
`
	want := ".a{w:1 2 3;v:4 0;n:4 1 2 3 9;c:1,2,3;g:1 1 2 3 9 7;s:1 2 3 9 7 5}"
	if got := compileMin(t, src); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
