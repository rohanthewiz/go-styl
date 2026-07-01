package styl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rohanthewiz/serr"
)

// M7: positioned errors — every compile error carries file:line:col in its
// message and as serr attributes.

func TestM7ErrorPositions(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string // expected message prefix (position) plus fragment
	}{
		{
			name: "undefined mixin, indent syntax",
			src:  "base = 10px\n\nbody\n  width base\n  nope()\n",
			want: `<input>:5:3: undefined mixin "nope"`,
		},
		{
			name: "undefined mixin, brace syntax",
			src:  ".card {\n  color: red;\n  nope();\n}\n",
			want: `<input>:3:3: undefined mixin "nope"`,
		},
		{
			name: "bad arithmetic positioned at declaration",
			src:  "body\n  color red\n  width 4px + \"x\"\n",
			want: `<input>:3:3: cannot apply "+"`,
		},
		{
			name: "parse error positioned",
			src:  "body\n  width (1 + 2\n",
			want: `<input>:2:3: expected ')'`,
		},
		{
			name: "lexer error positioned",
			src:  "body\n  content \"oops\n",
			want: `<input>:2:3: unterminated string literal`,
		},
		{
			name: "extend without selector",
			src:  "body\n  color red\n@extend\n",
			want: `<input>:3:1: @extend requires a selector`,
		},
		{
			name: "property outside selector",
			src:  "color red\n",
			want: `<input>:1:1: property "color" must appear inside a selector`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Compile(c.src, Options{Pretty: true})
			if err == nil {
				t.Fatalf("expected error, got none")
			}
			if !strings.HasPrefix(err.Error(), c.want) {
				t.Errorf("error = %q, want prefix %q", err.Error(), c.want)
			}
		})
	}
}

func TestM7FilenameInError(t *testing.T) {
	_, err := Compile("body\n  nope()\n", Options{Filename: "app.styl"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.HasPrefix(err.Error(), "app.styl:2:3: ") {
		t.Errorf("error = %q, want app.styl:2:3 prefix", err.Error())
	}
}

func TestM7SerrAttributes(t *testing.T) {
	_, err := Compile("body\n  nope()\n", Options{Filename: "app.styl"})
	if err == nil {
		t.Fatal("expected error")
	}
	se, ok := err.(*serr.SErr)
	if !ok {
		t.Fatalf("error is %T, want *serr.SErr", err)
	}
	m := se.FieldsMap()
	if m["file"] != "app.styl" || m["line"] != "2" || m["col"] != "3" {
		t.Errorf("attributes = file[%s] line[%s] col[%s], want app.styl 2 3",
			m["file"], m["line"], m["col"])
	}
}

func TestM7DidYouMean(t *testing.T) {
	src := "button()\n  border-radius 4px\n\n.a\n  buton()\n"
	_, err := Compile(src, Options{})
	if err == nil {
		t.Fatal("expected error")
	}
	want := `did you mean "button"?`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %q, want it to contain %q", err.Error(), want)
	}
}

// An error inside a mixin body is positioned at the definition site (where the
// bad code lives), not the call site.
func TestM7MixinBodyPosition(t *testing.T) {
	src := "broken()\n  width 1px + \"x\"\n\n.a\n  broken()\n"
	_, err := Compile(src, Options{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.HasPrefix(err.Error(), "<input>:2:3: ") {
		t.Errorf("error = %q, want position 2:3 (mixin body)", err.Error())
	}
}

// Errors inside an imported file are positioned in that file.
func TestM7ImportedFilePosition(t *testing.T) {
	dir := t.TempDir()
	partial := filepath.Join(dir, "_bad.styl")
	if err := os.WriteFile(partial, []byte("// partial\n.x\n  nope()\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(dir, "main.styl")
	if err := os.WriteFile(main, []byte("@import \"_bad\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := CompileFile(main, Options{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "_bad.styl:3:3: ") {
		t.Errorf("error = %q, want position _bad.styl:3:3", err.Error())
	}
}

// Brace one-liners keep the block's source line; later statements on the same
// line may drift down but stay close.
func TestM7BraceLineFidelity(t *testing.T) {
	src := "/* header\n   comment */\n.a {\n  color: red;\n}\n\n.b {\n  nope();\n}\n"
	_, err := Compile(src, Options{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.HasPrefix(err.Error(), "<input>:8:3: ") {
		t.Errorf("error = %q, want position 8:3", err.Error())
	}
}

// Regression tests for panics/hangs found by FuzzCompile.
func TestM7FuzzRegressions(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string // fragment of the expected error ("" = must merely not crash)
	}{
		{"unterminated string in url()", `a
  b url("0`, ""},
		{"unbounded mixin recursion", "box()\n  box()\na\n  box()\n", "call depth exceeds"},
		{"exponential selector nesting", "size(n=0)\n a,x\n  size\nsize\n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Compile(c.src, Options{})
			if c.want != "" {
				if err == nil || !strings.Contains(err.Error(), c.want) {
					t.Errorf("error = %v, want it to contain %q", err, c.want)
				}
			}
		})
	}
}
