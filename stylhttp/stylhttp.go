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
	return &handler{eng: stylserve.New(opts)}
}

type handler struct {
	eng *stylserve.Engine
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	asset, err := h.eng.Asset(r.URL.Path)
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
