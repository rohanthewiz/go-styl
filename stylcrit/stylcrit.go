// Package stylcrit is the HTTP-agnostic engine behind per-response critical
// CSS middleware: given a rendered HTML page, it prunes a .styl stylesheet
// down to the rules the page actually uses (styl.Prune) and injects the
// result as an inline <style> block in <head>.
//
// Pruned output is cached by the page's used-name set, so pages sharing a
// layout compile once; the cache is invalidated when the stylesheet or any
// of its @imports change.
package stylcrit

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	styl "github.com/rohanthewiz/go-styl"
)

// Options configures an Engine.
type Options struct {
	// Path is the .styl stylesheet to prune, on the OS filesystem, or in FS
	// when set.
	Path string
	// FS, when set, is the filesystem Path and its @imports resolve in
	// (e.g. an embed.FS).
	FS fs.FS
	// IncludePaths lists extra directories searched for @import.
	IncludePaths []string
	// Pretty emits expanded CSS; default is compressed (the usual choice for
	// inlined critical CSS).
	Pretty bool
	// MergeDuplicates folds rules with identical bodies (non-standard).
	MergeDuplicates bool
	// Globals defines variables in the stylesheet's root scope (see
	// styl.Options.Globals).
	Globals map[string]any
	// CustomProperties lists root-level variables to expose as CSS custom
	// properties (see styl.Options.CustomProperties).
	CustomProperties []string
	// Safelist names additional to what the HTML uses — classes, IDs, or
	// tags that client-side script toggles after load (styl.Prune would
	// otherwise drop their rules). Nil axes add nothing.
	Safelist styl.Used
	// MaxCached caps the number of cached used-set variants (default 256).
	// When full the cache resets — recompiles cost microseconds.
	MaxCached int
}

// Engine prunes and caches critical CSS per used-name set. Safe for
// concurrent use.
type Engine struct {
	opts Options
	mu   sync.Mutex
	css  map[[sha256.Size]byte]string
	deps []depStamp
}

type depStamp struct {
	path    string
	modTime time.Time
	size    int64
	ok      bool
}

// New creates an Engine for one stylesheet.
func New(opts Options) *Engine {
	if opts.MaxCached <= 0 {
		opts.MaxCached = 256
	}
	return &Engine{opts: opts, css: map[[sha256.Size]byte]string{}}
}

// CSS returns the critical CSS for a rendered HTML page: the stylesheet
// pruned to the names the page (plus the Safelist) uses.
func (e *Engine) CSS(html string) (string, error) {
	used := e.used(html)
	key := usedKey(used)

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stale() {
		if err := e.restamp(); err != nil {
			return "", err
		}
	}
	if css, ok := e.css[key]; ok {
		return css, nil
	}
	css, err := styl.PruneFile(e.opts.Path, used, e.stylOptions())
	if err != nil {
		return "", err
	}
	if len(e.css) >= e.opts.MaxCached {
		e.css = map[[sha256.Size]byte]string{}
	}
	e.css[key] = css
	return css, nil
}

// Inline returns the page with its critical CSS injected as a <style> block:
// before </head> when present, else before </body>, else prepended. A page
// that needs no CSS at all comes back unchanged.
func (e *Engine) Inline(html string) (string, error) {
	css, err := e.CSS(html)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(css) == "" {
		return html, nil
	}
	block := "<style>" + css + "</style>"
	lower := strings.ToLower(html)
	if i := strings.Index(lower, "</head>"); i >= 0 {
		return html[:i] + block + html[i:], nil
	}
	if i := strings.Index(lower, "</body>"); i >= 0 {
		return html[:i] + block + html[i:], nil
	}
	return block + html, nil
}

// used collects the page's names and merges in the Safelist.
func (e *Engine) used(html string) styl.Used {
	u := styl.UsedFromHTML(html)
	u.Classes = mergeNames(u.Classes, e.opts.Safelist.Classes)
	u.IDs = mergeNames(u.IDs, e.opts.Safelist.IDs)
	u.Tags = mergeNames(u.Tags, lowerNames(e.opts.Safelist.Tags))
	return u
}

func (e *Engine) stylOptions() styl.Options {
	return styl.Options{
		FS:               e.opts.FS,
		IncludePaths:     e.opts.IncludePaths,
		Pretty:           e.opts.Pretty,
		MergeDuplicates:  e.opts.MergeDuplicates,
		Globals:          e.opts.Globals,
		CustomProperties: e.opts.CustomProperties,
	}
}

// stale reports whether the stylesheet or any @import changed since the
// dependency list was recorded (an empty list means "not recorded yet").
func (e *Engine) stale() bool {
	if len(e.deps) == 0 {
		return true
	}
	for _, d := range e.deps {
		now := e.stamp(d.path)
		if now.ok != d.ok || now.modTime != d.modTime || now.size != d.size {
			return true
		}
	}
	return false
}

// restamp rebuilds the dependency list via a full compile and resets the
// cache. Called with e.mu held.
func (e *Engine) restamp() error {
	res, err := styl.BuildFile(e.opts.Path, e.stylOptions())
	if err != nil {
		return err
	}
	e.deps = e.deps[:0]
	for _, d := range res.Deps {
		e.deps = append(e.deps, e.stamp(d))
	}
	e.css = map[[sha256.Size]byte]string{}
	return nil
}

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

// usedKey fingerprints a used-name set. Axis boundaries are marked so
// {Classes: [a]} and {IDs: [a]} key differently.
func usedKey(u styl.Used) [sha256.Size]byte {
	h := sha256.New()
	for _, axis := range [][]string{u.Classes, u.IDs, u.Tags} {
		for _, name := range axis {
			h.Write([]byte(name))
			h.Write([]byte{0})
		}
		h.Write([]byte{1})
	}
	var key [sha256.Size]byte
	h.Sum(key[:0])
	return key
}

// mergeNames unions two sorted name lists, keeping the base's nil-ness when
// there is nothing to add.
func mergeNames(base, extra []string) []string {
	if len(extra) == 0 {
		return base
	}
	set := make(map[string]bool, len(base)+len(extra))
	for _, n := range base {
		set[n] = true
	}
	for _, n := range extra {
		set[n] = true
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func lowerNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = strings.ToLower(n)
	}
	return out
}
