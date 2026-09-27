package stylhttp

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/rohanthewiz/go-styl/stylcrit"
)

// Critical returns net/http middleware that inlines per-response critical
// CSS into HTML responses, the net/http twin of rweb's middleware/critical:
//
//	mux := http.NewServeMux()
//	mux.HandleFunc("/", renderPage)
//	http.ListenAndServe(":8080", stylhttp.Critical(stylcrit.Options{
//		Path:     "styles/app.styl", // or FS: an embed.FS
//		Safelist: styl.Used{Classes: []string{"menu--open"}},
//	})(mux))
//
// After the handler renders a page, the stylesheet is pruned to the tags,
// classes and IDs that page uses (plus Safelist) and injected as a <style>
// block before </head>. Results are cached per used-name set and invalidated
// when the stylesheet or its @imports change (see stylcrit).
//
// Only successful (2xx), uncompressed text/html responses are rewritten.
// Everything else passes through untouched and unbuffered, so streamed
// responses (SSE, large downloads) keep streaming. A stylesheet compile error
// fails the response with a 500 carrying the positioned (file:line:col)
// message, matching New.
//
// A rewritten response loses its Content-Length (the body grew) and its ETag
// (it no longer describes the body, and would survive a stylesheet change).
func Critical(opts stylcrit.Options) func(http.Handler) http.Handler {
	eng := stylcrit.New(opts)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cw := &critWriter{w: w, status: http.StatusOK}
			next.ServeHTTP(cw, r)
			cw.finish(eng)
		})
	}
}

// critWriter buffers a response until it can tell whether the response is an
// HTML page to rewrite. The decision is made once, when the status is
// written (explicitly, or implicitly by the first Write):
//
//	WriteHeader ─┬─ non-2xx, non-HTML type, or Content-Encoding ──▶ passthrough
//	             │  (headers sent now; later writes go straight through)
//	             └─ otherwise ──▶ buffer (sniffed in finish if no type was set)
type critWriter struct {
	w           http.ResponseWriter
	status      int
	wroteHeader bool
	passthrough bool
	buf         bytes.Buffer
}

func (c *critWriter) Header() http.Header { return c.w.Header() }

func (c *critWriter) WriteHeader(status int) {
	if c.wroteHeader {
		return
	}
	c.wroteHeader = true
	c.status = status
	h := c.w.Header()
	ct := h.Get("Content-Type")
	if status < 200 || status > 299 || h.Get("Content-Encoding") != "" ||
		(ct != "" && !strings.Contains(ct, "text/html")) {
		c.passthrough = true
		c.w.WriteHeader(status)
	}
}

func (c *critWriter) Write(p []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	if c.passthrough {
		return c.w.Write(p)
	}
	return c.buf.Write(p)
}

// Flush forwards to the underlying writer in passthrough mode. A buffered
// page can't be flushed early: the whole body is needed to prune its CSS.
func (c *critWriter) Flush() {
	if c.passthrough {
		if f, ok := c.w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (c *critWriter) Unwrap() http.ResponseWriter { return c.w }

// finish sends a buffered response, rewriting it when it is an HTML page.
func (c *critWriter) finish(eng *stylcrit.Engine) {
	if c.passthrough {
		return
	}
	h := c.w.Header()
	if !c.wroteHeader {
		// The handler wrote nothing at all: send its (empty) response.
		c.w.WriteHeader(c.status)
		return
	}
	body := c.buf.Bytes()
	ct := h.Get("Content-Type")
	if ct == "" && len(body) > 0 {
		// net/http would sniff the type on the first write; do the same so
		// a handler that never sets Content-Type still gets its CSS.
		ct = http.DetectContentType(body)
		h.Set("Content-Type", ct)
	}
	if len(body) > 0 && strings.Contains(ct, "text/html") {
		out, err := eng.Inline(string(body))
		if err != nil {
			h.Del("Content-Length")
			h.Del("ETag")
			http.Error(c.w, "critical CSS: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if len(out) != len(body) {
			body = []byte(out)
			h.Del("Content-Length")
			h.Del("ETag")
		}
	}
	c.w.WriteHeader(c.status)
	_, _ = c.w.Write(body)
}
