package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	styl "github.com/rohanthewiz/go-styl"
)

// runMigrate implements `styl migrate`: convert a Stylus file to modern,
// nested CSS, printing the review notes to stderr.
func runMigrate(args []string) {
	fs := flag.NewFlagSet("styl migrate", flag.ExitOnError)
	var (
		outPath string
		noNotes bool
		noVars  bool
		quiet   bool
		globals = map[string]any{}
	)
	fs.StringVar(&outPath, "o", "", "write CSS to this file instead of stdout")
	fs.BoolVar(&noNotes, "no-notes", false, "omit the inline /* styl-migrate: … */ review comments")
	fs.BoolVar(&noVars, "no-vars", false, "inline all variables instead of emitting custom properties")
	fs.BoolVar(&quiet, "q", false, "don't list the review notes on stderr")
	fs.Func("D", "define a global variable as name=value (repeatable)", func(s string) error {
		name, val, ok := strings.Cut(s, "=")
		if !ok || name == "" {
			return fmt.Errorf("expected name=value, got %q", s)
		}
		globals[name] = val
		return nil
	})
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: styl migrate [flags] <input.styl>")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if fs.NArg() != 1 {
		fs.Usage()
		os.Exit(2)
	}

	res, err := styl.MigrateFile(fs.Arg(0), styl.Options{Globals: globals},
		styl.MigrateOptions{NoNotes: noNotes, NoVars: noVars})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if !quiet && len(res.Notes) > 0 {
		for _, n := range res.Notes {
			fmt.Fprintln(os.Stderr, n)
		}
		fmt.Fprintf(os.Stderr, "%d note(s) to review\n", len(res.Notes))
	}

	if outPath == "" {
		fmt.Print(res.CSS)
		return
	}
	if err := os.WriteFile(outPath, []byte(res.CSS), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error writing output:", err)
		os.Exit(1)
	}
}
