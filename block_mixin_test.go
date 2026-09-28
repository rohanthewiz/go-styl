package styl_test

import (
	"strings"
	"testing"

	styl "github.com/rohanthewiz/go-styl"
)

// TestBlockMixin: a user mixin called as `+m(args)` with an indented body
// runs that body at its `{block}`, under the slot's selectors and @media,
// with variables resolved at the call site (stylus 0.64 output).
func TestBlockMixin(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"media wrapper", `c = red
mobile(w = 600px)
  @media (max-width: w)
    {block}
.btn
  color blue
  +mobile()
    color c
    .x
      width 1px
`, ".btn{color:blue}@media (max-width:600px){.btn{color:red}.btn .x{width:1px}}"},
		{"slot used twice, args", `wrap(cls)
  .{cls}
    margin 0
    {block}
    &:hover
      {block}
+wrap('card')
  color red
`, ".card{margin:0;color:red}.card:hover{color:red}"},
		// The passed body's own {block} is the enclosing mixin's block.
		{"nested block mixins", `inner()
  .i
    {block}
outer()
  .o
    +inner()
      padding 1px
      {block}
+outer()
  border 0
`, ".o .i{padding:1px;border:0}"},
		{"inside prefix-classes", `wrap(cls)
  .{cls}
    {block}
.p
  +prefix-classes('ui-')
    +wrap('k')
      top 0
`, ".p .ui-k{top:0}"},
		// Called without a block, the slot is empty (as in Stylus), so a
		// mixin can take an optional block.
		{"optional block", `m()
  color red
  .q
    {block}
.a
  m()
`, ".a{color:red}"},
		// A return inside the passed body ends the body only.
		{"return ends block", `m()
  .q
    {block}
    top 0
+m()
  width 1px
  return 5
  height 2px
`, ".q{width:1px;top:0}"},
		{"brace syntax", `m() {
  .z {
    {block}
  }
}
.b {
  +m() {
    color: red;
  }
}
`, ".b .z{color:red}"},
		// A user mixin named prefix-classes takes precedence over the
		// built-in, as for ordinary calls.
		{"user shadows built-in", `prefix-classes(p)
  .{p}
    {block}
+prefix-classes('x')
  top 0
`, ".x{top:0}"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := compileMin(t, c.src); got != c.want {
				t.Errorf("got  %q\nwant %q", got, c.want)
			}
		})
	}
}

// TestBlockSlotOutsideMixin: a {block} outside any mixin body is reported
// (Stylus silently drops it).
func TestBlockSlotOutsideMixin(t *testing.T) {
	_, err := styl.Compile(".a\n  color red\n  {block}\n", styl.Options{})
	if err == nil || !strings.Contains(err.Error(), "{block} is only valid inside a mixin body") {
		t.Fatalf("want the {block} error, got %v", err)
	}
}

// TestMigrateBlockMixin: migration expands a block mixin with the passed body
// nested at the slot.
func TestMigrateBlockMixin(t *testing.T) {
	res := migrate(t, `mobile()
  @media (max-width: 600px)
    {block}
.btn
  +mobile()
    color red
`)
	if !hasNote(res, "mixin", "expanded mobile()") {
		t.Errorf("missing mixin note: %+v", res.Notes)
	}
	want := "@media (max-width: 600px) {\n    color: red;"
	if !strings.Contains(res.CSS, want) {
		t.Errorf("want %q in\n%s", want, res.CSS)
	}
}
