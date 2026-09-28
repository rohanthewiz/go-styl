package styl

import (
	"fmt"

	"github.com/rohanthewiz/go-styl/internal/eval"
	"github.com/rohanthewiz/go-styl/internal/parser"
)

// MigrateOptions configures Migrate beyond the shared Options.
type MigrateOptions struct {
	// NoNotes leaves out the inline `/* styl-migrate: … */` review comments.
	// The notes are still returned in MigrateResult.Notes.
	NoNotes bool
	// NoVars inlines every variable, as Compile does, instead of exposing
	// root-level variables as CSS custom properties.
	NoVars bool
	// NoComments drops the source's comments. By default each one is
	// carried to the matching place in the CSS, a `//` comment as
	// `/* … */`; comments directly above a root variable move with it
	// into :root.
	NoComments bool
	// Split migrates file for file: each .styl file imported at the top
	// level of a sheet becomes a CSS file of its own, and its @import an
	// `@import "x.css";` (moved to the top of the importing file, as CSS
	// requires). A file that emits no CSS (variables and mixins only) is
	// not written and its import is dropped. The :root custom properties
	// go to a shared tokens file that the entry sheet imports first.
	// Imports inside a rule or at-rule are still inlined. The output is
	// in MigrateResult.Files.
	Split bool
	// TokensFile names the tokens file under Split ("tokens.css" when
	// empty), relative to the output root.
	TokensFile string
}

// MigratedFile is one output file of a Split migration.
type MigratedFile struct {
	// Path is relative to the output root, slash-separated. Paths mirror
	// the sources relative to the entry sheet's directory (x.styl →
	// x.css); a source outside it goes under _external/. The files'
	// @import URLs are relative to each other.
	Path string
	// Source is the Stylus file the CSS came from ("" for the tokens
	// file).
	Source string
	CSS    string
}

// MigrateNote marks a place where the migration made a judgment call that
// a person should review: an expanded mixin, an unrolled loop, a value
// computed from a variable, a rule moved out of nesting, …
type MigrateNote struct {
	File      string
	Line, Col int
	// Kind is a short category: mixin, loop, condition, frozen, hoisted,
	// extend, import, variable, prefix.
	Kind string
	Msg  string
}

// String formats the note as "file:line:col: kind: msg".
func (n MigrateNote) String() string {
	file := n.File
	if file == "" {
		file = "<input>"
	}
	return fmt.Sprintf("%s:%d:%d: %s: %s", file, n.Line, n.Col, n.Kind, n.Msg)
}

// MigrateResult is the output of Migrate.
type MigrateResult struct {
	// CSS is modern, hand-editable CSS that keeps the source's structure.
	CSS string
	// Notes lists every place that needs review, in source order of
	// discovery.
	Notes []MigrateNote
	// Deps lists the resolved path of every inlined .styl @import.
	Deps []string
	// Files is set under MigrateOptions.Split: the entry sheet first (its
	// CSS is also in CSS), then the tokens file when any custom property
	// is used, then each imported file that emits CSS, in import order.
	Files []MigratedFile
}

// Migrate converts Stylus source into modern CSS for teams leaving the
// preprocessor behind. Unlike Compile, it keeps the stylesheet's shape:
//
//   - nesting is kept as native CSS nesting (`&` included; a bare nested
//     `:hover` becomes `&:hover`)
//   - root-level variables become custom properties on :root, and direct
//     references become var(--name); values computed from a variable
//     (darken(primary, 10%), base * 2) are computed now and noted
//   - mixins, functions, loops and conditionals are expanded in place,
//     each marked with a `/* styl-migrate: … */` comment
//   - @extend adds the extending selectors to the target rule's list
//   - .styl imports are inlined; CSS imports pass through
//   - source comments are kept in place (see MigrateOptions.NoComments)
//   - with MigrateOptions.Split, imported files stay separate CSS files
//
// Selectors CSS nesting can't express — `&` concatenation such as BEM
// `&__elem`, or nesting under a pseudo-element — are written out in full
// after the enclosing block, and noted.
//
// Options supplies import resolution (Filename, BaseDir, IncludePaths,
// FS), Globals and Warn; output-shaping fields (Pretty, MergeDuplicates,
// CustomProperties, SourceMap) don't apply.
func Migrate(src string, opts Options, mo MigrateOptions) (MigrateResult, error) {
	sb, err := opts.sandbox(len(src))
	if err != nil {
		return MigrateResult{}, compileErr(err, opts.Filename)
	}
	parse := parser.ParseWithComments
	if mo.NoComments {
		parse = parser.Parse
	}
	sheet, err := parse(src)
	if err != nil {
		return MigrateResult{}, compileErr(err, opts.Filename)
	}
	res, err := eval.Migrate(sheet, eval.MigrateOptions{
		Options: eval.Options{
			Pretty:       true,
			Filename:     opts.Filename,
			BaseDir:      opts.baseDir(),
			IncludePaths: opts.IncludePaths,
			FS:           opts.FS,
			Globals:      opts.Globals,
			Warn:         opts.Warn,
			Sandbox:      sb,
		},
		Notes:      !mo.NoNotes,
		NoVars:     mo.NoVars,
		NoComments: mo.NoComments,
		Split:      mo.Split,
		TokensFile: mo.TokensFile,
	})
	if err != nil {
		return MigrateResult{}, compileErr(err, opts.Filename)
	}
	out := MigrateResult{CSS: res.CSS, Deps: res.Deps}
	for _, n := range res.Notes {
		out.Notes = append(out.Notes, MigrateNote(n))
	}
	for _, f := range res.Files {
		out.Files = append(out.Files, MigratedFile(f))
	}
	return out, nil
}

// MigrateFile migrates the Stylus file at path (from Options.FS when set).
// Deps include path itself.
func MigrateFile(path string, opts Options, mo MigrateOptions) (MigrateResult, error) {
	data, err := readSource(path, opts)
	if err != nil {
		return MigrateResult{}, err
	}
	if opts.Filename == "" {
		opts.Filename = path
	}
	res, err := Migrate(string(data), opts, mo)
	if err != nil {
		return MigrateResult{}, err
	}
	res.Deps = append([]string{path}, res.Deps...)
	return res, nil
}
