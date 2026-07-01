// Package styl is a pure-Go compiler for the Stylus (.styl) CSS preprocessor.
//
// It parses Stylus source into an AST, evaluates it (resolving variables,
// arithmetic, and built-in functions with lexical scoping), and renders CSS.
// Variables are inlined at compile time, matching Stylus semantics.
package styl

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/rohanthewiz/go-styl/internal/diag"
	"github.com/rohanthewiz/go-styl/internal/eval"
	"github.com/rohanthewiz/go-styl/internal/parser"
	"github.com/rohanthewiz/serr"
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
	// FS, when set, is the filesystem that CompileFile/CompileFileMap read from
	// and that @import resolves through (e.g. an embed.FS), instead of the OS
	// filesystem. Paths (Filename, BaseDir, IncludePaths, imports) are then
	// slash-separated fs paths; a leading '/' on an import means the FS root.
	FS fs.FS
	// Filename is the source path, used in error messages and to derive BaseDir.
	Filename string
	// OutFile is the generated CSS filename recorded in a source map's "file"
	// field (optional; used by CompileMap/CompileFileMap and Build).
	OutFile string
	// SourceMap asks Build/BuildFile to also produce a source map.
	SourceMap bool
}

// Result is the outcome of a Build: the CSS, the optional source map, and the
// source files the build read, for cache invalidation.
type Result struct {
	CSS string
	// Map is the Source Map v3 JSON document; "" unless Options.SourceMap.
	Map string
	// Deps lists the resolved path of every inlined @import in encounter order
	// (BuildFile prepends the input file itself). Paths are absolute OS paths,
	// or fs paths when Options.FS is set.
	Deps []string
}

// Compile compiles Stylus source to CSS.
//
// Errors are positioned — their message begins with "file:line:col:" (the file
// falls back to "<input>" when Options.Filename is unset) — and are wrapped
// with serr, carrying "file", "line", and "col" as structured attributes for
// serr-aware loggers.
func Compile(src string, opts Options) (string, error) {
	sheet, err := parser.Parse(src)
	if err != nil {
		return "", compileErr(err, opts.Filename)
	}
	out, err := eval.Evaluate(sheet, eval.Options{
		Pretty:          opts.Pretty,
		MergeDuplicates: opts.MergeDuplicates,
		Filename:        opts.Filename,
		BaseDir:         opts.baseDir(),
		IncludePaths:    opts.IncludePaths,
		FS:              opts.FS,
	})
	if err != nil {
		return "", compileErr(err, opts.Filename)
	}
	return out, nil
}

// Build compiles Stylus source like Compile but returns a Result carrying the
// import dependency list and, when Options.SourceMap is set, a source map.
func Build(src string, opts Options) (Result, error) {
	sheet, err := parser.Parse(src)
	if err != nil {
		return Result{}, compileErr(err, opts.Filename)
	}
	source := opts.Filename
	if source == "" {
		source = "input.styl"
	}
	cssOut, mapJSON, deps, err := eval.EvaluateFull(sheet, eval.Options{
		Pretty:          opts.Pretty,
		MergeDuplicates: opts.MergeDuplicates,
		Filename:        opts.Filename,
		BaseDir:         opts.baseDir(),
		IncludePaths:    opts.IncludePaths,
		FS:              opts.FS,
		SourceMap:       opts.SourceMap,
		SourceFile:      source,
		SourceContent:   src,
		OutFile:         opts.OutFile,
	})
	if err != nil {
		return Result{}, compileErr(err, opts.Filename)
	}
	return Result{CSS: cssOut, Map: mapJSON, Deps: deps}, nil
}

// BuildFile compiles the Stylus file at path (from Options.FS when set),
// returning a Result whose Deps include path itself.
func BuildFile(path string, opts Options) (Result, error) {
	data, err := readSource(path, opts)
	if err != nil {
		return Result{}, err
	}
	if opts.Filename == "" {
		opts.Filename = path
	}
	res, err := Build(string(data), opts)
	if err != nil {
		return Result{}, err
	}
	res.Deps = append([]string{path}, res.Deps...)
	return res, nil
}

// baseDir resolves the effective import base directory: BaseDir when set,
// otherwise the directory of Filename (slash-separated in FS mode).
func (o Options) baseDir() string {
	if o.BaseDir != "" || o.Filename == "" {
		return o.BaseDir
	}
	if o.FS != nil {
		return path.Dir(o.Filename)
	}
	return filepath.Dir(o.Filename)
}

// compileErr finishes an internal compile error for the public API: positioned
// errors get the top-level filename filled in (when still unknown) and are
// wrapped with serr so file/line/col are available as structured attributes.
func compileErr(err error, file string) error {
	if err == nil {
		return nil
	}
	err = diag.SetFile(err, file)
	var de *diag.Error
	if errors.As(err, &de) {
		return serr.Wrap(err,
			"file", de.File,
			"line", strconv.Itoa(de.Line),
			"col", strconv.Itoa(de.Col))
	}
	return serr.Wrap(err, "file", file)
}

// CompileMap compiles Stylus source to CSS and also returns a Source Map v3
// document (JSON) mapping output positions back to the source. The original
// source is embedded in the map (sourcesContent) so it is self-contained.
func CompileMap(src string, opts Options) (cssOut, mapJSON string, err error) {
	sheet, err := parser.Parse(src)
	if err != nil {
		return "", "", compileErr(err, opts.Filename)
	}
	source := opts.Filename
	if source == "" {
		source = "input.styl"
	}
	cssOut, mapJSON, err = eval.EvaluateMap(sheet, eval.Options{
		Pretty:          opts.Pretty,
		MergeDuplicates: opts.MergeDuplicates,
		Filename:        opts.Filename,
		BaseDir:         opts.baseDir(),
		IncludePaths:    opts.IncludePaths,
		FS:              opts.FS,
		SourceFile:      source,
		SourceContent:   src,
		OutFile:         opts.OutFile,
	})
	if err != nil {
		return "", "", compileErr(err, opts.Filename)
	}
	return cssOut, mapJSON, nil
}

// CompileFileMap compiles the Stylus file at path, returning CSS and its source
// map. Filename (used for the map's "sources") defaults to path. When
// Options.FS is set the file is read from it instead of the OS filesystem.
func CompileFileMap(path string, opts Options) (cssOut, mapJSON string, err error) {
	data, err := readSource(path, opts)
	if err != nil {
		return "", "", err
	}
	if opts.Filename == "" {
		opts.Filename = path
	}
	return CompileMap(string(data), opts)
}

// readSource reads a top-level source file from Options.FS when set, otherwise
// from the OS filesystem.
func readSource(path string, opts Options) ([]byte, error) {
	if opts.FS != nil {
		return fs.ReadFile(opts.FS, path)
	}
	return os.ReadFile(path)
}

// CompileReader compiles Stylus source read from r.
func CompileReader(r io.Reader, opts Options) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return Compile(string(data), opts)
}

// CompileFile compiles the Stylus file at path. When Options.FS is set the
// file is read from it instead of the OS filesystem.
func CompileFile(path string, opts Options) (string, error) {
	data, err := readSource(path, opts)
	if err != nil {
		return "", err
	}
	if opts.Filename == "" {
		opts.Filename = path
	}
	return Compile(string(data), opts)
}
