// Command styl compiles Stylus (.styl) files to CSS.
//
// Usage:
//
//	styl [flags] <input.styl>
//
// Flags:
//
//	-o <file>     write output to file instead of stdout
//	-compress     compressed output (default is pretty/expanded)
//	-merge        merge duplicate rule bodies into selector groups
package main

import (
	"flag"
	"fmt"
	"os"

	styl "github.com/rohanthewiz/go-styl"
)

func main() {
	var (
		outPath  string
		compress bool
		merge    bool
	)
	flag.StringVar(&outPath, "o", "", "write CSS to this file instead of stdout")
	flag.BoolVar(&compress, "compress", false, "compressed output")
	flag.BoolVar(&merge, "merge", false, "merge duplicate rule bodies into selector groups")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: styl [flags] <input.styl>")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	css, err := styl.CompileFile(flag.Arg(0), styl.Options{
		Pretty:          !compress,
		MergeDuplicates: merge,
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
