package styl_test

import (
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	styl "github.com/rohanthewiz/go-styl"
)

const m14Sheet = `primary = #0af
pad = 2 * 8px
font-stack = Helvetica, Arial, sans-serif

.card
  padding: pad
  background: primary
  .card__title--big
    font-weight: bold
  &:hover
    box-shadow: 0 0 4px #000

#main
  a.btn[href$=".png"]
    color: primary

@media (min-width: 900px)
  .sidebar
    width: 300px

@keyframes spin
  from
    transform: rotate(0)
  to
    transform: rotate(360deg)

.ghost
  unused = 1

.pill
  &:not(.hidden)
    border-radius: 999px
`

// TestM14Extract covers styl.Extract: classes (nested, &-suffixed, inside
// @media and :not()), IDs, keyframes names, and root-scope variables — while
// attribute-selector strings, output-less rules, and rule-local variables are
// excluded.
func TestM14Extract(t *testing.T) {
	m, err := styl.Extract(m14Sheet, styl.Options{})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	wantClasses := []string{"btn", "card", "card__title--big", "hidden", "pill", "sidebar"}
	if !reflect.DeepEqual(m.Classes, wantClasses) {
		t.Errorf("Classes = %v, want %v", m.Classes, wantClasses)
	}
	if want := []string{"main"}; !reflect.DeepEqual(m.IDs, want) {
		t.Errorf("IDs = %v, want %v", m.IDs, want)
	}
	if want := []string{"spin"}; !reflect.DeepEqual(m.Keyframes, want) {
		t.Errorf("Keyframes = %v, want %v", m.Keyframes, want)
	}
	wantVars := []styl.Var{
		{Name: "font-stack", Value: "Helvetica, Arial, sans-serif"},
		{Name: "pad", Value: "16px"},
		{Name: "primary", Value: "#0af"},
	}
	if !reflect.DeepEqual(m.Vars, wantVars) {
		t.Errorf("Vars = %v, want %v", m.Vars, wantVars)
	}
	if m.Source != "" {
		t.Errorf("Source = %q, want empty for Extract from a string", m.Source)
	}
}

// TestM14ExtractCases: focused extraction behaviors, one sheet each.
func TestM14ExtractCases(t *testing.T) {
	cases := []struct {
		name string
		src  string
		opts styl.Options
		want styl.Manifest
	}{
		{
			"extend grafts extender selector",
			".btn\n  color blue\n.primary\n  @extend .btn",
			styl.Options{},
			styl.Manifest{Classes: []string{"btn", "primary"}},
		},
		{
			"placeholder emits only extenders",
			"$base\n  color blue\n.alert\n  @extend $base",
			styl.Options{},
			styl.Manifest{Classes: []string{"alert"}},
		},
		{
			"merge duplicates keeps all selectors",
			".a\n  color red\n.b\n  color red",
			styl.Options{MergeDuplicates: true},
			styl.Manifest{Classes: []string{"a", "b"}},
		},
		{
			"interpolated class from a global",
			".{theme}-badge\n  color red",
			styl.Options{Globals: map[string]any{"theme": "dark"}},
			styl.Manifest{
				Classes: []string{"dark-badge"},
				Vars:    []styl.Var{{Name: "theme", Value: "dark"}},
			},
		},
		{
			"custom properties deref to concrete values",
			"primary = #0af\na\n  color primary",
			styl.Options{CustomProperties: []string{"primary"}},
			styl.Manifest{Vars: []styl.Var{{Name: "primary", Value: "#0af"}}},
		},
		{
			"vendor-prefixed keyframes",
			"@-webkit-keyframes fade\n  from\n    opacity 0\n  to\n    opacity 1",
			styl.Options{},
			styl.Manifest{Keyframes: []string{"fade"}},
		},
		{
			"quoted keyframes name",
			"@keyframes \"slide in\"\n  from\n    left 0\n  to\n    left 10px",
			styl.Options{},
			styl.Manifest{Keyframes: []string{"slide in"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, err := styl.Extract(c.src, c.opts)
			if err != nil {
				t.Fatalf("Extract: %v", err)
			}
			m.Source = ""
			if !reflect.DeepEqual(*m, c.want) {
				t.Errorf("manifest = %+v, want %+v", *m, c.want)
			}
		})
	}
}

// TestM14ExtractFile: imported sheets contribute classes and root variables,
// and the input path is recorded as Source.
func TestM14ExtractFile(t *testing.T) {
	fsys := fstest.MapFS{
		"app.styl":    {Data: []byte("@import 'theme'\n.card\n  color primary\n")},
		"theme.styl":  {Data: []byte("primary = #0af\n.themed\n  color primary\n")},
		"unused.styl": {Data: []byte(".nope\n  color red\n")},
	}
	m, err := styl.ExtractFile("app.styl", styl.Options{FS: fsys})
	if err != nil {
		t.Fatalf("ExtractFile: %v", err)
	}
	if want := []string{"card", "themed"}; !reflect.DeepEqual(m.Classes, want) {
		t.Errorf("Classes = %v, want %v", m.Classes, want)
	}
	wantVars := []styl.Var{{Name: "primary", Value: "#0af"}}
	if !reflect.DeepEqual(m.Vars, wantVars) {
		t.Errorf("Vars = %v, want %v", m.Vars, wantVars)
	}
	if m.Source != "app.styl" {
		t.Errorf("Source = %q, want %q", m.Source, "app.styl")
	}
}

// TestM14GoSource: the generated file has the DO NOT EDIT header, one constant
// per name with group suffixes, and parses as valid Go.
func TestM14GoSource(t *testing.T) {
	m, err := styl.Extract(m14Sheet, styl.Options{})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	m.Source = "app.styl"
	src, err := m.GoSource("css")
	if err != nil {
		t.Fatalf("GoSource: %v", err)
	}
	got := string(src)

	for _, sub := range []string{
		"// Code generated by styl gen from app.styl; DO NOT EDIT.",
		"package css\n",
		`Card         = "card"`,
		`CardTitleBig = "card__title--big"`,
		`Btn          = "btn"`,
		`MainID = "main"`,
		`SpinAnim = "spin"`,
		`PrimaryVar   = "#0af"`,
		`PadVar       = "16px"`,
		`FontStackVar = "Helvetica, Arial, sans-serif"`,
	} {
		if !strings.Contains(got, sub) {
			t.Errorf("generated source missing %q\n%s", sub, got)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "css_gen.go", src, 0); err != nil {
		t.Errorf("generated source does not parse: %v", err)
	}
}

// TestM14GoSourceEdgeCases: naming, collisions, and package validation.
func TestM14GoSourceEdgeCases(t *testing.T) {
	t.Run("default package is css", func(t *testing.T) {
		src, err := (&styl.Manifest{Classes: []string{"a"}}).GoSource("")
		if err != nil {
			t.Fatalf("GoSource: %v", err)
		}
		if !strings.Contains(string(src), "package css\n") {
			t.Errorf("missing default package clause:\n%s", src)
		}
	})

	t.Run("empty manifest is valid Go", func(t *testing.T) {
		src, err := (&styl.Manifest{}).GoSource("css")
		if err != nil {
			t.Fatalf("GoSource: %v", err)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), "css_gen.go", src, 0); err != nil {
			t.Errorf("empty-manifest source does not parse: %v\n%s", err, src)
		}
	})

	t.Run("leading digit gets N prefix", func(t *testing.T) {
		src, err := (&styl.Manifest{Classes: []string{"3col"}}).GoSource("css")
		if err != nil {
			t.Fatalf("GoSource: %v", err)
		}
		if !strings.Contains(string(src), `N3col = "3col"`) {
			t.Errorf("missing N-prefixed constant:\n%s", src)
		}
	})

	t.Run("collision within a group errors", func(t *testing.T) {
		_, err := (&styl.Manifest{Classes: []string{"card-big", "cardBig"}}).GoSource("css")
		if err == nil || !strings.Contains(err.Error(), "CardBig") {
			t.Errorf("want collision error naming CardBig, got %v", err)
		}
	})

	t.Run("collision across groups errors", func(t *testing.T) {
		m := &styl.Manifest{
			Classes: []string{"primary-var"},
			Vars:    []styl.Var{{Name: "primary", Value: "#0af"}},
		}
		_, err := m.GoSource("css")
		if err == nil || !strings.Contains(err.Error(), "PrimaryVar") {
			t.Errorf("want collision error naming PrimaryVar, got %v", err)
		}
	})

	t.Run("unusable name errors", func(t *testing.T) {
		_, err := (&styl.Manifest{Classes: []string{"--"}}).GoSource("css")
		if err == nil || !strings.Contains(err.Error(), `"--"`) {
			t.Errorf("want unusable-name error, got %v", err)
		}
	})

	t.Run("invalid package name errors", func(t *testing.T) {
		_, err := (&styl.Manifest{}).GoSource("my-css")
		if err == nil || !strings.Contains(err.Error(), "package name") {
			t.Errorf("want invalid package error, got %v", err)
		}
	})

	t.Run("unicode class name", func(t *testing.T) {
		src, err := (&styl.Manifest{Classes: []string{"über-box"}}).GoSource("css")
		if err != nil {
			t.Fatalf("GoSource: %v", err)
		}
		if !strings.Contains(string(src), `ÜberBox = "über-box"`) {
			t.Errorf("missing unicode constant:\n%s", src)
		}
	})
}
