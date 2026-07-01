// Package stylrweb serves compiled Stylus stylesheets from an rweb server
// (github.com/rohanthewiz/rweb).
//
// Register the handler on a wildcard route; the wildcard segment selects the
// stylesheet ("/css/app.css" -> "<root>/app.styl"):
//
//	s := rweb.NewServer(rweb.ServerOptions{Address: ":8080"})
//	s.Get("/css/*path", stylrweb.Handler(stylserve.Options{Dir: "./styles"}))
//
// With go:embed the stylesheets ship inside the binary:
//
//	//go:embed styles/*.styl
//	var styles embed.FS
//	sub, _ := fs.Sub(styles, "styles")
//	s.Get("/css/*path", stylrweb.Handler(stylserve.Options{FS: sub}))
//
// With Options.SourceMaps set, "<name>.css.map" is served alongside and the
// CSS gains a sourceMappingURL comment.
package stylrweb

import (
	"errors"
	"io/fs"
	"net/http"

	"github.com/rohanthewiz/go-styl/stylserve"
	"github.com/rohanthewiz/rweb"
)

// Handler returns an rweb handler serving compiled CSS from the source root
// described by opts. Mount it on a route whose final segment is the "*path"
// wildcard.
func Handler(opts stylserve.Options) func(rweb.Context) error {
	eng := stylserve.New(opts)

	return func(ctx rweb.Context) error {
		asset, err := eng.Asset(ctx.Request().PathParam("path"))
		if errors.Is(err, fs.ErrNotExist) {
			return ctx.SetStatus(http.StatusNotFound).WriteString("not found")
		}
		if err != nil {
			// Compile errors are positioned (file:line:col) — surface them.
			return ctx.SetStatus(http.StatusInternalServerError).WriteString(err.Error())
		}

		res := ctx.Response()
		res.SetHeader("Content-Type", asset.ContentType)
		res.SetHeader("ETag", asset.ETag)
		if !asset.ModTime.IsZero() {
			res.SetHeader("Last-Modified", asset.ModTime.UTC().Format(http.TimeFormat))
		}
		if ctx.Request().Header("If-None-Match") == asset.ETag {
			ctx.SetStatus(http.StatusNotModified)
			return nil
		}
		return ctx.Bytes(asset.Body)
	}
}
