package styl_test

import (
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

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
// print (one declaration, header or brace per line). Comments are
// dropped.
func parseCSS(t *testing.T, src string) *cssBlock {
	t.Helper()
	root := &cssBlock{}
	stack := []*cssBlock{root}
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		cur := stack[len(stack)-1]
		switch {
		case line == "" || strings.HasPrefix(line, "/*"):
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
