package eval

// File-for-file migration (MigrateOptions.Split, `styl migrate -split`).
//
// A plain migration inlines every .styl import into one stylesheet. A
// project migrated that way loses its file layout, so Split keeps it:
//
//	main.styl                        main.css
//	  @import "_vars"          →       @import "tokens.css";
//	  @import "buttons"                @import "buttons.css";
//	  .page { … }                      .page { … }
//	_vars.styl (variables only)      (no file: nothing to write)
//	buttons.styl (rules)             buttons.css
//	                                 tokens.css  (:root { --… })
//
// The walk is the same as for a single sheet: an imported file still runs
// in the importer's scope, so its variables and mixins stay visible to
// everything after it. Only where the output goes changes. A root-level
// import gets an output root of its own (an mfile) and leaves an mImport
// node in the importer. Once the tree is complete (after @extend, which
// can give a partial's $placeholder rule selectors late), renderSplit
// decides each file's fate:
//
//   - a file that emits no CSS (variables, mixins, functions, comments
//     only) is not written, and its @import is dropped (noted)
//   - otherwise the mImport becomes `@import "<relative url>";`, moved to
//     the top of the importing file, since CSS ignores an @import after a
//     rule (noted when it passes one: the cascade order changes)
//   - every custom property goes to one tokens file, imported first by
//     the entry sheet
//
// Output paths mirror the source tree relative to the entry sheet's
// directory, with .styl replaced by .css. A file outside that directory
// (an include path, ../shared) goes under _external/ by base name.

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// mfile is one output file of a Split migration.
type mfile struct {
	src  string // resolved source path ("" for the tokens file)
	out  string // output path, slash-separated, relative to the output root
	root *mnode
}

// entryOut is the entry sheet's output path: its base name as .css, or
// main.css when the source has no name.
func (m *migrator) entryOut() string {
	if m.opts.Filename == "" {
		return "main.css"
	}
	return cssName(path.Base(filepath.ToSlash(m.opts.Filename)))
}

// tokensOut is the tokens file's output path. A name that would leave
// the output root (absolute, or through ..) keeps only its base name.
func (m *migrator) tokensOut() string {
	if m.opts.TokensFile == "" {
		return "tokens.css"
	}
	p := path.Clean(filepath.ToSlash(m.opts.TokensFile))
	if path.IsAbs(p) || filepath.IsAbs(m.opts.TokensFile) || p == ".." || strings.HasPrefix(p, "../") {
		p = path.Base(p)
	}
	return p
}

// cssName swaps a .styl extension for .css (any other name gets .css
// appended, so a.css.styl can't collide with a literal a.css).
func cssName(name string) string {
	return strings.TrimSuffix(name, ".styl") + ".css"
}

// newFile registers abs as a file of its own with a unique output path.
func (m *migrator) newFile(abs string) *mfile {
	out := cssName(m.relSource(abs))
	if m.outNames[out] {
		// Two sources mapping to one name (two _external/x.styl, or a
		// source named like the tokens file): number the later one.
		base := strings.TrimSuffix(out, ".css")
		for i := 2; m.outNames[out]; i++ {
			out = fmt.Sprintf("%s-%d.css", base, i)
		}
	}
	m.outNames[out] = true
	f := &mfile{src: abs, out: out, root: &mnode{kind: mAt, isRoot: true}}
	m.files = append(m.files, f)
	m.byAbs[abs] = f
	return f
}

// relSource is abs relative to the entry sheet's directory, slash-
// separated, or _external/<base> when it lies outside that directory.
func (m *migrator) relSource(abs string) string {
	base := m.opts.BaseDir
	if base == "" {
		base = "."
	}
	if m.opts.FS == nil {
		// resolveImport returns absolute OS paths; BaseDir may be relative.
		if b, err := filepath.Abs(base); err == nil {
			base = b
		}
	}
	rel, err := filepath.Rel(filepath.FromSlash(base), filepath.FromSlash(abs))
	rel = filepath.ToSlash(rel)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "_external/" + path.Base(filepath.ToSlash(abs))
	}
	return rel
}

// relURL is the @import URL from the file at output path from to the one
// at to.
func relURL(from, to string) string {
	rel, err := filepath.Rel(filepath.FromSlash(path.Dir(from)), filepath.FromSlash(to))
	if err != nil {
		return to
	}
	return filepath.ToSlash(rel)
}

// renderSplit renders the entry sheet and every imported file, returning
// the files that have something to write (the entry always does, even if
// empty, since it is what the caller asked for).
func (m *migrator) renderSplit() []MigratedFile {
	entry := &mfile{src: m.opts.Filename, out: m.entryOut(), root: m.root}
	all := append([]*mfile{entry}, m.files...)

	// Which files emit CSS. An import only counts if the file it names
	// does, so this recurses along imports (no cycles: those are errors).
	emits := map[*mfile]bool{}
	var hasCSS func(f *mfile) bool
	hasCSS = func(f *mfile) bool {
		if v, ok := emits[f]; ok {
			return v
		}
		v := false
		for _, k := range f.root.kids {
			if (k.kind == mImport && hasCSS(k.imp)) || (k.kind != mComment && k.hasOutput()) {
				v = true
				break
			}
		}
		emits[f] = v
		return v
	}

	// Place the imports of every file; top is where the entry's tokens
	// import goes.
	top := 0
	for _, f := range all {
		t := m.placeImports(f, hasCSS)
		if f == entry {
			top = t
		}
	}

	// The :root rule collects the custom properties any file reads.
	var body strings.Builder
	for _, f := range all {
		if f == entry || hasCSS(f) {
			body.WriteString(m.renderKids(f.root.kids, 0))
		}
	}
	var tokens *mfile
	if rootRule := m.rootRule(body.String()); rootRule != nil {
		tokens = &mfile{out: m.tokensOut(), root: &mnode{kind: mAt, isRoot: true, kids: []*mnode{rootRule}}}
		imp := &mnode{kind: mRaw, head: importStmt(relURL(entry.out, tokens.out))}
		entry.root.kids = append(entry.root.kids[:top:top], append([]*mnode{imp}, entry.root.kids[top:]...)...)
	}

	out := []MigratedFile{m.renderFile(entry, true)}
	if tokens != nil {
		out = append(out, MigratedFile{Path: tokens.out, CSS: m.renderKids(tokens.root.kids, 0)})
	}
	for _, f := range m.files {
		if hasCSS(f) {
			out = append(out, m.renderFile(f, false))
		}
	}
	return out
}

// placeImports rewrites f's root for CSS's import rules and returns the
// index just past its @charset and header comments, where a file's first
// @import belongs. Each mImport becomes an @import line, or is dropped
// when its file emits nothing. Every @import (these and literal ones)
// moves up, after the @charset and the header:
//
//	/* header */          /* header */
//	                      @import "b.css";   noted: b.css's rules now
//	.a { … }        →                        load before .a
//	@import "b";          .a { … }
//
// The comments directly above an @import travel with it (and are dropped
// with it). The header is the run of comments before the first rule or
// import that ends at a blank line in the source.
func (m *migrator) placeImports(f *mfile, hasCSS func(*mfile) bool) int {
	var head, charset, imports, rest, pend []*mnode
	passedRule := false // a node with output precedes this point
	// split separates pend into the header part (only before any rule or
	// import) and the part attached to the node that follows.
	split := func() (header, attached []*mnode) {
		if passedRule || len(imports) > 0 || len(head) > 0 {
			return nil, pend
		}
		cut := 0
		for i, c := range pend {
			if c.src != nil && c.src.BlankAfter {
				cut = i + 1
			}
		}
		return pend[:cut], pend[cut:]
	}
	for _, k := range f.root.kids {
		if k.kind == mComment {
			pend = append(pend, k)
			continue
		}
		header, attached := split()
		head = append(head, header...)
		pend = nil
		switch {
		case k.kind == mImport:
			ec := &execCtx{file: k.file}
			if !hasCSS(k.imp) {
				m.note(ec, k.line, k.col, "import",
					fmt.Sprintf("%s emits no CSS (only variables, mixins or functions); no %s written", m.relSource(k.imp.src), k.imp.out))
				continue // its comments go with it
			}
			line := &mnode{kind: mRaw, head: importStmt(relURL(f.out, k.imp.out)), lead: k.lead}
			if passedRule {
				n, first := m.note(ec, k.line, k.col, "hoisted",
					fmt.Sprintf("@import of %s moved to the top of %s (CSS ignores an @import after a rule); its rules now load before the ones above it", k.imp.out, f.out))
				m.lead(line, n, first)
			}
			imports = append(append(imports, attached...), line)
		case k.kind == mRaw && strings.HasPrefix(k.head, "@charset"):
			charset = append(append(charset, attached...), k)
		case k.kind == mRaw && strings.HasPrefix(k.head, "@import"):
			imports = append(append(imports, attached...), k)
		default:
			rest = append(append(rest, attached...), k)
			if k.hasOutput() {
				passedRule = true
			}
		}
	}
	// Comments after the last node: a file of comments only keeps them as
	// its header, otherwise they close the file.
	header, attached := split()
	head = append(head, header...)
	rest = append(rest, attached...)

	kids := make([]*mnode, 0, len(f.root.kids))
	kids = append(append(append(kids, charset...), head...), imports...)
	top := len(charset) + len(head)
	f.root.kids = append(kids, rest...)
	return top
}

// renderFile prints one output file, with the migration header when notes
// concern it (any note, for the entry sheet, as in a plain migration).
func (m *migrator) renderFile(f *mfile, entry bool) MigratedFile {
	body := m.renderKids(f.root.kids, 0)
	if m.opts.Notes {
		noted := entry && len(m.notes) > 0
		for _, n := range m.notes {
			noted = noted || n.File == f.src
		}
		if noted {
			src := f.src
			if src == "" {
				src = "Stylus source"
			}
			body = fmt.Sprintf("/* Migrated from %s by styl migrate. Review each styl-migrate note. */\n\n", commentSafe(src)) + body
		}
	}
	return MigratedFile{Path: f.out, Source: f.src, CSS: body}
}
