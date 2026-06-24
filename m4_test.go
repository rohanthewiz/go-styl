package styl_test

import (
	"strings"
	"testing"

	styl "github.com/rohanthewiz/go-styl"
)

// TestM4Features exercises interpolation, @extend, and $placeholder selectors by
// compiling small snippets compressed and comparing the whole output.
func TestM4Features(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			"selector interpolation",
			"n = 2\n.col-{n}\n  width 1px",
			".col-2{width:1px}",
		},
		{
			"interpolation in loop",
			"for i in 1 2\n  .c-{i}\n    order i",
			".c-1{order:1}.c-2{order:2}",
		},
		{
			"property name interpolation",
			"side = left\na\n  margin-{side} 5px",
			"a{margin-left:5px}",
		},
		{
			"string interpolation",
			`p = col` + "\na\n  content \"x-{p}\"",
			`a{content:"x-col"}`,
		},
		{
			"lone interpolation yields value",
			"n = 7\na\n  z-index {n}",
			"a{z-index:7}",
		},
		{
			"mixed interpolation yields ident",
			"n = 7\na\n  font weight-{n}",
			"a{font:weight-7}",
		},
		{
			"extend a rule groups selectors",
			".m\n  padding 1px\n.e\n  @extend .m\n  color red",
			".m,.e{padding:1px}.e{color:red}",
		},
		{
			"extend a placeholder",
			"$box\n  padding 1px\n.a\n  @extend $box",
			".a{padding:1px}",
		},
		{
			"unextended placeholder is dropped",
			"$box\n  padding 1px\n.a\n  color red",
			".a{color:red}",
		},
		{
			"@extends alias",
			".m\n  padding 1px\n.e\n  @extends .m",
			".m,.e{padding:1px}",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := styl.Compile(c.src+"\n", styl.Options{Pretty: false})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if strings.TrimRight(got, "\n") != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestM4Import compiles a fixture that inlines a .styl partial (sharing its
// variables and mixins) and leaves a .css import as a verbatim passthrough.
func TestM4Import(t *testing.T) {
	got, err := styl.CompileFile("testdata/imports/main.styl", styl.Options{Pretty: false})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	want := `@import "reset.css";.btn{background:#3498db;border-radius:4px;color:white}`
	if strings.TrimRight(got, "\n") != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
