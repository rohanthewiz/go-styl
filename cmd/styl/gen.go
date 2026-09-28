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
		scoped  bool
		cssOut  string
		globals = map[string]any{}
	)
	fs.StringVar(&outPath, "o", "", "write Go source to this file instead of stdout")
	fs.StringVar(&pkg, "pkg", "css", "package name for the generated file")
	fs.BoolVar(&scoped, "scoped", false, "compile as a scoped component (styl.Component): class and keyframes constants hold the hashed names")
	fs.StringVar(&cssOut, "css", "", "with -scoped, also write the scoped CSS to this file")
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

	if cssOut != "" && !scoped {
		fmt.Fprintln(os.Stderr, "error: -css requires -scoped")
		os.Exit(2)
	}

	// Both paths yield a Manifest for the same GoSource renderer; a scoped
	// one differs only in carrying the local -> hashed name map.
	var manifest *styl.Manifest
	if scoped {
		comp, err := styl.ComponentFile(fs.Arg(0), styl.Options{Globals: globals, Pretty: true})
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		manifest = comp.Manifest
		if cssOut != "" {
			if err := os.WriteFile(cssOut, []byte(comp.CSS+"\n"), 0o644); err != nil {
				fmt.Fprintln(os.Stderr, "error writing CSS:", err)
				os.Exit(1)
			}
		}
	} else {
		var err error
		manifest, err = styl.ExtractFile(fs.Arg(0), styl.Options{Globals: globals})
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
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
