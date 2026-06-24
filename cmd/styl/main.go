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
//	-sourcemap    also emit a source map (requires -o); appends sourceMappingURL
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	styl "github.com/rohanthewiz/go-styl"
)

func main() {
	var (
		outPath   string
		compress  bool
		merge     bool
		sourcemap bool
	)
	flag.StringVar(&outPath, "o", "", "write CSS to this file instead of stdout")
	flag.BoolVar(&compress, "compress", false, "compressed output")
	flag.BoolVar(&merge, "merge", false, "merge duplicate rule bodies into selector groups")
	flag.BoolVar(&sourcemap, "sourcemap", false, "emit a source map next to the output (requires -o)")
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
		runWithSourceMap(in, outPath, compress, merge)
		return
	}

	css, err := styl.CompileFile(in, styl.Options{
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

// runWithSourceMap compiles in -> outPath plus a "<outPath>.map" source map and
// appends a sourceMappingURL comment to the CSS.
func runWithSourceMap(in, outPath string, compress, merge bool) {
	if outPath == "" {
		fmt.Fprintln(os.Stderr, "error: -sourcemap requires -o <file>")
		os.Exit(2)
	}
	mapPath := outPath + ".map"

	css, mapJSON, err := styl.CompileFileMap(in, styl.Options{
		Pretty:          !compress,
		MergeDuplicates: merge,
		OutFile:         filepath.Base(outPath),
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
