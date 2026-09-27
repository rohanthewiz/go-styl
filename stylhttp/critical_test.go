package stylhttp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	styl "github.com/rohanthewiz/go-styl"
	"github.com/rohanthewiz/go-styl/stylcrit"
)

var critFS = fstest.MapFS{
	"app.styl": &fstest.MapFile{Data: []byte(`@import "theme"

.card
  background primary

.unused
  color red

.menu--open
  display block
`)},
	"theme.styl":  &fstest.MapFile{Data: []byte("primary = #0af\n")},
	"broken.styl": &fstest.MapFile{Data: []byte(".x\n  nope()\n")},
}

const critPage = `<html><head><title>x</title></head><body><div class="card">hi</div></body></html>`

// serveCrit runs one request through Critical wrapping h.
func serveCrit(t *testing.T, opts stylcrit.Options, h http.HandlerFunc) *http.Response {
	t.Helper()
	if opts.Path == "" {
		opts.Path = "app.styl"
	}
	if opts.FS == nil {
		opts.FS = critFS
	}
	rec := httptest.NewRecorder()
	Critical(opts)(h).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec.Result()
}

func TestCriticalInlinesIntoHTML(t *testing.T) {
	res := serveCrit(t, stylcrit.Options{}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Length", "999")
		w.Header().Set("ETag", `"v1"`)
		io.WriteString(w, critPage)
	})
	got := body(t, res)
	if !strings.Contains(got, "<style>.card{background:#0af}</style></head>") {
		t.Errorf("critical CSS not inlined:\n%s", got)
	}
	if strings.Contains(got, ".unused") {
		t.Errorf("unused rule leaked:\n%s", got)
	}
	if res.Header.Get("ETag") != "" || res.Header.Get("Content-Length") == "999" {
		t.Errorf("stale ETag/Content-Length kept: %v", res.Header)
	}
}

func TestCriticalSniffsUntypedHTML(t *testing.T) {
	// No Content-Type set: net/http would sniff text/html, and so do we.
	res := serveCrit(t, stylcrit.Options{}, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, critPage)
	})
	if got := body(t, res); !strings.Contains(got, "<style>.card{") {
		t.Errorf("untyped HTML not rewritten:\n%s", got)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestCriticalSafelist(t *testing.T) {
	res := serveCrit(t, stylcrit.Options{Safelist: styl.Used{Classes: []string{"menu--open"}}},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, critPage)
		})
	if got := body(t, res); !strings.Contains(got, ".menu--open") {
		t.Errorf("safelisted rule pruned:\n%s", got)
	}
}

func TestCriticalPassesThrough(t *testing.T) {
	cases := []struct {
		name string
		h    http.HandlerFunc
	}{
		{"json", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"html":"<head></head>"}`)
		}},
		{"error status", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, critPage)
		}},
		{"compressed", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("Content-Encoding", "gzip")
			io.WriteString(w, critPage)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := serveCrit(t, stylcrit.Options{}, c.h)
			if got := body(t, res); strings.Contains(got, "<style>") {
				t.Errorf("response rewritten:\n%s", got)
			}
		})
	}
}

func TestCriticalPassthroughStreams(t *testing.T) {
	// A non-HTML response is written straight through, so Flush reaches the
	// client before the handler returns.
	rec := httptest.NewRecorder()
	flushedEarly := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: 1\n\n")
		w.(http.Flusher).Flush()
		flushedEarly = rec.Flushed && strings.Contains(rec.Body.String(), "data: 1")
	})
	Critical(stylcrit.Options{Path: "app.styl", FS: critFS})(h).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !flushedEarly {
		t.Error("event stream was buffered instead of flushed through")
	}
}

func TestCriticalCompileError(t *testing.T) {
	res := serveCrit(t, stylcrit.Options{Path: "broken.styl"}, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, critPage)
	})
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", res.StatusCode)
	}
	if got := body(t, res); !strings.Contains(got, "broken.styl:2:3") {
		t.Errorf("error not positioned: %q", got)
	}
}

func TestCriticalEmptyResponse(t *testing.T) {
	res := serveCrit(t, stylcrit.Options{}, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if res.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", res.StatusCode)
	}
}
