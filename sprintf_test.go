package styl_test

import (
	"strings"
	"testing"
)

// TestSprintfOperator covers Stylus's string `%` operator (N-028), which
// formats like s(): `"calc(100vh - %s)" % x`. Expected values match
// reference stylus 0.64.
func TestSprintfOperator(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"calc", "x = 50px\n.a\n  height \"calc(100vh - %s)\" % x", `.a{height:calc(100vh - 50px)}`},
		{"expression argument", "x = 50px\n.a\n  height \"calc(100vh - %s)\" % (x + 2px)", `.a{height:calc(100vh - 52px)}`},
		{"list spreads", `b "%s and %s" % (1px 2px)`, `.a{b:1px and 2px}`},
		{"idents", `c "%s-%s" % (a b)`, `.a{c:a-b}`},
		{"list variable spreads", "w = 1px 2px\n.a\n  b \"%s|%s\" % w", `.a{b:1px|2px}`},
		{"comma list variable spreads", "z = 1px, 2px\n.a\n  b \"%s|%s\" % z", `.a{b:1px|2px}`},
		{"string keeps quotes", `e "%s" % "str"`, `.a{e:"str"}`},
		{"single quotes", `g '%s,%s' % (1 2)`, `.a{g:1,2}`},
		{"no placeholder", `h "none" % 1px`, `.a{h:none}`},
		{"%d drops unit", `c "%d|%s" % (3px 4em)`, `.a{c:3|4em}`},
		{"%% is not an escape", `m "100%% %s" % 1px`, `.a{m:100%% 1px}`},
		{"color", `n "%s" % #fff`, `.a{n:#fff}`},
		{"binds like *", `e "%s" % 1px 3px`, `.a{e:1px 3px}`},
		{"null", `g "a%sb" % null`, `.a{g:ab}`},
		{"in a mixin", "m(w)\n  width \"calc(%s - 8px)\" % w\n.a\n  m(50%)", `.a{width:calc(50% - 8px)}`},
		{"number modulo still works", `z-index 7 % 3`, `.a{z-index:1}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := c.src
			if !strings.Contains(src, "\n") {
				src = ".a\n  " + src
			}
			if got := compileMin(t, src+"\n"); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
