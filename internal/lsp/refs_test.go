package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// spans renders locations as "file:line:col-col" (0-based, as sent) for
// order-independent comparison.
func spans(locs []Location) string {
	var out []string
	for _, l := range locs {
		out = append(out, fmt.Sprintf("%s:%d:%d-%d", filepath.Base(l.URI), l.Range.Start.Line, l.Range.Start.Character, l.Range.End.Character))
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

func refsReq(uri string, line, char int, decl bool) map[string]any {
	p := pos(uri, line, char)
	p["context"] = map[string]any{"includeDeclaration": decl}
	return p
}

// TestReferencesAndRename runs over the shared project fixture:
//
//	_vars.styl                    main.styl
//	0 brand = #0af                0 @import '_vars'
//	1 pad = 4px                   1 gap = pad * 3
//	2 button(bg = brand)          2 .card
//	3   background bg             3   margin gap
//	4   padding pad * 2           4   color #fff
//	                              5   button()
//	                              6   gap = 1px
//	                              7   padding gap
//	                              8 #bad
//	                              9   border 1px solid brand
func TestReferencesAndRename(t *testing.T) {
	dir, uri, src := project(t)
	c := newClient(t)
	c.open(uri, src)

	t.Run("across the import, from the use", func(t *testing.T) {
		var locs []Location
		json.Unmarshal(c.call("textDocument/references", refsReq(uri, 9, 20, true)), &locs)
		want := "_vars.styl:0:0-5 _vars.styl:2:12-17 main.styl:9:19-24"
		if got := spans(locs); got != want {
			t.Errorf("references of brand:\n got %s\nwant %s", got, want)
		}
		json.Unmarshal(c.call("textDocument/references", refsReq(uri, 9, 20, false)), &locs)
		if got := spans(locs); got != "_vars.styl:2:12-17 main.styl:9:19-24" {
			t.Errorf("without the declaration: %s", got)
		}
	})

	t.Run("a local is its own symbol", func(t *testing.T) {
		var locs []Location
		json.Unmarshal(c.call("textDocument/references", refsReq(uri, 3, 10, true)), &locs)
		if got := spans(locs); got != "main.styl:1:0-3 main.styl:3:9-12" {
			t.Errorf("root gap: %s", got)
		}
		json.Unmarshal(c.call("textDocument/references", refsReq(uri, 7, 11, true)), &locs)
		if got := spans(locs); got != "main.styl:6:2-5 main.styl:7:10-13" {
			t.Errorf("local gap: %s", got)
		}
	})

	t.Run("rename across files leaves properties alone", func(t *testing.T) {
		var we WorkspaceEdit
		json.Unmarshal(c.call("textDocument/rename", map[string]any{
			"textDocument": map[string]any{"uri": uri}, "position": map[string]any{"line": 1, "character": 7}, "newName": "space",
		}), &we)
		varsURI := pathToURI(filepath.Join(dir, "_vars.styl"))
		got := map[string]string{}
		for u, edits := range we.Changes {
			var ls []Location
			for _, e := range edits {
				if e.NewText != "space" {
					t.Errorf("edit text %q", e.NewText)
				}
				ls = append(ls, Location{URI: u, Range: e.Range})
			}
			got[u] = spans(ls)
		}
		if got[uri] != "main.styl:1:6-9" || got[varsURI] != "_vars.styl:1:0-3 _vars.styl:4:10-13" {
			t.Errorf("rename edits: %v", got)
		}
	})

	t.Run("prepareRename", func(t *testing.T) {
		if raw := string(c.call("textDocument/prepareRename", pos(uri, 3, 4))); raw != "null" {
			t.Errorf("a property name is not renameable: %s", raw)
		}
		var r Range
		json.Unmarshal(c.call("textDocument/prepareRename", pos(uri, 5, 4)), &r)
		if r.Start.Line != 5 || r.Start.Character != 2 || r.End.Character != 8 {
			t.Errorf("prepareRename of button: %+v", r)
		}
	})
}

// TestCollectUses covers use sites the AST only knows by name.
func TestCollectUses(t *testing.T) {
	src := "base = '/img'\n" + // 1
		"n = 3\n" + //              2
		".col-{n}\n" + //           3
		"  // n in a comment\n" + //4
		"  content 'n={n}'\n" + //  5
		"  background url(base + '/x.png')\n" + // 6
		"  font-family n,\n" + //   7
		"    n\n" + //              8 (continuation of the value)
		"  if n > 1\n" + //         9
		"    color red\n" + //     10
		"  else if n < 0\n" + //   11
		"    color blue\n" + //    12
		"  .n\n" + //              13  (a class, not a use)
		"    margin 0\n" //        14
	lines := strings.Split(src, "\n")
	an := analyze("", src, nil, false)
	if an.sheet == nil {
		t.Fatalf("parse: %+v", an.diags)
	}
	var got []string
	for _, u := range an.uses {
		if u.Name == "n" || u.Name == "base" {
			got = append(got, fmt.Sprintf("%s@%d:%d", u.Name, u.Line, u.Start))
		}
	}
	want := "n@3:6 n@5:14 base@6:17 n@7:14 n@8:4 n@9:5 n@11:10"
	if strings.Join(got, " ") != want {
		t.Errorf("uses:\n got %s\nwant %s", strings.Join(got, " "), want)
	}
	_ = lines
}

func TestSignatureHelp(t *testing.T) {
	uri := "untitled:sig"
	c := newClient(t)
	// Text being typed doesn't parse; the index comes from the last good
	// version.
	c.open(uri, "pair(a, b = f(1, 2), rest...)\n  x a\n.a\n  color red\n")
	c.change(uri, "pair(a, b = f(1, 2), rest...)\n  x a\n.a\n  +pair(1, \n  pair(1, 2, 3, 4)\n  darken(red, \n  c rgba(1, 2, \n")
	var h SignatureHelp
	json.Unmarshal(c.call("textDocument/signatureHelp", pos(uri, 3, 11)), &h)
	if len(h.Signatures) != 1 || h.Signatures[0].Label != "pair(a, b = f(1, 2), rest...)" || h.ActiveParameter != 1 {
		t.Errorf("signature help: %+v", h)
	}
	if ps := h.Signatures[0].Parameters; len(ps) != 3 || ps[1].Label != "b = f(1, 2)" {
		t.Errorf("parameters: %+v", ps)
	}
	json.Unmarshal(c.call("textDocument/signatureHelp", pos(uri, 4, 16)), &h)
	if h.ActiveParameter != 2 {
		t.Errorf("a rest parameter takes the extra arguments: %+v", h)
	}

	// Built-ins: parameter lists from the registry.
	h = SignatureHelp{}
	json.Unmarshal(c.call("textDocument/signatureHelp", pos(uri, 5, 14)), &h)
	if len(h.Signatures) != 1 || h.Signatures[0].Label != "darken(color, amount)" || h.ActiveParameter != 1 {
		t.Errorf("built-in signature help: %+v", h)
	}
	// rgba has two forms; from the third argument only the four-channel one fits.
	h = SignatureHelp{}
	json.Unmarshal(c.call("textDocument/signatureHelp", pos(uri, 6, 15)), &h)
	if len(h.Signatures) != 2 || h.ActiveSignature != 0 || h.ActiveParameter != 2 ||
		h.Signatures[1].Label != "rgba(color, alpha)" {
		t.Errorf("rgba signature help: %+v", h)
	}
}

func TestBuiltinHover(t *testing.T) {
	uri := "untitled:bihover"
	c := newClient(t)
	c.open(uri, ".a\n  color darken(red, 10%)\n")
	var h Hover
	json.Unmarshal(c.call("textDocument/hover", pos(uri, 1, 10)), &h)
	if !strings.Contains(h.Contents.Value, "darken(color, amount)") {
		t.Errorf("hover: %q", h.Contents.Value)
	}
}

func TestPropertyCompletion(t *testing.T) {
	uri := "untitled:props"
	c := newClient(t)
	c.open(uri, ".a\n  backg\nroot\n.b { color: red; bor }\n")
	has := func(line, char int) bool {
		var list CompletionList
		json.Unmarshal(c.call("textDocument/completion", pos(uri, line, char)), &list)
		for _, it := range list.Items {
			if it.Label == "background-color" && it.Kind == kindProperty {
				return true
			}
		}
		return false
	}
	if !has(1, 7) {
		t.Error("no properties on an indented line")
	}
	if has(2, 4) {
		t.Error("properties offered at the root")
	}
	if !has(3, 20) {
		t.Error("no properties after ';' in brace syntax")
	}
	if has(3, 14) {
		t.Error("properties offered in a value")
	}
}

func lintMsgs(diags []Diagnostic) string {
	var out []string
	for _, d := range diags {
		if d.Source == lintSource {
			out = append(out, fmt.Sprintf("%d:%s", d.Range.Start.Line, d.Message))
		}
	}
	return strings.Join(out, "; ")
}

func TestLint(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "main.styl")
	src := "" +
		"theme = red\n" + //                        0 root var: never flagged
		"unused-mixin()\n" + //                     1 root mixin in an entry sheet
		"  color red\n" + //                        2
		"used-mixin(p)\n" + //                      3 p unused: parameters aren't flagged
		"  tmp = 1\n" + //                          4 unused local
		"  helper(x)\n" + //                        5 unused local function
		"    x\n" + //                              6
		"  color blue\n" + //                       7
		"@import '_partial'\n" + //                 8
		".a\n" + //                                 9
		"  +used-mixin(1)\n" + //                  10
		"  +from-entry()\n" + //                   11
		"  display -webkit-box\n" + //             12
		"  display flex\n" + //                    13 fallback: fine
		"  margin 0\n" + //                        14
		"  color red\n" + //                       15
		"  margin 1px\n" + //                      16 duplicate
		"  for i, x in 1 2\n" + //                 17 loop vars aren't flagged
		"    width 1px\n" + //                     18
		"from-entry()\n" + //                      19 called from main itself
		"  z-index 1\n" + //                       20
		"called-by-partial()\n" + //               21 called only from _partial
		"  z-index 2\n" //                         22
	os.WriteFile(filepath.Join(dir, "_partial.styl"), []byte("lib-mixin()\n  top 0\n.p\n  +called-by-partial()\n"), 0o644)
	an := analyze(main, src, nil, false)
	want := "16:duplicate property margin (also set on line 15); " +
		"1:function unused-mixin is never used; " +
		"4:variable tmp is never used; " +
		"5:function helper is never used"
	if got := lintMsgs(an.diags); got != want {
		t.Errorf("lint:\n got %s\nwant %s", got, want)
	}
	for _, d := range an.diags {
		if strings.Contains(d.Message, "never used") && (d.Severity != SeverityHint || len(d.Tags) != 1) {
			t.Errorf("unused should be a faded hint: %+v", d)
		}
	}

	// The same sheet as a partial: root functions aren't checked.
	an = analyze(main, src, nil, true)
	if got := lintMsgs(an.diags); strings.Contains(got, "unused-mixin") {
		t.Errorf("partial lint: %s", got)
	}
	// Nor is a _file, and lookup() switches off the unused-variable check.
	an = analyze(filepath.Join(dir, "_lib.styl"), "m()\n  v = 1\n  width lookup('v')\n", nil, false)
	if got := lintMsgs(an.diags); got != "" {
		t.Errorf("_lib lint: %s", got)
	}
}

// TestTransparentMixinUse: a declaration whose property names a mixin calls
// it, so it counts as a use; the same property inside the mixin's own body
// is a plain property (Stylus doesn't recurse there).
func TestTransparentMixinUse(t *testing.T) {
	src := "size(w)\n  width w\n  size w\nradius(n)\n  radius n\n.a\n  size 10px\n"
	file := filepath.Join(t.TempDir(), "main.styl")
	an := analyze(file, src, nil, false)
	if got := lintMsgs(an.diags); got != "3:function radius is never used" {
		t.Errorf("lint: %s", got)
	}
	var uses []string
	for _, u := range an.uses {
		if d, ok := resolveUse(an, file, u); ok && d.Kind == defFunc {
			uses = append(uses, fmt.Sprintf("%s@%d", u.Name, u.Line))
		}
	}
	if got := strings.Join(uses, " "); got != "size@7" {
		t.Errorf("function uses: %s", got)
	}
}
