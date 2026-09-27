package styl_test

import "testing"

// TestInterpolateStringUnquoted covers interpolating a quoted string (N-038):
// it contributes its raw text, as in reference stylus 0.64. The url() and
// quoted-string cases use go-styl's interpolation extensions.
func TestInterpolateStringUnquoted(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"selector", "s = \"x\"\n.a-{s}\n  b 1", `.a-x{b:1}`},
		{"property name", "p = \"margin\"\n.a\n  {p}-top 1px", `.a{margin-top:1px}`},
		{"selector group", "s = \"x\"\nn = foo\n.b-{n}, .c-{s}\n  top 0", `.b-foo,.c-x{top:0}`},
		{"url path", "base = \"/img/\"\n.a\n  a url({base}x.png)", `.a{a:url(/img/x.png)}`},
		{"inside a quoted string", "s = \"x\"\n.a\n  content \"a {s} b\"", `.a{content:"a x b"}`},
		{"units kept", "bp = 768px\n@media (min-width: {bp})\n  .a\n    top 0", `@media (min-width:768px){.a{top:0}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := compileMin(t, c.src+"\n"); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
