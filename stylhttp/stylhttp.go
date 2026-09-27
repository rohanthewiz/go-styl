// Package stylhttp serves compiled Stylus stylesheets over net/http.
//
// Requests for "<name>.css" compile "<name>.styl" from the configured source
// root on demand, with caching invalidated by source (and @import) changes:
//
//	mux.Handle("/css/", http.StripPrefix("/css/",
//		stylhttp.New(stylserve.Options{Dir: "./styles"})))
//
// With go:embed the stylesheets ship inside the binary:
//
//	//go:embed styles/*.styl
//	var styles embed.FS
//	sub, _ := fs.Sub(styles, "styles")
//	mux.Handle("/css/", http.StripPrefix("/css/",
//		stylhttp.New(stylserve.Options{FS: sub})))
//
// With Options.SourceMaps set, "<name>.css.map" is served alongside and the
// CSS gains a sourceMappingURL comment.
//
// NewWithGlobals varies the CSS per request (a tenant's brand color, a
// user's theme), compiling and caching each distinct variable set once.
//
// In development, Options.LiveReload adds a script that swaps in fresh CSS
// when a source changes: include <script src="/css/_live.js"></script>.
package stylhttp

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"

	"github.com/rohanthewiz/go-styl/stylserve"
)

// New returns an http.Handler serving compiled CSS from the source root
// described by opts. The request path (after any mux prefix stripping) is
// mapped to a .styl source: "sub/app.css" -> "sub/app.styl".
func New(opts stylserve.Options) http.Handler {
	return &handler{eng: stylserve.New(opts), live: opts.LiveReload}
}

// NewWithGlobals is New with per-request globals: globals(r) returns
// variables layered over opts.Globals for that request (nil or empty for
// none). Each distinct set is compiled once and cached, up to
// opts.MaxVariants sets:
//
//	h := stylhttp.NewWithGlobals(stylserve.Options{Dir: "./styles"},
//		func(r *http.Request) map[string]any {
//			t := tenantFor(r) // e.g. from the Host header
//			return map[string]any{"brand": t.Brand, "radius": t.Radius}
//		})
//
// The response depends on whatever globals reads, so vary is sent as the
// Vary header when non-empty (e.g. "Host", "Cookie"); with no vary list the
// response is marked Cache-Control: private, so shared caches don't serve
// one tenant's CSS to another.
func NewWithGlobals(opts stylserve.Options, globals func(r *http.Request) map[string]any, vary ...string) http.Handler {
	return &handler{eng: stylserve.New(opts), globals: globals, vary: vary, live: opts.LiveReload}
}

type handler struct {
	eng     *stylserve.Engine
	globals func(r *http.Request) map[string]any // nil for New
	vary    []string
	live    bool // serve _live.js and _live (see live.go)
}

// requestGlobals returns the per-request globals (nil without a callback).
func (h *handler) requestGlobals(r *http.Request) map[string]any {
	if h.globals == nil {
		return nil
	}
	return h.globals(r)
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.live && h.serveLive(w, r) {
		return
	}

	vars := h.requestGlobals(r)
	if h.globals != nil {
		if len(h.vary) > 0 {
			for _, v := range h.vary {
				w.Header().Add("Vary", v)
			}
		} else {
			w.Header().Set("Cache-Control", "private")
		}
	}
	asset, err := h.eng.AssetWith(r.URL.Path, vars)
	if errors.Is(err, fs.ErrNotExist) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		// Compile errors are positioned (file:line:col) — surface them.
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", asset.ContentType)
	w.Header().Set("ETag", asset.ETag)
	// ServeContent handles If-None-Match/If-Modified-Since (304), HEAD, and
	// range requests. The empty name is fine: Content-Type is already set.
	http.ServeContent(w, r, "", asset.ModTime, bytes.NewReader(asset.Body))
}
