package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	styl "github.com/rohanthewiz/go-styl"
)

// runGen implements `styl gen`: extract the class/ID/keyframes/variable
// manifest of a stylesheet and emit it as a Go constants file.
func runGen(args []string) {
	fs := flag.NewFlagSet("styl gen", flag.ExitOnError)
	var (
		outPath string
		pkg     string
		globals = map[string]any{}
	)
	fs.StringVar(&outPath, "o", "", "write Go source to this file instead of stdout")
	fs.StringVar(&pkg, "pkg", "css", "package name for the generated file")
	fs.Func("D", "define a global variable as name=value (repeatable)", func(s string) error {
		name, val, ok := strings.Cut(s, "=")
		if !ok || name == "" {
			return fmt.Errorf("expected name=value, got %q", s)
		}
		globals[name] = val
		return nil
	})
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: styl gen [flags] <input.styl>")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if fs.NArg() != 1 {
		fs.Usage()
		os.Exit(2)
	}

	manifest, err := styl.ExtractFile(fs.Arg(0), styl.Options{Globals: globals})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	src, err := manifest.GoSource(pkg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if outPath == "" {
		os.Stdout.Write(src)
		return
	}
	if err := os.WriteFile(outPath, src, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error writing output:", err)
		os.Exit(1)
	}
}
