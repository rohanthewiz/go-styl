package styl_test

import (
	"reflect"
	"strings"
	"testing"

	styl "github.com/rohanthewiz/go-styl"
)

const m15Sheet = `primary = #0af

.card
  background: primary
  .card__title
    font-weight: bold
  &:hover
    box-shadow: 0 0 4px #000

.unused-widget
  color: red

#main
  padding: 8px

#gone
  padding: 9px

a.btn
  color: primary

.pill:not(.hidden)
  border-radius: 999px

[data-theme="dark"]
  color: #eee

@media (min-width: 900px)
  .sidebar
    width: 300px
  .unused-widget
    width: 100px

@font-face
  font-family: 'Body'
  src: url('body.woff2')

.spinner
  animation: spin 2s linear infinite

.unused-anim
  animation: vanish 1s

@keyframes spin
  from
    transform: rotate(0)
  to
    transform: rotate(360deg)

@keyframes vanish
  to
    opacity: 0
`

var m15Used = styl.Used{
	Classes: []string{"btn", "card", "card__title", "pill", "sidebar", "spinner"},
	IDs:     []string{"main"},
}

// TestM15Prune covers the flagship path: used classes/IDs survive (nested,
// &-suffixed, inside @media), unused ones are dropped, selectors that require
// nothing (@font-face, attribute-only) always survive, :not() arguments are
// not required, and @keyframes live or die with the animations that reference
// them.
func TestM15Prune(t *testing.T) {
	out, err := styl.Prune(m15Sheet, m15Used, styl.Options{Pretty: true})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	for _, want := range []string{
		".card {", ".card .card__title {", ".card:hover {", "#main {",
		"a.btn {", ".pill:not(.hidden) {", `[data-theme="dark"] {`,
		"@media (min-width: 900px)", ".sidebar {",
		"@font-face {", ".spinner {", "@keyframes spin {",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	for _, gone := range []string{".unused-widget", "#gone", ".unused-anim", "@keyframes vanish", "vanish"} {
		if strings.Contains(out, gone) {
			t.Errorf("output still contains %q:\n%s", gone, out)
		}
	}
}

// TestM15PruneCases: focused pruning behaviors, one sheet each.
func TestM15PruneCases(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		used    styl.Used
		opts    styl.Options
		want    []string
		notWant []string
	}{
		{
			name: "multi-selector rule keeps only matchable selectors",
			src:  ".a, .b, .c\n  color: red\n",
			used: styl.Used{Classes: []string{"a", "c"}},
			want: []string{".a, .c {"}, notWant: []string{".b"},
		},
		{
			name: "compound selector requires every class",
			src:  ".a.b\n  color: red\n.a\n  color: blue\n",
			used: styl.Used{Classes: []string{"a"}},
			want: []string{".a {"}, notWant: []string{".a.b"},
		},
		{
			name: "nil axis is not filtered",
			src:  "#only-id\n  color: red\n.some-class\n  color: blue\n",
			used: styl.Used{Classes: []string{"some-class"}}, // IDs nil
			want: []string{"#only-id {", ".some-class {"},
		},
		{
			name: "empty non-nil axis prunes all",
			src:  "#only-id\n  color: red\n",
			used: styl.Used{IDs: []string{}},
			want: []string{}, notWant: []string{"#only-id"},
		},
		{
			name: "tag selectors pruned only when Tags is set",
			src:  "table\n  width: 100%\np\n  margin: 0\n",
			used: styl.Used{Tags: []string{"p"}},
			want: []string{"p {"}, notWant: []string{"table"},
		},
		{
			name: "tag requirement inside compound selector",
			src:  "table.grid\n  width: 100%\n",
			used: styl.Used{Classes: []string{"grid"}, Tags: []string{"p"}},
			want: []string{}, notWant: []string{"table.grid"},
		},
		{
			name: "descendant combinators require every step",
			src:  ".nav > .item\n  color: red\n.nav .gone\n  color: blue\n",
			used: styl.Used{Classes: []string{"nav", "item"}},
			want: []string{".nav > .item {"}, notWant: []string{".gone"},
		},
		{
			name: "pseudo-classes and pseudo-elements are not tags",
			src:  ".x:hover::before\n  content: ''\n",
			used: styl.Used{Classes: []string{"x"}, Tags: []string{"div"}},
			want: []string{".x:hover::before {"},
		},
		{
			name: ":is arguments are not required",
			src:  ".x:is(.gone, .also-gone)\n  color: red\n",
			used: styl.Used{Classes: []string{"x"}},
			want: []string{".x:is(.gone, .also-gone) {"},
		},
		{
			name: "extend graft pruned independently of host",
			src:  ".host\n  color: red\n.gone-extender\n  @extend .host\n",
			used: styl.Used{Classes: []string{"host"}},
			want: []string{".host {"}, notWant: []string{".gone-extender"},
		},
		{
			name: "extender survives a pruned host",
			src:  ".gone-host\n  color: red\n.keeper\n  @extend .gone-host\n",
			used: styl.Used{Classes: []string{"keeper"}},
			want: []string{".keeper {"}, notWant: []string{".gone-host"},
		},
		{
			name: "merged duplicates pruned per selector",
			src:  ".a\n  color: red\n.gone\n  color: red\n.c\n  color: red\n",
			used: styl.Used{Classes: []string{"a", "c"}},
			opts: styl.Options{MergeDuplicates: true},
			want: []string{".a, .c {"}, notWant: []string{".gone"},
		},
		{
			name: "at-rule emptied by pruning disappears",
			src:  "@media print\n  .gone\n    display: none\n.kept\n  color: red\n",
			used: styl.Used{Classes: []string{"kept"}},
			want: []string{".kept {"}, notWant: []string{"@media print"},
		},
		{
			name: "quoted and vendor keyframes tracked",
			src: ".x\n  animation: \"fade out\" 1s\n  -webkit-animation: legacy 1s\n" +
				"@keyframes \"fade out\"\n  to\n    opacity: 0\n" +
				"@-webkit-keyframes legacy\n  to\n    opacity: 0\n" +
				"@keyframes dead\n  to\n    opacity: 1\n",
			used:    styl.Used{Classes: []string{"x"}},
			want:    []string{`@keyframes "fade out" {`, "@-webkit-keyframes legacy {"},
			notWant: []string{"@keyframes dead"},
		},
		{
			name: "animation-name longhand counts as a reference",
			src:  ".x\n  animation-name: fade\n@keyframes fade\n  to\n    opacity: 0\n",
			used: styl.Used{Classes: []string{"x"}},
			want: []string{"@keyframes fade {"},
		},
		{
			name: "keyframes referenced only by pruned rules are dropped",
			src:  ".gone\n  animation: fade 1s\n@keyframes fade\n  to\n    opacity: 0\n",
			used: styl.Used{Classes: []string{"x"}},
			want: []string{}, notWant: []string{"@keyframes fade"},
		},
		{
			name: "keyframes inside kept media survive, dead media drops",
			src: ".x\n  animation: fade 1s\n" +
				"@media (prefers-reduced-motion: no-preference)\n  @keyframes fade\n    to\n      opacity: 0\n" +
				"@media print\n  @keyframes dead\n    to\n      opacity: 1\n",
			used: styl.Used{Classes: []string{"x"}},
			want: []string{"@keyframes fade {"}, notWant: []string{"@media print", "dead"},
		},
		{
			name: "root custom properties block always survives",
			src:  "primary = #0af\n.gone\n  color: primary\n",
			used: styl.Used{Classes: []string{}},
			opts: styl.Options{CustomProperties: []string{"primary"}},
			want: []string{":root {", "--primary: #0af;"}, notWant: []string{".gone"},
		},
		{
			name: "attribute selector strings are not class requirements",
			src:  `a[href$=".png"]` + "\n  color: red\n",
			used: styl.Used{Classes: []string{}},
			want: []string{`a[href$=".png"] {`},
		},
		{
			name: "raw at-rule lines pass through",
			src:  "@charset \"utf-8\"\n.gone\n  color: red\n",
			used: styl.Used{Classes: []string{}},
			want: []string{`@charset "utf-8";`}, notWant: []string{".gone"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.Pretty = true
			out, err := styl.Prune(tc.src, tc.used, tc.opts)
			if err != nil {
				t.Fatalf("Prune: %v", err)
			}
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in:\n%s", w, out)
				}
			}
			for _, nw := range tc.notWant {
				if strings.Contains(out, nw) {
					t.Errorf("unexpected %q in:\n%s", nw, out)
				}
			}
		})
	}
}

// TestM15PruneIsSubsetOfCompile: pruning with everything marked used is a
// no-op, byte for byte, in both pretty and compressed modes.
func TestM15PruneNoOpWhenAllUsed(t *testing.T) {
	m, err := styl.Extract(m15Sheet, styl.Options{})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	used := styl.Used{Classes: m.Classes, IDs: m.IDs} // Tags nil
	for _, pretty := range []bool{true, false} {
		full, err := styl.Compile(m15Sheet, styl.Options{Pretty: pretty})
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		pruned, err := styl.Prune(m15Sheet, used, styl.Options{Pretty: pretty})
		if err != nil {
			t.Fatalf("Prune: %v", err)
		}
		if pruned != full {
			t.Errorf("pretty=%v: pruned output differs from full compile\n--- full:\n%s\n--- pruned:\n%s", pretty, full, pruned)
		}
	}
}

// TestM15UsedFromHTML covers the HTML scanner: tags, multi-class attributes,
// IDs, quoted/unquoted values, comments, self-closing tags, and case
// handling. All slices come back non-nil, sorted, and duplicate-free.
func TestM15UsedFromHTML(t *testing.T) {
	html := `<!doctype html>
<!-- <div class="commented-out"></div> -->
<html><body id="top">
  <DIV CLASS="Card shadow">text</DIV>
  <div class='card'><span class=badge></span></div>
  <img src=/img/x.png class=thumb />
  <a href="/x?a=1&b=2" id=nav-link>link</a>
  <p data-info="class=fake id=fake"></p>
</body></html>`
	u := styl.UsedFromHTML(html)
	if want := []string{"Card", "badge", "card", "shadow", "thumb"}; !reflect.DeepEqual(u.Classes, want) {
		t.Errorf("Classes = %v, want %v", u.Classes, want)
	}
	if want := []string{"nav-link", "top"}; !reflect.DeepEqual(u.IDs, want) {
		t.Errorf("IDs = %v, want %v", u.IDs, want)
	}
	if want := []string{"a", "body", "div", "html", "img", "p", "span"}; !reflect.DeepEqual(u.Tags, want) {
		t.Errorf("Tags = %v, want %v", u.Tags, want)
	}

	empty := styl.UsedFromHTML("just text, no markup")
	if empty.Classes == nil || empty.IDs == nil || empty.Tags == nil {
		t.Errorf("UsedFromHTML must return non-nil slices, got %+v", empty)
	}
}

// TestM15EndToEnd: the documented flow — render HTML, collect names, prune —
// and the pruned sheet only contains rules for what the page uses.
func TestM15EndToEnd(t *testing.T) {
	page := `<body><main id="main">
	  <div class="card"><h2 class="card__title">Hi</h2></div>
	  <a class="btn" href="#">Go</a>
	  <div class="spinner"></div>
	</body>`
	out, err := styl.Prune(m15Sheet, styl.UsedFromHTML(page), styl.Options{Pretty: true})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	for _, want := range []string{".card {", ".card .card__title {", "a.btn {", "#main {", ".spinner {", "@keyframes spin {"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// Tags were collected from the page, so the unused `table`-style rules and
	// the sheet's unused names are all gone.
	for _, gone := range []string{".unused-widget", "#gone", ".sidebar", ".pill", "@keyframes vanish"} {
		if strings.Contains(out, gone) {
			t.Errorf("unexpected %q in:\n%s", gone, out)
		}
	}
}

// TestM15PruneFileAndErrors: PruneFile reads through Options.FS, and parse
// errors surface positioned like Compile's.
func TestM15PruneErrors(t *testing.T) {
	if _, err := styl.Prune(".a\n  color:\n    :bad\n", styl.Used{}, styl.Options{}); err == nil {
		t.Error("want error for malformed sheet, got nil")
	}
	if _, err := styl.PruneFile("missing.styl", styl.Used{}, styl.Options{}); err == nil {
		t.Error("want error for missing file, got nil")
	}
}
