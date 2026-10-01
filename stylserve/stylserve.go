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
	"sort"
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
	// Globals defines variables in every stylesheet's root scope (see
	// styl.Options.Globals for value conversion). For per-request or
	// per-tenant values, pass them to AssetWith, which layers them over these.
	Globals map[string]any
	// CustomProperties lists root-level variables to expose as CSS custom
	// properties (see styl.Options.CustomProperties).
	CustomProperties []string
	// LiveReload (development only) asks HTTP adapters to serve a small
	// live-reload script and event stream next to the stylesheets, so a page
	// swaps in fresh CSS when a source changes, without a reload. See
	// stylhttp for the net/http wiring. The engine itself doesn't use it.
	LiveReload bool
	// MaxVariants caps how many per-globals variants (AssetWith with a
	// non-empty set) are cached across all stylesheets (default 256). When
	// full, the variants are dropped and rebuilt on demand; builds with no
	// extra globals are never evicted. The cap matters when the variable
	// set comes from request data: without it, the cache could grow
	// without bound.
	MaxVariants int
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
	// variants caches AssetWith builds with extra globals. The key is the
	// request path plus a fingerprint of the merged globals
	// ("<base>.css\x00<sha256>"), so each variable set compiles once.
	variants map[string]*entry
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
	if opts.MaxVariants <= 0 {
		opts.MaxVariants = 256
	}
	return &Engine{opts: opts, cache: map[string]*entry{}, variants: map[string]*entry{}}
}

// Asset resolves a request path ("app.css", "sub/app.css", "app.css.map")
// to a compiled artifact, recompiling if the source or any import changed.
// A path that does not map to an existing .styl source returns an error
// satisfying errors.Is(err, fs.ErrNotExist).
func (e *Engine) Asset(reqPath string) (*Asset, error) {
	return e.AssetWith(reqPath, nil)
}

// AssetWith is Asset with extra globals layered over Options.Globals (a
// key in globals wins), for CSS that varies per request or tenant:
//
//	asset, err := eng.AssetWith("app.css", map[string]any{"brand": tenant.Color})
//
// Each distinct merged variable set is compiled once and cached, and
// invalidated like any other build when a source changes. An empty globals
// map is the same as Asset. Responses served this way differ by whatever
// chose the globals, so an HTTP layer should send a matching Vary header
// (or Cache-Control: private).
func (e *Engine) AssetWith(reqPath string, globals map[string]any) (*Asset, error) {
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

	// Pick the cache for this build: the base cache, or the variant cache
	// keyed by the merged globals' fingerprint.
	cache, key, merged := e.cache, cssPath, e.opts.Globals
	if len(globals) > 0 {
		merged = mergeGlobals(e.opts.Globals, globals)
		cache, key = e.variants, cssPath+"\x00"+globalsKey(merged)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	ent, ok := cache[key]
	if !ok || e.stale(ent) {
		var err error
		ent, err = e.build(cssPath, merged)
		if err != nil {
			return nil, err
		}
		if len(globals) > 0 && !ok && len(e.variants) >= e.opts.MaxVariants {
			e.variants = map[string]*entry{}
			cache = e.variants
		}
		cache[key] = ent
	}
	if wantMap {
		return ent.srcMap, nil
	}
	return ent.css, nil
}

// build compiles the .styl source behind a "<base>.css" request path with
// the given root-scope globals.
func (e *Engine) build(cssPath string, globals map[string]any) (*entry, error) {
	srcRel := strings.TrimSuffix(cssPath, ".css") + ".styl"
	// The map is served at "<cssPath>.map", and the URL tree mirrors the
	// source tree under Dir, so in source terms it sits beside the .styl
	// file. Naming sources from there gives "app.styl" and
	// "partials/_btn.styl" instead of Dir-prefixed (or, on the OS
	// filesystem, absolute server) paths.
	mapRel := cssPath + ".map"

	var res styl.Result
	var err error
	if e.opts.FS != nil {
		src := path.Join(rootOr(e.opts.Dir), srcRel)
		res, err = styl.BuildFile(src, styl.Options{
			FS:               e.opts.FS,
			Pretty:           e.opts.Pretty,
			MergeDuplicates:  e.opts.MergeDuplicates,
			IncludePaths:     e.opts.IncludePaths,
			Globals:          globals,
			CustomProperties: e.opts.CustomProperties,
			SourceMap:        e.opts.SourceMaps,
			OutFile:          path.Base(cssPath),
			MapFile:          path.Join(rootOr(e.opts.Dir), mapRel),
		})
	} else {
		src := filepath.Join(e.opts.Dir, filepath.FromSlash(srcRel))
		res, err = styl.BuildFile(src, styl.Options{
			Pretty:           e.opts.Pretty,
			MergeDuplicates:  e.opts.MergeDuplicates,
			IncludePaths:     e.opts.IncludePaths,
			Globals:          globals,
			CustomProperties: e.opts.CustomProperties,
			SourceMap:        e.opts.SourceMaps,
			OutFile:          path.Base(cssPath),
			MapFile:          filepath.Join(e.opts.Dir, filepath.FromSlash(mapRel)),
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

// mergeGlobals returns base overlaid with extra (extra wins), as a new map.
func mergeGlobals(base, extra map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// globalsKey fingerprints a globals map independent of map order: each
// name and its value's type and Go-syntax form, sorted by name. The type
// is part of it, so "10" (a Stylus expression string) and 10 (a number)
// key differently even though they print alike.
func globalsKey(g map[string]any) string {
	names := make([]string, 0, len(g))
	for k := range g {
		names = append(names, k)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, k := range names {
		fmt.Fprintf(h, "%s\x00%T\x00%#v\x00", k, g[k], g[k])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// rootOr returns dir or "." for fs.FS path joining.
func rootOr(dir string) string {
	if dir == "" {
		return "."
	}
	return dir
}
