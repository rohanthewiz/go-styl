package styl_test

import (
	"go/parser"
	"go/token"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	styl "github.com/rohanthewiz/go-styl"
)

const componentSheet = `accent ?= #0af

.card
  color: accent
  animation: fade 1s ease-in, "spin" 2s
  & :global(.htmx-request)
    opacity: .5
  .title, &:not(.hidden)
    margin: 0
  a[href$=".png"]
    color: red

#main .card
  width: 10px

@media (min-width: 900px)
  .card
    animation-name: fade

@keyframes fade
  from
    opacity: 0

@-webkit-keyframes spin
  to
    opacity: 1

.external
  animation-name: bounce
`

// suffixRe pulls the component hash out of a scoped name ("card_abcd2345").
var suffixRe = regexp.MustCompile(`^card(_[a-z2-7]{8})$`)

// TestComponentScopes covers the rename rules of styl.Component: classes
// (nested, in :not(), in @media) and keyframes names (plain, vendor-prefixed,
// quoted in animation values) get the component suffix; IDs, attribute
// strings, :global(...) contents, and references to keyframes defined
// elsewhere stay as written.
func TestComponentScopes(t *testing.T) {
	c, err := styl.Component(componentSheet, styl.Options{Pretty: true})
	if err != nil {
		t.Fatalf("Component: %v", err)
	}
	m := suffixRe.FindStringSubmatch(c.Names["card"])
	if m == nil {
		t.Fatalf(`Names["card"] = %q, want card_<8 base32 chars>`, c.Names["card"])
	}
	sfx := m[1]

	for _, local := range []string{"card", "title", "hidden", "external", "fade", "spin"} {
		if got, want := c.Names[local], local+sfx; got != want {
			t.Errorf("Names[%q] = %q, want %q", local, got, want)
		}
	}
	for _, global := range []string{"htmx-request", "main", "bounce"} {
		if s, ok := c.Names[global]; ok {
			t.Errorf("Names[%q] = %q, want no entry (not a local class/keyframes)", global, s)
		}
	}

	wantCSS := strings.NewReplacer("SFX", sfx).Replace(`.cardSFX {
	color: #0af;
	animation: fadeSFX 1s ease-in, "spinSFX" 2s;
}

.cardSFX .htmx-request {
	opacity: 0.5;
}

.cardSFX .titleSFX, .cardSFX:not(.hiddenSFX) {
	margin: 0;
}

.cardSFX a[href$=".png"] {
	color: red;
}

#main .cardSFX {
	width: 10px;
}

@media (min-width: 900px) {
	.cardSFX {
		animation-name: fadeSFX;
	}
}

@keyframes fadeSFX {
	from {
		opacity: 0;
	}
}

@-webkit-keyframes spinSFX {
	to {
		opacity: 1;
	}
}

.externalSFX {
	animation-name: bounce;
}`)
	if c.CSS != wantCSS {
		t.Errorf("CSS mismatch\n got:\n%s\nwant:\n%s", c.CSS, wantCSS)
	}

	if got, want := c.Class("card", "title", "u-flex"), "card"+sfx+" title"+sfx+" u-flex"; got != want {
		t.Errorf("Class = %q, want %q", got, want)
	}
}

// TestComponentHashStability pins what the suffix depends on: the source text
// only. Globals (per-request theming) and the filename must not move it, or
// build-time generated constants would drift from runtime CSS.
func TestComponentHashStability(t *testing.T) {
	src := ".card\n  color: accent\n"
	base, err := styl.Component(src, styl.Options{Globals: map[string]any{"accent": "red"}})
	if err != nil {
		t.Fatal(err)
	}
	themed, err := styl.Component(src, styl.Options{Globals: map[string]any{"accent": "blue"}, Filename: "x/card.styl"})
	if err != nil {
		t.Fatal(err)
	}
	if base.Names["card"] != themed.Names["card"] {
		t.Errorf("suffix changed with Globals/Filename: %q vs %q", base.Names["card"], themed.Names["card"])
	}
	other, err := styl.Component(src+".other\n  color: red\n", styl.Options{Globals: map[string]any{"accent": "red"}})
	if err != nil {
		t.Fatal(err)
	}
	if base.Names["card"] == other.Names["card"] {
		t.Errorf("different sources share suffix %q", base.Names["card"])
	}
}

// TestComponentUnitsNotRenamed guards the animation-value scan: a keyframes
// block named like a unit must not capture the unit of a duration.
func TestComponentUnitsNotRenamed(t *testing.T) {
	c, err := styl.Component("@keyframes s\n  to\n    opacity: 1\n.a\n  animation: s 1s -2s\n", styl.Options{})
	if err != nil {
		t.Fatal(err)
	}
	sfx := strings.TrimPrefix(c.Names["s"], "s")
	if want := "animation:s" + sfx + " 1s -2s"; !strings.Contains(c.CSS, want) {
		t.Errorf("CSS %q does not contain %q", c.CSS, want)
	}
}

// TestComponentExtendAndMerge checks that @extend grafts and merged duplicate
// rules — selectors living outside Rule.Selector — are scoped too.
func TestComponentExtendAndMerge(t *testing.T) {
	src := ".base\n  color: red\n.btn\n  @extend .base\n  margin: 0\n.x\n  margin: 0\n"
	c, err := styl.Component(src, styl.Options{MergeDuplicates: true})
	if err != nil {
		t.Fatal(err)
	}
	n := c.Names
	for _, want := range []string{
		"." + n["base"] + ",." + n["btn"] + "{color:red}",
		"." + n["btn"] + ",." + n["x"] + "{margin:0}",
	} {
		if !strings.Contains(c.CSS, want) {
			t.Errorf("CSS %q does not contain %q", c.CSS, want)
		}
	}
}

// TestComponentGoSource checks the scoped Manifest through the shared
// GoSource renderer: constants are named after local names and hold scoped
// values; :global classes keep their own name; the output parses as Go.
func TestComponentGoSource(t *testing.T) {
	fsys := fstest.MapFS{"ui/card.styl": {Data: []byte(componentSheet)}}
	c, err := styl.ComponentFile("ui/card.styl", styl.Options{FS: fsys})
	if err != nil {
		t.Fatalf("ComponentFile: %v", err)
	}
	src, err := c.Manifest.GoSource("cardcss")
	if err != nil {
		t.Fatalf("GoSource: %v", err)
	}
	// Collapse gofmt's column alignment so assertions don't depend on the
	// widths of neighbouring constant names.
	out := regexp.MustCompile(`[ \t]+`).ReplaceAllString(string(src), " ")
	if _, err := parser.ParseFile(token.NewFileSet(), "gen.go", src, 0); err != nil {
		t.Fatalf("generated source does not parse: %v\n%s", err, out)
	}
	for _, want := range []string{
		"from ui/card.styl",
		`Card = "` + c.Names["card"] + `" // card`,
		`HtmxRequest = "htmx-request"` + "\n",
		`FadeAnim = "` + c.Names["fade"] + `" // fade`,
		`MainID = "main"`,
		`AccentVar = "#0af" // accent`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("GoSource missing %q\n%s", want, out)
		}
	}
}
