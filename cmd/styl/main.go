// Command styl compiles Stylus (.styl) files to CSS.
//
// Usage:
//
//	styl [flags] <input.styl>
//	styl gen [flags] <input.styl>
//	styl migrate [flags] <input.styl>
//	styl fmt [-w] [-l] [files...]
//
// Flags:
//
//	-o <file>     write output to file instead of stdout
//	-compress     compressed output (default is pretty/expanded)
//	-merge        merge duplicate rule bodies into selector groups
//	-sourcemap    also emit a source map (requires -o); appends sourceMappingURL
//	-D name=value define a global variable (repeatable); value is a Stylus
//	              expression, e.g. -D primary=#0af -D 'pad=2 * 8px'
//	-cssvar name  expose a root-level variable as a CSS custom property
//	              (repeatable): emits --name on :root, references become
//	              var(--name)
//
// The gen subcommand emits a Go source file of typed constants for every
// class name, element ID, keyframes name, and root-level variable in the
// stylesheet, for typo-proof selector references (b.Div(css.Card)). Wire it
// up with go:generate:
//
//	//go:generate go run github.com/rohanthewiz/go-styl/cmd/styl gen -pkg css -o css_gen.go app.styl
//
// gen flags:
//
//	-o <file>     write the Go source to file instead of stdout
//	-pkg <name>   package name for the generated file (default "css")
//	-D name=value define a global variable (repeatable), as above
//	-scoped       compile as a scoped component (styl.Component): class and
//	              keyframes constants hold the hashed names
//	-css <file>   with -scoped, also write the scoped CSS to file
//
// The migrate subcommand converts a Stylus file to modern CSS for leaving
// the preprocessor: nesting stays nested (native CSS nesting), root-level
// variables become custom properties on :root, and mixins, loops,
// conditionals and @extend are resolved in place, each flagged with a
// /* styl-migrate: … */ comment for review. The notes are also listed on
// stderr.
//
// migrate flags:
//
//	-o <file>     write the CSS to file instead of stdout
//	-no-notes     omit the inline review comments
//	-no-vars      inline all variables instead of emitting custom properties
//	-q            don't list the notes on stderr
//	-D name=value define a global variable (repeatable), as above
//
// The fmt subcommand formats Stylus source in place of gofmt's role:
// two-space indentation by nesting depth, single spaces inside lines, no
// trailing whitespace, at most one blank line in a row. Comments are kept,
// and a result that wouldn't parse to the same stylesheet is never written.
// With no files it formats stdin to stdout.
//
// fmt flags:
//
//	-w            rewrite changed files in place
//	-l            list files whose formatting differs
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	styl "github.com/rohanthewiz/go-styl"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "gen" {
		runGen(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		runMigrate(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "fmt" {
		runFmt(os.Args[2:])
		return
	}

	var (
		outPath   string
		compress  bool
		merge     bool
		sourcemap bool
		globals   = map[string]any{}
		cssVars   []string
	)
	flag.StringVar(&outPath, "o", "", "write CSS to this file instead of stdout")
	flag.BoolVar(&compress, "compress", false, "compressed output")
	flag.BoolVar(&merge, "merge", false, "merge duplicate rule bodies into selector groups")
	flag.BoolVar(&sourcemap, "sourcemap", false, "emit a source map next to the output (requires -o)")
	flag.Func("D", "define a global variable as name=value (repeatable)", func(s string) error {
		name, val, ok := strings.Cut(s, "=")
		if !ok || name == "" {
			return fmt.Errorf("expected name=value, got %q", s)
		}
		globals[name] = val
		return nil
	})
	flag.Func("cssvar", "expose a root-level variable as a CSS custom property (repeatable)", func(s string) error {
		if s == "" {
			return fmt.Errorf("expected a variable name")
		}
		cssVars = append(cssVars, s)
		return nil
	})
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: styl [flags] <input.styl>")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	in := flag.Arg(0)

	if sourcemap {
		runWithSourceMap(in, outPath, compress, merge, globals, cssVars)
		return
	}

	css, err := styl.CompileFile(in, styl.Options{
		Pretty:           !compress,
		MergeDuplicates:  merge,
		Globals:          globals,
		CustomProperties: cssVars,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if outPath == "" {
		fmt.Println(css)
		return
	}
	if err := os.WriteFile(outPath, []byte(css+"\n"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error writing output:", err)
		os.Exit(1)
	}
}

// runWithSourceMap compiles in -> outPath plus a "<outPath>.map" source map and
// appends a sourceMappingURL comment to the CSS.
func runWithSourceMap(in, outPath string, compress, merge bool, globals map[string]any, cssVars []string) {
	if outPath == "" {
		fmt.Fprintln(os.Stderr, "error: -sourcemap requires -o <file>")
		os.Exit(2)
	}
	mapPath := outPath + ".map"

	css, mapJSON, err := styl.CompileFileMap(in, styl.Options{
		Pretty:           !compress,
		MergeDuplicates:  merge,
		Globals:          globals,
		CustomProperties: cssVars,
		OutFile:          filepath.Base(outPath),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	css += "\n/*# sourceMappingURL=" + filepath.Base(mapPath) + " */\n"
	if err := os.WriteFile(outPath, []byte(css), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error writing output:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(mapPath, []byte(mapJSON), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error writing source map:", err)
		os.Exit(1)
	}
}
