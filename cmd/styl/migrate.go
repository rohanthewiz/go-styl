package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
		noComms bool
		split   bool
		tokens  string
		quiet   bool
		globals = map[string]any{}
	)
	fs.StringVar(&outPath, "o", "", "write CSS to this file instead of stdout")
	fs.BoolVar(&noNotes, "no-notes", false, "omit the inline /* styl-migrate: … */ review comments")
	fs.BoolVar(&noVars, "no-vars", false, "inline all variables instead of emitting custom properties")
	fs.BoolVar(&noComms, "no-comments", false, "drop the source's comments instead of carrying them into the CSS")
	fs.BoolVar(&split, "split", false, "write each imported .styl file as its own CSS file (needs -o <dir>)")
	fs.StringVar(&tokens, "tokens", "", "with -split, the shared custom-property file (default tokens.css)")
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
	if split && outPath == "" {
		fmt.Fprintln(os.Stderr, "error: -split writes several files; give an output directory with -o")
		os.Exit(2)
	}

	res, err := styl.MigrateFile(fs.Arg(0), styl.Options{Globals: globals},
		styl.MigrateOptions{NoNotes: noNotes, NoVars: noVars, NoComments: noComms, Split: split, TokensFile: tokens})
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

	if split {
		writeSplit(outPath, res.Files, quiet)
		return
	}
	if outPath == "" {
		fmt.Print(res.CSS)
		return
	}
	if err := writeOutput(outPath, []byte(res.CSS)); err != nil {
		fmt.Fprintln(os.Stderr, "error writing output:", err)
		os.Exit(1)
	}
}

// writeSplit writes a -split migration's files under dir, creating dir and
// subdirectories as the paths need (writeOutput). Paths come from the migration (never
// absolute, never above the output root), so they are joined as given.
func writeSplit(dir string, files []styl.MigratedFile, quiet bool) {
	for _, f := range files {
		dst := filepath.Join(dir, filepath.FromSlash(f.Path))
		if err := writeOutput(dst, []byte(f.CSS)); err != nil {
			fmt.Fprintln(os.Stderr, "error writing output:", err)
			os.Exit(1)
		}
		if !quiet {
			fmt.Fprintln(os.Stderr, "wrote", dst)
		}
	}
}
