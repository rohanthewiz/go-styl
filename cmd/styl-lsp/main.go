// Command styl-lsp is a Language Server Protocol server for Stylus (.styl),
// speaking LSP over stdin/stdout.
//
// Install:
//
//	go install github.com/rohanthewiz/go-styl/cmd/styl-lsp@latest
//
// It provides diagnostics as you type (the real compile's errors, with
// did-you-mean hints, plus lint for unused locals and duplicate properties),
// completion for variables, mixins, built-ins and CSS properties, hover with
// a variable's computed value, go-to-definition, references and rename
// across @import, signature help, document symbols, color swatches with a
// picker, and formatting (the same output as `styl fmt`).
//
// Point an editor's generic LSP client at the binary for *.styl files, e.g.
// Neovim:
//
//	vim.lsp.start({ name = "styl", cmd = { "styl-lsp" } })
//
// Flags:
//
//	-log <file>   append server logs to file (default: stderr)
//	-version      print the version and exit
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/rohanthewiz/go-styl/internal/lsp"
)

func main() {
	logPath := flag.String("log", "", "append server logs to this file (default stderr)")
	version := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *version {
		fmt.Println("styl-lsp", lsp.Version)
		return
	}

	var logw io.Writer = os.Stderr
	if *logPath != "" {
		f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintln(os.Stderr, "styl-lsp:", err)
			os.Exit(2)
		}
		defer f.Close()
		logw = f
	}
	if err := lsp.Serve(os.Stdin, os.Stdout, logw); err != nil {
		fmt.Fprintln(logw, "styl-lsp:", err)
		os.Exit(1)
	}
}
