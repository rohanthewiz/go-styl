package difftest

import (
	"regexp"
	"strings"
	"testing"
)

// normalize maps formatting-only variations in the two compilers' compressed
// output onto one canonical form, so the differential test compares semantics.
// Every rewrite is applied identically to both outputs, so equality is
// preserved; keep the rules conservative — anything erased here is invisible
// to the test forever. Quoted string contents are left untouched.
//
// Rules (all outside quoted strings):
//   - newlines removed (stylus keeps one after @charset/@import even compressed)
//   - ";}" → "}"
//   - spaces around the > and ~ combinators removed (stylus keeps them)
//   - "!important" gets exactly one leading space
//   - decimals get a leading zero (.5 → 0.5)
//   - named colors → hex (stylus rewrites them in compressed mode)
//   - hex colors → lowercase 6/8-digit long form (#FfF → #ffffff)
func normalize(css string) string {
	return mapOutsideStrings(strings.TrimSpace(css), func(s string) string {
		s = strings.ReplaceAll(s, "\n", "")
		s = strings.ReplaceAll(s, ";}", "}")
		s = strings.ReplaceAll(s, " > ", ">")
		s = strings.ReplaceAll(s, " ~ ", "~")
		s = collapseSiblingCombinator(s)
		s = reImportant.ReplaceAllString(s, " !important")
		s = reBareDecimal.ReplaceAllString(s, "${1}0.$2")
		s = reNamedColor.ReplaceAllStringFunc(s, func(m string) string {
			i := strings.IndexFunc(m, func(r rune) bool { return r != ':' && r != ',' && r != '(' && r != ' ' })
			return m[:i] + namedColors[m[i:]]
		})
		s = reHexColor.ReplaceAllStringFunc(s, longHex)
		return s
	})
}

var (
	reImportant = regexp.MustCompile(`\s*!important\b`)
	// A decimal with no integer part, in value position (after : , ( or space).
	reBareDecimal = regexp.MustCompile(`([:,( ])\.([0-9])`)
	// Hex tokens; longHex leaves lengths other than 3/4/6/8 alone. Also matches
	// all-hex-letter id selectors (#cafe) — mangled identically on both sides,
	// so comparisons still hold.
	reHexColor = regexp.MustCompile(`#[0-9a-fA-F]+\b`)
	// Named colors in value position only (after : , ( or space) — never after
	// . or # where the word would be a class/id selector.
	reNamedColor *regexp.Regexp
)

// namedColors maps CSS color keywords seen in output to 6-digit hex. Extend as
// the corpus grows; an unmapped name just shows up as a diff to classify.
var namedColors = map[string]string{
	"aqua": "#00ffff", "beige": "#f5f5dc", "black": "#000000", "blue": "#0000ff",
	"brown": "#a52a2a", "coral": "#ff7f50", "crimson": "#dc143c", "cyan": "#00ffff",
	"fuchsia": "#ff00ff", "gold": "#ffd700", "gray": "#808080", "green": "#008000",
	"grey": "#808080", "indigo": "#4b0082", "ivory": "#fffff0", "khaki": "#f0e68c",
	"lime": "#00ff00", "magenta": "#ff00ff", "maroon": "#800000", "navy": "#000080",
	"olive": "#808000", "orange": "#ffa500", "orchid": "#da70d6", "pink": "#ffc0cb",
	"plum": "#dda0dd", "purple": "#800080", "red": "#ff0000", "salmon": "#fa8072",
	"silver": "#c0c0c0", "tan": "#d2b48c", "teal": "#008080", "tomato": "#ff6347",
	"turquoise": "#40e0d0", "violet": "#ee82ee", "wheat": "#f5deb3", "white": "#ffffff",
	"yellow": "#ffff00",
}

func init() {
	names := make([]string, 0, len(namedColors))
	for n := range namedColors {
		names = append(names, n)
	}
	reNamedColor = regexp.MustCompile(`([:,( ])(` + strings.Join(names, "|") + `)\b`)
}

// collapseSiblingCombinator removes the spaces around a ' + ' outside any
// parentheses. At paren depth 0, + is the adjacent-sibling combinator (stylus
// keeps the spaces even compressed); inside parens it is arithmetic — think
// calc(100% + 10px) — where spacing is significant and must survive.
func collapseSiblingCombinator(s string) string {
	var out strings.Builder
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ' ':
			if depth == 0 && i+2 < len(s) && s[i+1] == '+' && s[i+2] == ' ' {
				out.WriteByte('+')
				i += 2
				continue
			}
		}
		out.WriteByte(s[i])
	}
	return out.String()
}

// longHex canonicalizes a #rgb/#rgba/#rrggbb/#rrggbbaa token to lowercase long
// form; other lengths pass through unchanged.
func longHex(tok string) string {
	tok = strings.ToLower(tok)
	digits := tok[1:]
	switch len(digits) {
	case 3, 4:
		var b strings.Builder
		b.WriteByte('#')
		for i := 0; i < len(digits); i++ {
			b.WriteByte(digits[i])
			b.WriteByte(digits[i])
		}
		return b.String()
	case 6, 8:
		return tok
	}
	return tok
}

// mapOutsideStrings applies f to the chunks of css that are not inside single-
// or double-quoted strings, leaving quoted contents (and their quotes) as-is.
func mapOutsideStrings(css string, f func(string) string) string {
	var out strings.Builder
	chunkStart := 0
	for i := 0; i < len(css); i++ {
		q := css[i]
		if q != '"' && q != '\'' {
			continue
		}
		out.WriteString(f(css[chunkStart:i]))
		j := i + 1
		for j < len(css) && css[j] != q {
			if css[j] == '\\' {
				j++
			}
			j++
		}
		if j < len(css) {
			j++ // include the closing quote
		}
		out.WriteString(css[i:j])
		chunkStart = j
		i = j - 1
	}
	out.WriteString(f(css[chunkStart:]))
	return out.String()
}

func TestNormalize(t *testing.T) {
	cases := []struct{ a, b string }{
		{`a{color:white}`, `a{color:#fff}`},
		{`a{color:#fff !important}`, "a{color:white!important}"},
		{`@charset "utf-8";` + "\n" + `a{b:c}`, `@charset "utf-8";a{b:c}`},
		{`nav > .brand{x:y}`, `nav>.brand{x:y}`},
		{`a{c:rgba(0,0,0,.5)}`, `a{c:rgba(0,0,0,0.5)}`},
		{`a{m:0 .25em}`, `a{m:0 0.25em}`},
		{`a{b:c;}`, `a{b:c}`},
		{`a{c:#ABC}`, `a{c:#aabbcc}`},
		{`nav + .sidebar{x:y}`, `nav+.sidebar{x:y}`},
	}
	for _, c := range cases {
		if na, nb := normalize(c.a), normalize(c.b); na != nb {
			t.Errorf("normalize(%q)=%q but normalize(%q)=%q — want equal", c.a, na, c.b, nb)
		}
	}
	// Quoted contents must survive untouched.
	in := `a{content:"white > .5\"x"}`
	if got := normalize(in); got != in {
		t.Errorf("quoted content changed: %q -> %q", in, got)
	}
	// Arithmetic + inside parens must keep its spaces.
	in = `a{width:calc(100% + 10px)}`
	if got := normalize(in); got != in {
		t.Errorf("calc + collapsed: %q -> %q", in, got)
	}
	// whitesmoke must not be rewritten via the 'white' entry.
	if got := normalize(`a{c:whitesmoke}`); got != `a{c:whitesmoke}` {
		t.Errorf("whitesmoke mangled: %q", got)
	}
}
