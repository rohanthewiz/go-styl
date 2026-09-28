package styl_test

import (
	"fmt"
	"math"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	styl "github.com/rohanthewiz/go-styl"
)

// migrate runs Migrate on src with notes on and fails the test on error.
func migrate(t *testing.T, src string) styl.MigrateResult {
	t.Helper()
	res, err := styl.Migrate(src, styl.Options{}, styl.MigrateOptions{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return res
}

// hasNote reports whether res carries a note of kind whose message
// contains substr.
func hasNote(res styl.MigrateResult, kind, substr string) bool {
	for _, n := range res.Notes {
		if n.Kind == kind && strings.Contains(n.Msg, substr) {
			return true
		}
	}
	return false
}

func TestMigrateKeepsNesting(t *testing.T) {
	res := migrate(t, `
.card
  color red
  :hover
    color blue
  > .title
    font-weight bold
  .icon
    width 10px
  .dark &
    color white
`)
	want := `.card {
  color: red;

  &:hover {
    color: blue;
  }

  > .title {
    font-weight: bold;
  }

  .icon {
    width: 10px;
  }

  .dark & {
    color: white;
  }
}
`
	if res.CSS != want {
		t.Errorf("got:\n%s\nwant:\n%s", res.CSS, want)
	}
	if len(res.Notes) != 0 {
		t.Errorf("unexpected notes: %v", res.Notes)
	}
}

func TestMigrateVariablesBecomeCustomProperties(t *testing.T) {
	res := migrate(t, `
primary = #0af
link = primary
gap = 8px
unused = 3px
flag = true
.a
  color link
  border 1px solid primary
  margin gap 0
  if flag
    display block
`)
	for _, want := range []string{
		":root {\n  --primary: #0af;\n  --link: var(--primary);\n  --gap: 8px;\n}",
		"color: var(--link);",
		"border: 1px solid var(--primary);",
		"margin: var(--gap) 0;",
		"display: block;",
	} {
		if !strings.Contains(res.CSS, want) {
			t.Errorf("missing %q in:\n%s", want, res.CSS)
		}
	}
	// Only variables the output reads get a property; booleans never do.
	for _, bad := range []string{"--unused", "--flag"} {
		if strings.Contains(res.CSS, bad) {
			t.Errorf("unexpected %s in:\n%s", bad, res.CSS)
		}
	}
}

func TestMigrateDerivedVariables(t *testing.T) {
	res := migrate(t, `
brand = #0af
dark = darken(brand, 10%)
top = 10px
mid = top * 2
.a
  color dark
  margin mid
`)
	for _, want := range []string{
		"--mid: calc(var(--top) * 2);",
		"--dark: #0099e6;",
	} {
		if !strings.Contains(res.CSS, want) {
			t.Errorf("missing %q in:\n%s", want, res.CSS)
		}
	}
	if !hasNote(res, "frozen", "--dark computed at migrate time from --brand") {
		t.Errorf("no frozen note for --dark: %v", res.Notes)
	}
	// --dark was computed, so nothing reads --brand any more.
	if strings.Contains(res.CSS, "--brand:") {
		t.Errorf("unused --brand emitted:\n%s", res.CSS)
	}
}

func TestMigrateNoVarsInlines(t *testing.T) {
	res, err := styl.Migrate("c = red\n.a\n  color c\n", styl.Options{}, styl.MigrateOptions{NoVars: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := ".a {\n  color: red;\n}\n"; res.CSS != want {
		t.Errorf("got:\n%s\nwant:\n%s", res.CSS, want)
	}
}

func TestMigrateCalcAndFrozen(t *testing.T) {
	res := migrate(t, `
base = 8px
c = #0af
.a
  width base * 10
  padding base (base / 2) ((base + 2px) * 3)
  height base + 2
  color darken(c, 10%)
`)
	for _, want := range []string{
		"width: calc(var(--base) * 10);",
		"padding: var(--base) calc(var(--base) / 2) calc((var(--base) + 2px) * 3);",
		// 8px + 2 is 10px in Stylus but invalid calc(): computed instead.
		"height: 10px;",
		"color: #0099e6;",
	} {
		if !strings.Contains(res.CSS, want) {
			t.Errorf("missing %q in:\n%s", want, res.CSS)
		}
	}
	if !hasNote(res, "frozen", "height computed at migrate time from --base") {
		t.Errorf("no frozen note for height: %v", res.Notes)
	}
	if !hasNote(res, "frozen", "color computed at migrate time from --c") {
		t.Errorf("no frozen note for color: %v", res.Notes)
	}
	if !strings.Contains(res.CSS, "/* styl-migrate: frozen: color computed") {
		t.Errorf("missing inline note in:\n%s", res.CSS)
	}
}

func TestMigrateHoistsConcatenation(t *testing.T) {
	res := migrate(t, `
.block
  color red
  &__el
    color blue
  @media (min-width: 10px)
    &--wide
      width 100%
.next
  color green
`)
	want := `.block {
  color: red;
}

/* styl-migrate: hoisted: "&__el" can't nest in CSS; moved after the enclosing block as .block__el (check cascade order) */
.block__el {
  color: blue;
}

@media (min-width: 10px) {
  /* styl-migrate: hoisted: "&--wide" can't nest in CSS; moved after the enclosing block as .block--wide (check cascade order) */
  .block--wide {
    width: 100%;
  }
}

.next {
  color: green;
}
`
	if strings.TrimPrefix(res.CSS, headerLine(res.CSS)) != want {
		t.Errorf("got:\n%s\nwant:\n%s", res.CSS, want)
	}
}

// headerLine returns the migrate header comment and its blank line, if any.
func headerLine(css string) string {
	if !strings.HasPrefix(css, "/* Migrated from") {
		return ""
	}
	i := strings.Index(css, "\n\n")
	return css[:i+2]
}

func TestMigratePseudoElementParentHoists(t *testing.T) {
	res := migrate(t, "a::before\n  content ''\n  &:hover\n    color red\n")
	if !strings.Contains(res.CSS, "\na::before:hover {") {
		t.Errorf("expected hoisted a::before:hover in:\n%s", res.CSS)
	}
}

func TestMigrateMixinsLoopsAndExtend(t *testing.T) {
	res := migrate(t, `
round(r = 4px)
  border-radius r
  &:focus
    outline none
.btn
  round()
.msg
  padding 1px
.warn
  @extend .msg
  color orange
$hidden
  display none
.modal
  @extend $hidden
for i in 1..2
  .m-{i}
    margin i
`)
	for _, want := range []string{
		"/* styl-migrate: mixin: expanded round() */\n  border-radius: 4px;\n\n  &:focus {\n    outline: none;\n  }",
		".msg, .warn {\n  padding: 1px;\n}",
		".modal {\n  display: none;\n}",
		"/* styl-migrate: loop: unrolled for-loop (2 iterations) */\n.m-1 {",
		".m-2 {\n  margin: 2;\n}",
	} {
		if !strings.Contains(res.CSS, want) {
			t.Errorf("missing %q in:\n%s", want, res.CSS)
		}
	}
	if strings.Contains(res.CSS, "$hidden") {
		t.Errorf("placeholder leaked into:\n%s", res.CSS)
	}
}

func TestMigrateReassignedVariable(t *testing.T) {
	res := migrate(t, "c = red\n.a\n  color c\nc = blue\n.b\n  color c\n")
	for _, want := range []string{"--c: red;", ".a {\n  color: var(--c);", ".b {\n  color: blue;"} {
		if !strings.Contains(res.CSS, want) {
			t.Errorf("missing %q in:\n%s", want, res.CSS)
		}
	}
	if !hasNote(res, "variable", "c is reassigned") {
		t.Errorf("no reassignment note: %v", res.Notes)
	}
}

func TestMigrateImportPreambleOrder(t *testing.T) {
	res := migrate(t, "@import 'reset.css'\nc = red\n.a\n  color c\n")
	i, j := strings.Index(res.CSS, "@import"), strings.Index(res.CSS, ":root")
	if i < 0 || j < 0 || i > j {
		t.Errorf("@import must precede :root in:\n%s", res.CSS)
	}
}

func TestMigrateNoNotes(t *testing.T) {
	res, err := styl.Migrate("m()\n  color red\n.a\n  m()\n", styl.Options{}, styl.MigrateOptions{NoNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.CSS, "/*") {
		t.Errorf("comments with NoNotes:\n%s", res.CSS)
	}
	if len(res.Notes) != 1 {
		t.Errorf("notes should still be returned, got %v", res.Notes)
	}
}

func TestMigrateErrorsArePositioned(t *testing.T) {
	_, err := styl.Migrate(".a\n  color undefined-fn(\n", styl.Options{Filename: "x.styl"}, styl.MigrateOptions{})
	if err == nil || !strings.HasPrefix(err.Error(), "x.styl:") {
		t.Errorf("want positioned error, got %v", err)
	}
}

// TestMigrateRoundTrip is the migration's correctness check: for every
// example and fixture sheet, the migrated CSS — with its nesting resolved
// the way a browser does, var() replaced by the :root values and calc()
// computed — must declare exactly what Compile's output declares.
// Declarations are compared as a set of (at-rules, selector, property,
// value), so hoisting and @extend regrouping don't count as differences.
func TestMigrateRoundTrip(t *testing.T) {
	var files []string
	for _, pat := range []string{"examples/*.styl", "testdata/*.styl", "difftest/corpus/*.styl"} {
		m, err := filepath.Glob(pat)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Fatal("no sheets found")
	}
	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			compiled, cerr := styl.CompileFile(f, styl.Options{Pretty: true, Warn: func(string) {}})
			res, merr := styl.MigrateFile(f, styl.Options{Warn: func(string) {}}, styl.MigrateOptions{})
			if cerr != nil || merr != nil {
				// A sheet that doesn't compile (an error fixture) must not
				// migrate either.
				if (cerr == nil) != (merr == nil) {
					t.Fatalf("compile err %v, migrate err %v", cerr, merr)
				}
				return
			}
			want := flatDecls(t, compiled)
			got := flatDecls(t, res.CSS)
			if !slices.Equal(want, got) {
				t.Errorf("declarations differ\n--- compiled only:\n%s\n--- migrated only:\n%s\n--- migrated CSS:\n%s",
					strings.Join(minus(want, got), "\n"), strings.Join(minus(got, want), "\n"), res.CSS)
			}
		})
	}
}

// minus returns the items of a missing from b.
func minus(a, b []string) []string {
	var out []string
	for _, x := range a {
		if !slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

// cssBlock is a parsed block of pretty CSS: a header (selector list or
// at-rule prelude) with declarations and child blocks. The root has no
// header.
type cssBlock struct {
	header string
	decls  [][2]string
	raws   []string
	kids   []*cssBlock
}

// parseCSS reads the line-oriented pretty CSS that Compile and Migrate
// print (one declaration, header or brace per line). Comments, which
// Migrate writes on lines of their own, are dropped (a multi-line one
// whole).
func parseCSS(t *testing.T, src string) *cssBlock {
	t.Helper()
	root := &cssBlock{}
	stack := []*cssBlock{root}
	inComment := false
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		cur := stack[len(stack)-1]
		if inComment || strings.HasPrefix(line, "/*") {
			inComment = !strings.HasSuffix(line, "*/")
			continue
		}
		switch {
		case line == "":
		case strings.HasSuffix(line, "{"):
			b := &cssBlock{header: strings.TrimSpace(strings.TrimSuffix(line, "{"))}
			cur.kids = append(cur.kids, b)
			stack = append(stack, b)
		case line == "}":
			stack = stack[:len(stack)-1]
		case strings.HasPrefix(line, "@"):
			cur.raws = append(cur.raws, line)
		default:
			prop, val, ok := strings.Cut(strings.TrimSuffix(line, ";"), ":")
			if !ok {
				t.Fatalf("unparsed line %q", line)
			}
			cur.decls = append(cur.decls, [2]string{strings.TrimSpace(prop), strings.TrimSpace(val)})
		}
	}
	return root
}

// flatDecls flattens pretty CSS (nested or not) into sorted
// "at-rules | selector | prop: value" lines, resolving var() against the
// top-level :root rule and computing calc().
func flatDecls(t *testing.T, src string) []string {
	root := parseCSS(t, src)
	vars := map[string]string{}
	var kids []*cssBlock
	for _, k := range root.kids {
		if k.header == ":root" && len(k.decls) > 0 && strings.HasPrefix(k.decls[0][0], "--") {
			for _, d := range k.decls {
				vars[d[0]] = d[1]
			}
			continue
		}
		kids = append(kids, k)
	}
	var out []string
	for _, r := range root.raws {
		out = append(out, "raw | "+r)
	}
	var walk func(b *cssBlock, sels []string, at string, inKeyframes bool)
	walk = func(b *cssBlock, sels []string, at string, inKeyframes bool) {
		for _, d := range b.decls {
			val := evalCalc(t, resolveVars(d[1], vars))
			if len(sels) == 0 {
				out = append(out, fmt.Sprintf("%s |  | %s: %s", at, d[0], val))
			}
			for _, s := range sels {
				out = append(out, fmt.Sprintf("%s | %s | %s: %s", at, s, d[0], val))
			}
		}
		for _, k := range b.kids {
			switch {
			case strings.HasPrefix(k.header, "@"):
				kf := strings.Contains(k.header, "keyframes")
				ks := sels
				if kf || strings.HasPrefix(k.header, "@font-face") || strings.HasPrefix(k.header, "@page") {
					ks = nil
				}
				walk(k, ks, strings.TrimSpace(at+" "+k.header), kf)
			case inKeyframes || len(sels) == 0:
				walk(k, splitSelectors(k.header), at, false)
			default:
				var combined []string
				for _, c := range splitSelectors(k.header) {
					for _, p := range sels {
						if strings.Contains(c, "&") {
							combined = append(combined, strings.ReplaceAll(c, "&", p))
						} else {
							combined = append(combined, p+" "+c)
						}
					}
				}
				walk(k, combined, at, false)
			}
		}
	}
	walk(&cssBlock{kids: kids}, nil, "", false)
	slices.Sort(out)
	return slices.Compact(out)
}

// splitSelectors splits a selector list at top-level commas.
func splitSelectors(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, c := range s {
		switch c {
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	return append(out, strings.TrimSpace(s[start:]))
}

// resolveVars substitutes var(--x) with its :root value until none are
// left.
func resolveVars(val string, vars map[string]string) string {
	for range 32 {
		i := strings.Index(val, "var(--")
		if i < 0 {
			return val
		}
		j := strings.IndexByte(val[i:], ')')
		name := val[i+4 : i+j]
		val = val[:i] + vars[name] + val[i+j+1:]
	}
	return val
}

// evalCalc replaces each calc(...) holding plain numeric arithmetic with
// its result, formatted as go-styl formats numbers.
func evalCalc(t *testing.T, val string) string {
	for {
		i := strings.Index(val, "calc(")
		if i < 0 {
			return val
		}
		depth, end := 0, -1
		for j := i + 4; j < len(val); j++ {
			if val[j] == '(' {
				depth++
			} else if val[j] == ')' {
				depth--
				if depth == 0 {
					end = j
					break
				}
			}
		}
		p := &calcParser{s: val[i+5 : end]}
		n, unit := p.sum()
		if p.err != nil {
			t.Fatalf("calc %q: %v", val[i:end+1], p.err)
		}
		val = val[:i] + formatCalc(n) + unit + val[end+1:]
	}
}

func formatCalc(f float64) string {
	if f == math.Trunc(f) {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	r, _ := strconv.ParseFloat(strconv.FormatFloat(f, 'f', 15, 64), 64)
	return strconv.FormatFloat(r, 'f', -1, 64)
}

// calcParser evaluates + - * / over numbers with units and parentheses.
// A result's unit is whichever operand had one.
type calcParser struct {
	s   string
	pos int
	err error
}

func (p *calcParser) skip() {
	for p.pos < len(p.s) && p.s[p.pos] == ' ' {
		p.pos++
	}
}

func (p *calcParser) sum() (float64, string) {
	n, u := p.product()
	for {
		p.skip()
		if p.pos >= len(p.s) || (p.s[p.pos] != '+' && p.s[p.pos] != '-') {
			return n, u
		}
		op := p.s[p.pos]
		p.pos++
		m, v := p.product()
		if op == '+' {
			n += m
		} else {
			n -= m
		}
		if u == "" {
			u = v
		}
	}
}

func (p *calcParser) product() (float64, string) {
	n, u := p.atom()
	for {
		p.skip()
		if p.pos >= len(p.s) || (p.s[p.pos] != '*' && p.s[p.pos] != '/') {
			return n, u
		}
		op := p.s[p.pos]
		p.pos++
		m, v := p.atom()
		if op == '*' {
			n *= m
		} else {
			n /= m
		}
		if u == "" {
			u = v
		}
	}
}

func (p *calcParser) atom() (float64, string) {
	p.skip()
	if p.pos < len(p.s) && p.s[p.pos] == '(' {
		p.pos++
		n, u := p.sum()
		p.skip()
		p.pos++ // ')'
		return n, u
	}
	start := p.pos
	for p.pos < len(p.s) && (p.s[p.pos] == '.' || p.s[p.pos] == '-' && p.pos == start || p.s[p.pos] >= '0' && p.s[p.pos] <= '9') {
		p.pos++
	}
	n, err := strconv.ParseFloat(p.s[start:p.pos], 64)
	if err != nil {
		p.err = fmt.Errorf("bad number at %d", start)
		return 0, ""
	}
	us := p.pos
	for p.pos < len(p.s) && (p.s[p.pos] == '%' || p.s[p.pos] >= 'a' && p.s[p.pos] <= 'z') {
		p.pos++
	}
	return n, p.s[us:p.pos]
}

// TestMigrateKeepsComments checks that source comments reach the CSS in
// place: `//` as `/* … */`, inline ones above their statement, block
// comments verbatim and re-indented, a comment closing a block at the end
// of it, and a header's blank line kept.
func TestMigrateKeepsComments(t *testing.T) {
	src := `// Card styles

.card
  color red // why red
  /* multi
     line */
  padding 4px
  // closing
.b
  x 1
`
	res, err := styl.Migrate(src, styl.Options{}, styl.MigrateOptions{NoNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	want := `/* Card styles */

.card {
  /* why red */
  color: red;
  /* multi
     line */
  padding: 4px;
  /* closing */
}

.b {
  x: 1;
}
`
	if res.CSS != want {
		t.Errorf("got:\n%s\nwant:\n%s", res.CSS, want)
	}
}

func TestMigrateCommentsBraceSyntax(t *testing.T) {
	src := "/* Header */\n.a {\n  color: red; // why\n  .b { x: 1; } /* nested */\n}\n"
	res, err := styl.Migrate(src, styl.Options{}, styl.MigrateOptions{NoNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"/* Header */", "/* why */", "/* nested */"} {
		if !strings.Contains(res.CSS, c) {
			t.Errorf("missing %s in:\n%s", c, res.CSS)
		}
	}
	if strings.Index(res.CSS, "/* why */") > strings.Index(res.CSS, "color: red") {
		t.Errorf("inline comment should precede its declaration:\n%s", res.CSS)
	}
}

// TestMigrateVariableDocsMoveToRoot: comments directly above (or on the
// line of) a root variable go into :root with its custom property; one
// set apart by a blank line stays where it was, above :root.
func TestMigrateVariableDocsMoveToRoot(t *testing.T) {
	src := `// Theme tokens

// Brand color
primary = #c00
gap = 8px // base spacing

.a
  color primary
  margin gap
`
	res, err := styl.Migrate(src, styl.Options{}, styl.MigrateOptions{NoNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	want := `/* Theme tokens */

:root {
  /* Brand color */
  --primary: #c00;
  /* base spacing */
  --gap: 8px;
}

.a {
  color: var(--primary);
  margin: var(--gap);
}
`
	if res.CSS != want {
		t.Errorf("got:\n%s\nwant:\n%s", res.CSS, want)
	}

	// With NoVars nothing moves: the comments stay where they were.
	res, err = styl.Migrate(src, styl.Options{}, styl.MigrateOptions{NoNotes: true, NoVars: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.CSS, "/* Brand color */") || !strings.Contains(res.CSS, "/* base spacing */") {
		t.Errorf("NoVars lost variable comments:\n%s", res.CSS)
	}
}

// TestMigrateMixinComments: a mixin's doc comment goes with its
// definition (noted), comments inside its body are written at each
// expansion, and a loop body's comment is written once.
func TestMigrateMixinComments(t *testing.T) {
	src := `// Clears floats
clearfix()
  // IE hack
  zoom 1

.a
  clearfix()
.b
  clearfix()
for i in 1..3
  // per column
  .c{i}
    x i
`
	res := migrate(t, src)
	if strings.Contains(res.CSS, "Clears floats") {
		t.Errorf("mixin doc comment kept:\n%s", res.CSS)
	}
	if !hasNote(res, "comment", "clearfix()") {
		t.Errorf("no note for the dropped comment: %v", res.Notes)
	}
	if n := strings.Count(res.CSS, "/* IE hack */"); n != 2 {
		t.Errorf("mixin body comment written %d times, want 2:\n%s", n, res.CSS)
	}
	if n := strings.Count(res.CSS, "/* per column */"); n != 1 {
		t.Errorf("loop body comment written %d times, want 1:\n%s", n, res.CSS)
	}
}

func TestMigrateNoComments(t *testing.T) {
	res, err := styl.Migrate("// a\n.a\n  x 1 // b\n", styl.Options{}, styl.MigrateOptions{NoComments: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.CSS, "/*") {
		t.Errorf("comments with NoComments:\n%s", res.CSS)
	}
}

// linkSplit splices a Split migration back into one stylesheet: each
// `@import "x.css";` naming another output file is replaced by that
// file's (linked) CSS. Other imports stay.
func linkSplit(t *testing.T, files []styl.MigratedFile) string {
	t.Helper()
	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.CSS
	}
	var link func(p string, depth int) string
	link = func(p string, depth int) string {
		if depth > 20 {
			t.Fatalf("import loop at %s", p)
		}
		var b strings.Builder
		for _, line := range strings.SplitAfter(byPath[p], "\n") {
			trimmed := strings.TrimSpace(line)
			if url, ok := strings.CutPrefix(trimmed, `@import "`); ok {
				target := path.Join(path.Dir(p), strings.TrimSuffix(url, `";`))
				if _, known := byPath[target]; known {
					b.WriteString(link(target, depth+1))
					continue
				}
			}
			b.WriteString(line)
		}
		return b.String()
	}
	return link(files[0].Path, 0)
}

// TestMigrateSplitRoundTrip: under Split, the files spliced back together
// declare exactly what Compile's output does (see TestMigrateRoundTrip).
func TestMigrateSplitRoundTrip(t *testing.T) {
	var files []string
	for _, pat := range []string{"examples/*.styl", "testdata/*.styl", "testdata/*/main.styl", "difftest/corpus/*.styl"} {
		m, err := filepath.Glob(pat)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			compiled, cerr := styl.CompileFile(f, styl.Options{Pretty: true, Warn: func(string) {}})
			res, merr := styl.MigrateFile(f, styl.Options{Warn: func(string) {}}, styl.MigrateOptions{Split: true})
			if cerr != nil || merr != nil {
				if (cerr == nil) != (merr == nil) {
					t.Fatalf("compile err %v, migrate err %v", cerr, merr)
				}
				return
			}
			if len(res.Files) == 0 || res.Files[0].CSS != res.CSS {
				t.Fatalf("Files[0] should be the entry sheet")
			}
			want := flatDecls(t, compiled)
			got := flatDecls(t, linkSplit(t, res.Files))
			if !slices.Equal(want, got) {
				t.Errorf("declarations differ\n--- compiled only:\n%s\n--- migrated only:\n%s",
					strings.Join(minus(want, got), "\n"), strings.Join(minus(got, want), "\n"))
			}
		})
	}
}

// splitProject is a small multi-file project covering each Split rule.
var splitProject = fstest.MapFS{
	"src/main.styl": {Data: []byte(`// Site entry

@import "_vars"
@import "ui/buttons"
.page
  color brand
  @import "_inner"
@import "late"
`)},
	// Variables and a mixin only: no file.
	"src/_vars.styl": {Data: []byte("brand = #c00\npad(n)\n  padding n\n")},
	// Rules, plus a placeholder another file extends and a nested import.
	"src/ui/buttons.styl": {Data: []byte("@import \"_base\"\n.btn\n  pad(4px)\n  color brand\n")},
	"src/ui/_base.styl":   {Data: []byte("$reset\n  margin 0\n")},
	// Imported inside a rule: inlined.
	"src/_inner.styl": {Data: []byte("background white\n")},
	// Imported after a rule: hoisted, noted.
	"src/late.styl": {Data: []byte(".late\n  @extend $reset\n  x 1\n")},
}

func TestMigrateSplitProject(t *testing.T) {
	res, err := styl.MigrateFile("src/main.styl", styl.Options{FS: splitProject}, styl.MigrateOptions{Split: true, NoNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	var paths []string
	for _, f := range res.Files {
		got[f.Path] = f.CSS
		paths = append(paths, f.Path)
	}
	if want := []string{"main.css", "tokens.css", "ui/buttons.css", "ui/_base.css", "late.css"}; !slices.Equal(paths, want) {
		t.Fatalf("files = %v, want %v", paths, want)
	}

	wantMain := `/* Site entry */

@import "tokens.css";
@import "ui/buttons.css";
@import "late.css";

.page {
  color: var(--brand);
  background: white;
}
`
	if got["main.css"] != wantMain {
		t.Errorf("main.css:\n%s\nwant:\n%s", got["main.css"], wantMain)
	}
	if !strings.Contains(got["tokens.css"], "--brand: #c00;") {
		t.Errorf("tokens.css:\n%s", got["tokens.css"])
	}
	// Imports are relative to the importing file.
	if !strings.HasPrefix(got["ui/buttons.css"], `@import "_base.css";`) || !strings.Contains(got["ui/buttons.css"], "padding: 4px;") {
		t.Errorf("ui/buttons.css:\n%s", got["ui/buttons.css"])
	}
	// The placeholder took its extender, so the partial is written.
	if !strings.Contains(got["ui/_base.css"], ".late {") {
		t.Errorf("ui/_base.css:\n%s", got["ui/_base.css"])
	}

	for _, want := range []struct{ kind, substr string }{
		{"import", "_vars.styl emits no CSS"},
		{"import", "inside a rule or at-rule"},
		{"hoisted", "@import of late.css moved to the top"},
	} {
		if !hasNote(res, want.kind, want.substr) {
			t.Errorf("missing %s note %q in %v", want.kind, want.substr, res.Notes)
		}
	}
}

// TestMigrateSplitNames: a source outside the entry's directory goes under
// _external/, a name collision is numbered, and TokensFile is honored.
func TestMigrateSplitNames(t *testing.T) {
	fsys := fstest.MapFS{
		"app/main.styl":      {Data: []byte("@import \"../lib/a\"\n@import \"b\"\nc = 1px\n.m\n  x c\n")},
		"lib/a.styl":         {Data: []byte(".a\n  x 1\n")},
		"app/b.styl":         {Data: []byte(".b\n  x 1\n")},
		"app/theme/vars.css": {Data: []byte("")},
	}
	res, err := styl.MigrateFile("app/main.styl", styl.Options{FS: fsys},
		styl.MigrateOptions{Split: true, NoNotes: true, TokensFile: "b.css"})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, f := range res.Files {
		paths = append(paths, f.Path)
	}
	if want := []string{"main.css", "b.css", "_external/a.css", "b-2.css"}; !slices.Equal(paths, want) {
		t.Errorf("files = %v, want %v", paths, want)
	}
	if !strings.Contains(res.CSS, `@import "_external/a.css";`) || !strings.Contains(res.CSS, `@import "b-2.css";`) {
		t.Errorf("main.css:\n%s", res.CSS)
	}
}

// TestMigrateSplitRepeatImport: a plain second @import of a file already
// written out is inlined, noted.
func TestMigrateSplitRepeatImport(t *testing.T) {
	fsys := fstest.MapFS{
		"main.styl": {Data: []byte("@import \"a\"\n@import \"a\"\n")},
		"a.styl":    {Data: []byte(".a\n  x 1\n")},
	}
	res, err := styl.MigrateFile("main.styl", styl.Options{FS: fsys}, styl.MigrateOptions{Split: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.CSS, `@import "a.css";`) || !strings.Contains(res.CSS, ".a {") {
		t.Errorf("main.css:\n%s", res.CSS)
	}
	if !hasNote(res, "import", "imported before as a.css") {
		t.Errorf("notes: %v", res.Notes)
	}
}
