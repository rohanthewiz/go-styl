// Package stylserve is the HTTP-agnostic engine behind the stylhttp and
// stylrweb middleware: it maps request paths like "app.css" (and
// "app.css.map") to .styl sources, compiles them on demand, and caches the
// result, invalidating when the source or any of its @imports change.
package stylserve

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	styl "github.com/rohanthewiz/go-styl"
)

// Options configures an Engine.
type Options struct {
	// Dir is the root directory of .styl sources on the OS filesystem.
	Dir string
	// FS, when set, is the source root instead of Dir (e.g. an embed.FS).
	// Invalidation still works when the FS supports fs.Stat with real
	// modification times (os.DirFS); embed.FS assets are compiled once.
	FS fs.FS
	// IncludePaths lists extra directories searched for @import.
	IncludePaths []string
	// Pretty emits expanded CSS; default is compressed.
	Pretty bool
	// MergeDuplicates folds rules with identical bodies (non-standard).
	MergeDuplicates bool
	// SourceMaps builds a source map per stylesheet, serves it at
	// "<name>.css.map", and appends the sourceMappingURL comment to the CSS.
	SourceMaps bool
}

// Asset is a servable compiled artifact.
type Asset struct {
	Body        []byte
	ContentType string
	ETag        string    // strong ETag, quoted
	ModTime     time.Time // zero when unknown (e.g. embed.FS)
}

// Engine compiles and caches stylesheets. Safe for concurrent use.
type Engine struct {
	opts  Options
	mu    sync.Mutex
	cache map[string]*entry // key: cleaned "<base>.css" request path
}

type entry struct {
	css, srcMap *Asset
	deps        []depStamp
}

type depStamp struct {
	path    string
	modTime time.Time
	size    int64
	ok      bool // stat succeeded when recorded
}

// New creates an Engine over the given source root.
func New(opts Options) *Engine {
	return &Engine{opts: opts, cache: map[string]*entry{}}
}

// Asset resolves a request path ("app.css", "sub/app.css", "app.css.map")
// to a compiled artifact, recompiling if the source or any import changed.
// A path that does not map to an existing .styl source returns an error
// satisfying errors.Is(err, fs.ErrNotExist).
func (e *Engine) Asset(reqPath string) (*Asset, error) {
	reqPath = path.Clean(strings.TrimPrefix(reqPath, "/"))
	if reqPath == "." || strings.HasPrefix(reqPath, "../") || reqPath == ".." {
		return nil, fs.ErrNotExist
	}

	wantMap := false
	cssPath := reqPath
	if strings.HasSuffix(reqPath, ".css.map") {
		wantMap = true
		cssPath = strings.TrimSuffix(reqPath, ".map")
	}
	if !strings.HasSuffix(cssPath, ".css") {
		return nil, fs.ErrNotExist
	}
	if wantMap && !e.opts.SourceMaps {
		return nil, fs.ErrNotExist
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	ent, ok := e.cache[cssPath]
	if !ok || e.stale(ent) {
		var err error
		ent, err = e.build(cssPath)
		if err != nil {
			return nil, err
		}
		e.cache[cssPath] = ent
	}
	if wantMap {
		return ent.srcMap, nil
	}
	return ent.css, nil
}

// build compiles the .styl source behind a "<base>.css" request path.
func (e *Engine) build(cssPath string) (*entry, error) {
	srcRel := strings.TrimSuffix(cssPath, ".css") + ".styl"

	var res styl.Result
	var err error
	if e.opts.FS != nil {
		src := path.Join(rootOr(e.opts.Dir), srcRel)
		res, err = styl.BuildFile(src, styl.Options{
			FS:              e.opts.FS,
			Pretty:          e.opts.Pretty,
			MergeDuplicates: e.opts.MergeDuplicates,
			IncludePaths:    e.opts.IncludePaths,
			SourceMap:       e.opts.SourceMaps,
			OutFile:         path.Base(cssPath),
		})
	} else {
		src := filepath.Join(e.opts.Dir, filepath.FromSlash(srcRel))
		res, err = styl.BuildFile(src, styl.Options{
			Pretty:          e.opts.Pretty,
			MergeDuplicates: e.opts.MergeDuplicates,
			IncludePaths:    e.opts.IncludePaths,
			SourceMap:       e.opts.SourceMaps,
			OutFile:         path.Base(cssPath),
		})
	}
	if err != nil {
		return nil, err
	}

	body := []byte(res.CSS)
	var srcMap *Asset
	if e.opts.SourceMaps {
		body = append(body, []byte("\n/*# sourceMappingURL="+path.Base(cssPath)+".map */\n")...)
		srcMap = newAsset([]byte(res.Map), "application/json; charset=utf-8")
	}

	ent := &entry{
		css:    newAsset(body, "text/css; charset=utf-8"),
		srcMap: srcMap,
	}
	for _, d := range res.Deps {
		ent.deps = append(ent.deps, e.stamp(d))
	}
	// Freshest dep mtime becomes the asset ModTime (Last-Modified).
	for _, d := range ent.deps {
		if d.ok && d.modTime.After(ent.css.ModTime) {
			ent.css.ModTime = d.modTime
			if srcMap != nil {
				srcMap.ModTime = d.modTime
			}
		}
	}
	return ent, nil
}

// stale reports whether any dependency changed since the entry was built.
func (e *Engine) stale(ent *entry) bool {
	for _, d := range ent.deps {
		now := e.stamp(d.path)
		if now.ok != d.ok || now.modTime != d.modTime || now.size != d.size {
			return true
		}
	}
	return false
}

// stamp records a dependency's current stat fingerprint.
func (e *Engine) stamp(p string) depStamp {
	var info fs.FileInfo
	var err error
	if e.opts.FS != nil {
		info, err = fs.Stat(e.opts.FS, p)
	} else {
		info, err = os.Stat(p)
	}
	if err != nil {
		return depStamp{path: p}
	}
	return depStamp{path: p, modTime: info.ModTime(), size: info.Size(), ok: true}
}

func newAsset(body []byte, ctype string) *Asset {
	sum := sha256.Sum256(body)
	return &Asset{
		Body:        body,
		ContentType: ctype,
		ETag:        fmt.Sprintf("%q", hex.EncodeToString(sum[:8])),
	}
}

// rootOr returns dir or "." for fs.FS path joining.
func rootOr(dir string) string {
	if dir == "" {
		return "."
	}
	return dir
}
