// Package styl is a pure-Go compiler for the Stylus (.styl) CSS preprocessor.
//
// It parses Stylus source into an AST, evaluates it (resolving variables,
// arithmetic, and built-in functions with lexical scoping), and renders CSS.
// Variables are inlined at compile time, matching Stylus semantics.
package styl

import (
	"io"
	"os"
	"path/filepath"

	"github.com/rohanthewiz/go-styl/internal/eval"
	"github.com/rohanthewiz/go-styl/internal/parser"
)

// Options configures compilation.
type Options struct {
	// Pretty renders expanded, human-readable CSS. When false, output is compressed.
	Pretty bool
	// MergeDuplicates folds rules with identical declaration bodies into a single
	// comma-separated selector group (scarlet's extra-compression pass). Off by
	// default, since standard Stylus does not do this.
	MergeDuplicates bool
	// IncludePaths lists additional directories searched for @import.
	IncludePaths []string
	// BaseDir is the directory that relative @import paths resolve against. When
	// empty it defaults to the directory of Filename, or the process working
	// directory if Filename is also unset.
	BaseDir string
	// Filename is the source path, used in error messages and to derive BaseDir.
	Filename string
}

// Compile compiles Stylus source to CSS.
func Compile(src string, opts Options) (string, error) {
	sheet, err := parser.Parse(src)
	if err != nil {
		return "", err
	}
	baseDir := opts.BaseDir
	if baseDir == "" && opts.Filename != "" {
		baseDir = filepath.Dir(opts.Filename)
	}
	return eval.Evaluate(sheet, eval.Options{
		Pretty:          opts.Pretty,
		MergeDuplicates: opts.MergeDuplicates,
		BaseDir:         baseDir,
		IncludePaths:    opts.IncludePaths,
	})
}

// CompileReader compiles Stylus source read from r.
func CompileReader(r io.Reader, opts Options) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return Compile(string(data), opts)
}

// CompileFile compiles the Stylus file at path.
func CompileFile(path string, opts Options) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if opts.Filename == "" {
		opts.Filename = path
	}
	return Compile(string(data), opts)
}
