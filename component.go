package styl

import (
	"crypto/sha256"
	"encoding/base32"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/eval"
	"github.com/rohanthewiz/go-styl/internal/parser"
)

// Scoped is a component stylesheet compiled by Component: CSS whose class and
// @keyframes names were made unique to the component, plus the mapping from
// the names the author wrote to the names in the CSS.
type Scoped struct {
	// CSS is the compiled stylesheet with scoped names.
	CSS string
	// Names maps each local class or @keyframes name to its scoped form
	// ("card" -> "card_k3xqa2mf"). Classes written inside :global(...) have
	// no entry.
	Names map[string]string
	// Manifest lists the sheet's names in local form, with Manifest.Scoped
	// set to Names, so Manifest.GoSource emits constants holding the scoped
	// values (see `styl gen -scoped`).
	Manifest *Manifest
}

// Class returns the space-separated scoped form of the given local class
// names, ready for an HTML class attribute:
//
//	b.Div("class", card.Class("card", "card--active"))
//
// A name the component does not define passes through unchanged, so global
// utility classes can be mixed in. For typo-checked references, generate
// constants with `styl gen -scoped` instead.
func (s *Scoped) Class(locals ...string) string {
	out := make([]string, len(locals))
	for i, l := range locals {
		if scoped, ok := s.Names[l]; ok {
			out[i] = scoped
		} else {
			out[i] = l
		}
	}
	return strings.Join(out, " ")
}

// Component compiles Stylus source as a scoped component stylesheet — CSS
// Modules for server-rendered Go, with no build step. Every class name and
// @keyframes name gets a suffix derived from the source, so two components
// can both style ".title" or animate "fade" without colliding:
//
//	.card            .card_k3xqa2mf
//	  .title    =>   .card_k3xqa2mf .title_k3xqa2mf
//	  animation fade 1s     animation: fade_k3xqa2mf 1s
//
// Element IDs, type selectors, and attribute selectors stay global, as do
// names wrapped in ":global(...)" (the wrapper is removed:
// ".card :global(.is-open)" -> ".card_k3xqa2mf .is-open"). In Stylus a nested
// selector that starts with ':' attaches to its parent like a pseudo-class,
// so write "& :global(.x)" for a descendant.
//
// The suffix is a hash of src alone — not of Options, so per-request Globals
// (runtime theming) never change the names, and constants generated at build
// time by `styl gen -scoped` from the same file match a runtime Component of
// it. Editing the source changes every name; regenerate. Options is honored as
// in Compile.
func Component(src string, opts Options) (*Scoped, error) {
	sb, err := opts.sandbox(len(src))
	if err != nil {
		return nil, compileErr(err, opts.Filename)
	}
	sheet, err := parser.Parse(src)
	if err != nil {
		return nil, compileErr(err, opts.Filename)
	}
	suffix := "_" + scopeHash(src)
	out, em, renamed, err := eval.EvaluateScoped(sheet, eval.Options{
		Pretty:           opts.Pretty,
		MergeDuplicates:  opts.MergeDuplicates,
		Filename:         opts.Filename,
		BaseDir:          opts.baseDir(),
		IncludePaths:     opts.IncludePaths,
		FS:               opts.FS,
		Globals:          opts.Globals,
		CustomProperties: opts.CustomProperties,
		Sandbox:          sb,
	}, func(local string) string { return local + suffix })
	if err != nil {
		return nil, compileErr(err, opts.Filename)
	}
	m := manifestFrom(em)
	m.Scoped = renamed
	return &Scoped{CSS: out, Names: renamed, Manifest: m}, nil
}

// ComponentFile compiles the Stylus file at path (from Options.FS when set)
// as a scoped component. The path is recorded as Manifest.Source; it does not
// feed the scope hash, so the names are the same however the file is reached.
func ComponentFile(path string, opts Options) (*Scoped, error) {
	data, err := readSource(path, opts)
	if err != nil {
		return nil, err
	}
	if opts.Filename == "" {
		opts.Filename = path
	}
	s, err := Component(string(data), opts)
	if err != nil {
		return nil, err
	}
	s.Manifest.Source = path
	return s, nil
}

// scopeHash returns the component suffix for a source text: the first 40 bits
// of its SHA-256 as 8 lower-case base32 characters. Base32's alphabet (a-z,
// 2-7) is valid in CSS identifiers after the "_" separator, and 40 bits keeps
// the chance of two components in one app colliding negligible (~1e-6 at 1000
// components) while staying short enough to read in dev tools.
//
// The raw source text is hashed rather than a file path: go generate and the
// running app often reach the same file by different paths (relative vs
// embedded), and the names must agree between them.
func scopeHash(src string) string {
	sum := sha256.Sum256([]byte(src))
	return strings.ToLower(base32.StdEncoding.EncodeToString(sum[:5]))
}
