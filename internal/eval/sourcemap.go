package eval

import (
	"path"
	"path/filepath"

	"github.com/rohanthewiz/go-styl/internal/css"
)

// Multi-file source maps.
//
// Every position the evaluator emits carries the file key of the statement
// that produced it (css.Pos.File): Options.Filename for the entry, and an
// @import's resolved path for an imported file. Mixin and function bodies keep
// the key of the file that defined them (Closure.File), so a declaration a
// mixin emits maps into the partial the mixin lives in, not into the caller.
//
// The map lists the entry first and each imported file once something maps
// into it. Keys are not good "sources" names: in OS mode they are absolute
// paths, while the entry is named as the caller gave it (often relative). So
// each import is named relative to the entry file, in the entry name's terms:
//
//	entry "styles/app.styl", import /cwd/styles/partials/_btn.styl
//	  → rel(/cwd/styles, …) = "partials/_btn.styl"
//	  → join(dir("styles/app.styl"), …) = "styles/partials/_btn.styl"
//
// An absolute entry name gives absolute import names; a relative one gives
// names relative to the same base. Names are slash-separated (they are URLs).
//
// Relative to the map (Options.MapFile). The v3 spec resolves each "sources"
// entry against the map's own URL, so the names above are only right when the
// map sits where the entry name is relative to (usually: the current
// directory). When the caller says where the map goes, every source — the
// entry included — is instead named from the map's directory:
//
//	map out/app.css.map, entry styles/app.styl, import styles/_btn.styl
//	  → "../styles/app.styl", "../styles/_btn.styl"
//
// Both sides are brought to the evaluator's key form first (absolute OS
// paths, or fs paths under FS), so a relative MapFile and an absolute key
// compare correctly. If no relative path exists (different Windows volumes)
// the default name is kept.

// importedSource is one file read by @import: its evaluator key and text.
type importedSource struct {
	key     string
	content string
}

// noteSource records an imported file's text for sourcesContent, once per
// key (a plain @import may pull the same file in more than once).
func (ev *evaluator) noteSource(key string, data []byte) {
	if !ev.opts.SourceMap {
		return
	}
	for _, s := range ev.sources {
		if s.key == key {
			return
		}
	}
	ev.sources = append(ev.sources, importedSource{key: key, content: string(data)})
}

// newSourceMap creates the render-time collector with the entry as source 0
// and every imported file registered under its key.
func (ev *evaluator) newSourceMap() *css.SourceMap {
	entryName := ev.opts.SourceFile
	if name, ok := ev.mapRelName(ev.entryLoc()); ok {
		entryName = name
	}
	sm := css.NewSourceMap(ev.opts.OutFile, entryName, ev.opts.SourceContent)
	sm.SetEntry(ev.opts.Filename)
	for _, s := range ev.sources {
		name, ok := ev.mapRelName(s.key)
		if !ok {
			name = ev.sourceName(s.key)
		}
		sm.AddSource(s.key, name, s.content)
	}
	return sm
}

// entryLoc returns where the entry file sits, in key form (an absolute OS
// path, or an fs path under FS) so it compares with import keys. A compile
// of a bare string has no Filename; it is taken to be "input.styl" in the
// directory its imports resolve against (BaseDir), matching sourceName.
func (ev *evaluator) entryLoc() string {
	if ev.opts.FS != nil {
		if ev.opts.Filename != "" {
			return path.Clean(ev.opts.Filename)
		}
		return path.Join(ev.opts.BaseDir, "input.styl")
	}
	loc := ev.opts.Filename
	if loc == "" {
		loc = filepath.Join(ev.opts.BaseDir, "input.styl")
	}
	if abs, err := filepath.Abs(loc); err == nil {
		return abs
	}
	return loc
}

// mapRelName names the source at loc (key form) relative to the directory of
// Options.MapFile. ok is false when no MapFile is set or no relative path
// exists, and the caller keeps its default name.
func (ev *evaluator) mapRelName(loc string) (name string, ok bool) {
	if ev.opts.MapFile == "" {
		return "", false
	}
	var mapDir string
	if ev.opts.FS != nil {
		// fs paths: lexical, as in sourceName. FromSlash only lets
		// filepath.Rel parse them.
		mapDir = filepath.FromSlash(path.Dir(path.Clean(ev.opts.MapFile)))
		loc = filepath.FromSlash(loc)
	} else {
		abs, err := filepath.Abs(filepath.Dir(ev.opts.MapFile))
		if err != nil {
			return "", false
		}
		mapDir = abs
	}
	r, err := filepath.Rel(mapDir, loc)
	if err != nil {
		return "", false
	}
	return filepath.ToSlash(r), true
}

// sourceName names an imported file (by its key) for the map's "sources",
// relative to the entry as described above.
func (ev *evaluator) sourceName(key string) string {
	entryName := ev.opts.SourceFile
	if entryName == "" {
		entryName = "input.styl"
	}
	// entryDir is where the entry actually lives: its Filename's directory,
	// or, for a compile of a bare string, the directory imports resolve
	// against (BaseDir), which is where such an entry is taken to sit.
	entryDir := ev.opts.BaseDir
	if ev.opts.Filename != "" {
		if ev.opts.FS != nil {
			entryDir = path.Dir(ev.opts.Filename)
		} else {
			entryDir = filepath.Dir(ev.opts.Filename)
		}
	}
	if entryDir == "" {
		entryDir = "."
	}

	var rel string
	if ev.opts.FS != nil {
		// fs paths are relative to the FS root, as the entry's is, so a
		// lexical Rel is exact (FromSlash only lets filepath.Rel parse them).
		r, err := filepath.Rel(filepath.FromSlash(path.Clean(entryDir)), filepath.FromSlash(key))
		if err != nil {
			return key
		}
		rel = filepath.ToSlash(r)
	} else {
		// OS keys are absolute (resolveImport makes them so); bring the
		// entry directory to the same form before comparing.
		abs, err := filepath.Abs(entryDir)
		if err != nil {
			return filepath.ToSlash(key)
		}
		r, err := filepath.Rel(abs, key)
		if err != nil {
			return filepath.ToSlash(key)
		}
		rel = filepath.ToSlash(r)
	}
	return path.Join(path.Dir(filepath.ToSlash(entryName)), rel)
}
