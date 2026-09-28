package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	styl "github.com/rohanthewiz/go-styl"
)

// runFmt implements `styl fmt [-w] [-l] [files...]`, modeled on gofmt: with
// no files it formats stdin to stdout; with files it prints each formatted
// file, or with -w rewrites changed files in place, and with -l lists the
// files whose formatting differs. The exit status is 1 if any file failed
// to parse or format.
func runFmt(args []string) {
	fs := flag.NewFlagSet("fmt", flag.ExitOnError)
	write := fs.Bool("w", false, "write the result back to each file instead of stdout")
	list := fs.Bool("l", false, "list files whose formatting differs")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: styl fmt [-w] [-l] [file.styl ...]")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	if fs.NArg() == 0 {
		if *write {
			fmt.Fprintln(os.Stderr, "error: -w needs file arguments")
			os.Exit(2)
		}
		src, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		out, err := styl.Format(string(src))
		if err != nil {
			fmt.Fprintln(os.Stderr, "error: <stdin>:", err)
			os.Exit(1)
		}
		if *list {
			if out != string(src) {
				fmt.Println("<stdin>")
			}
			return
		}
		fmt.Print(out)
		return
	}

	failed := false
	for _, path := range fs.Args() {
		src, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			failed = true
			continue
		}
		out, err := styl.Format(string(src))
		if err != nil {
			// Parse errors from Format carry "<input>:line:col"; name the file.
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
			failed = true
			continue
		}
		changed := !bytes.Equal(src, []byte(out))
		if *list && changed {
			fmt.Println(path)
		}
		if *write {
			if changed {
				if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
					fmt.Fprintln(os.Stderr, "error:", err)
					failed = true
				}
			}
			continue
		}
		if !*list {
			fmt.Print(out)
		}
	}
	if failed {
		os.Exit(1)
	}
}
