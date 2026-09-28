package parser

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rohanthewiz/go-styl/internal/ast"
)

// stmtSliceType is the type of every statement list in the AST (bodies,
// branches, blocks), which is where comments can appear.
var stmtSliceType = reflect.TypeOf([]ast.Stmt(nil))

// dropComments removes every *ast.Comment from the tree in place and
// returns them in walk order (which is source order within a body).
func dropComments(v reflect.Value) []*ast.Comment {
	var out []*ast.Comment
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			out = append(out, dropComments(v.Elem())...)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			out = append(out, dropComments(v.Field(i))...)
		}
	case reflect.Slice:
		if v.Type() == stmtSliceType && v.CanSet() {
			kept := reflect.MakeSlice(stmtSliceType, 0, v.Len())
			for i := 0; i < v.Len(); i++ {
				if c, ok := v.Index(i).Interface().(*ast.Comment); ok {
					out = append(out, c)
					continue
				}
				kept = reflect.Append(kept, v.Index(i))
			}
			if kept.Len() == 0 {
				kept = reflect.Zero(stmtSliceType) // Parse leaves empty bodies nil
			}
			v.Set(kept)
		}
		for i := 0; i < v.Len(); i++ {
			out = append(out, dropComments(v.Index(i))...)
		}
	}
	return out
}

// checkCommentParse asserts the ParseWithComments invariants for src:
// it fails exactly when Parse fails, every comment scanComments finds is
// in the tree exactly once, and without its comments the tree is Parse's.
func checkCommentParse(t *testing.T, name, src string) {
	t.Helper()
	plain, perr := Parse(src)
	withC, cerr := ParseWithComments(src)
	if (perr == nil) != (cerr == nil) {
		t.Fatalf("%s: Parse err %v, ParseWithComments err %v", name, perr, cerr)
	}
	if perr != nil {
		return
	}
	got := dropComments(reflect.ValueOf(withC))
	if want := len(scanComments(src)); len(got) != want {
		t.Errorf("%s: %d comments in the tree, source has %d", name, len(got), want)
	}
	if !sameAST(plain, withC) {
		t.Errorf("%s: comments changed the parsed structure", name)
	}
}

func TestParseWithCommentsCorpus(t *testing.T) {
	var files []string
	for _, pat := range []string{"../../testdata/*.styl", "../../testdata/*/*.styl", "../../examples/*.styl", "../../examples/*/*.styl", "../../difftest/corpus/*.styl", "../../difftest/corpus/*/*.styl"} {
		m, _ := filepath.Glob(pat)
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Fatal("no fixtures found")
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		checkCommentParse(t, f, string(data))
	}
}

// commentCases cover each attachment rule, in both syntaxes.
var commentCases = []string{
	"// head\n\n.a\n  color red // why\n  // closing\n.b\n  x 1\n",
	".a\n  // first\n  color red\n",
	".a\n  // only a comment\n.b\n  x 1\n",
	"a = 1 // one\n// b's doc\nb = 2\n",
	".a,\n// between\n.b\n  x 1\n",
	".a\n  if x\n    y 1\n  // before else\n  else\n    y 2 // two\n  // end\n",
	".a {\n  color: red; // why\n  /* multi\n     line */\n  x: 1;\n  // closing\n}\n",
	".a { x: 1 } // same line\n.b { y: 2 }\n",
	"theme = {\n  a: 1, // in object\n  b: 2\n}\n.x\n  y theme.a\n",
	"// nothing else\n",
	"/* unterminated\n.a\n  x 1\n",
	"url = 'http://x' // after string\n.a\n  b url(http://x.y/z) // after url\n",
	"m()\n  // in mixin\n  x 1\n.a\n  m()\n",
	".a\n  x 1; // semi\n  ; // lone semi\n",
	"; /*", // a block whose only statement is a dropped `;`
}

func TestParseWithCommentsCases(t *testing.T) {
	for _, src := range commentCases {
		checkCommentParse(t, src, src)
	}
}

// TestCommentPlacement pins where comments land in the tree.
func TestCommentPlacement(t *testing.T) {
	sheet, err := ParseWithComments("// head\n\n.a\n  // first\n  color red // why\n  // closing\n.b\n  x 1\n")
	if err != nil {
		t.Fatal(err)
	}
	st := sheet.Statements
	if len(st) != 3 {
		t.Fatalf("want head comment, .a, .b; got %d statements", len(st))
	}
	head, ok := st[0].(*ast.Comment)
	if !ok || head.Text != " head" || head.Inline || !head.BlankAfter {
		t.Errorf("head = %#v", st[0])
	}
	var texts []string
	for _, s := range st[1].(*ast.RuleSet).Body {
		switch n := s.(type) {
		case *ast.Comment:
			texts = append(texts, strings.TrimSpace(n.Text))
		case *ast.Declaration:
			texts = append(texts, n.Property)
		}
	}
	if got := strings.Join(texts, ","); got != "first,why,color,closing" {
		t.Errorf(".a body = %s", got)
	}
}

// FuzzParseWithComments checks the invariants of checkCommentParse on
// arbitrary input.
func FuzzParseWithComments(f *testing.F) {
	for _, c := range commentCases {
		f.Add(c)
	}
	f.Fuzz(func(t *testing.T, src string) {
		checkCommentParse(t, "fuzz", src)
	})
}
