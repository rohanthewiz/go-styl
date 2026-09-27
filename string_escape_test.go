package styl_test

import (
	"strings"
	"testing"
)

// TestStringEscapes covers backslash escapes in quoted strings (N-026): they
// pass through to the CSS verbatim, as in reference stylus 0.64, so that
// `content "\2022"` renders a bullet rather than the text "2022". The two
// escaped-quote cases are a ParseError in Stylus; go-styl keeps them as valid
// CSS instead.
func TestStringEscapes(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"unicode escape", `content "\2022"`, `.a{content:"\2022"}`},
		{"single-quoted", `content '\201C'`, `.a{content:'\201C'}`},
		{"icon font codepoint", `content "\f101"`, `.a{content:"\f101"}`},
		{"escaped backslash", `content "back\\slash"`, `.a{content:"back\\slash"}`},
		{"newline escape", `content "\a"`, `.a{content:"\a"}`},
		{"nested quotes", `content '"\2022"'`, `.a{content:'"\2022"'}`},
		{"several escapes", `font-family "\5FAE\8F6F\96C5\9ED1"`, `.a{font-family:"\5FAE\8F6F\96C5\9ED1"}`},
		{"unquote keeps escape", `content unquote("\2022")`, `.a{content:\2022}`},
		{"through a variable", "x = \"\\f101\"\n.a\n  content x", `.a{content:"\f101"}`},
		{"escaped single quote", `content 'it\'s'`, `.a{content:'it\'s'}`},
		{"escaped double quote", `content "say \"hi\""`, `.a{content:"say \"hi\""}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// A one-line case is a declaration inside `.a`; a multi-line one
			// is a full stylesheet.
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
