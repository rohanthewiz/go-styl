package stylhttp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rohanthewiz/go-styl/stylserve"
)

func newServer(t *testing.T, opts stylserve.Options) *httptest.Server {
	t.Helper()
	if opts.Dir == "" && opts.FS == nil {
		dir := t.TempDir()
		files := map[string]string{
			"app.styl":    "@import \"theme\"\nbody\n  color fg\n",
			"theme.styl":  "fg = #333\n",
			"broken.styl": ".x\n  nope()\n",
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		opts.Dir = dir
	}
	mux := http.NewServeMux()
	mux.Handle("/css/", http.StripPrefix("/css/", New(opts)))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, url string, hdr map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func body(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestServesCompiledCSS(t *testing.T) {
	srv := newServer(t, stylserve.Options{})
	resp := get(t, srv.URL+"/css/app.css", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/css; charset=utf-8" {
		t.Errorf("content-type = %q", ct)
	}
	if b := body(t, resp); !strings.Contains(b, "color:#333") {
		t.Errorf("body = %q", b)
	}
	if resp.Header.Get("ETag") == "" {
		t.Error("missing ETag")
	}
}

func TestConditionalRequest304(t *testing.T) {
	srv := newServer(t, stylserve.Options{})
	first := get(t, srv.URL+"/css/app.css", nil)
	etag := first.Header.Get("ETag")

	resp := get(t, srv.URL+"/css/app.css", map[string]string{"If-None-Match": etag})
	if resp.StatusCode != http.StatusNotModified {
		t.Errorf("status = %d, want 304", resp.StatusCode)
	}
}

func TestNotFound(t *testing.T) {
	srv := newServer(t, stylserve.Options{})
	for _, p := range []string{"/css/missing.css", "/css/app.styl", "/css/../secret.css"} {
		if resp := get(t, srv.URL+p, nil); resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", p, resp.StatusCode)
		}
	}
}

func TestCompileError500(t *testing.T) {
	srv := newServer(t, stylserve.Options{})
	resp := get(t, srv.URL+"/css/broken.css", nil)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	if b := body(t, resp); !strings.Contains(b, "broken.styl:2:3") {
		t.Errorf("body = %q, want positioned error", b)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	srv := newServer(t, stylserve.Options{})
	resp, err := http.Post(srv.URL+"/css/app.css", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestSourceMapServed(t *testing.T) {
	srv := newServer(t, stylserve.Options{SourceMaps: true})
	resp := get(t, srv.URL+"/css/app.css.map", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if b := body(t, resp); !strings.Contains(b, `"version":3`) {
		t.Errorf("map body = %q", b)
	}
}

func TestNewWithGlobals(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.styl"), []byte("brand ?= #333\n.btn\n  color brand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	brands := map[string]string{"a.example": "#f00", "b.example": "#00f"}
	h := NewWithGlobals(stylserve.Options{Dir: dir}, func(r *http.Request) map[string]any {
		if b, ok := brands[r.Host]; ok {
			return map[string]any{"brand": b}
		}
		return nil
	}, "Host")

	serve := func(host string) *http.Response {
		req := httptest.NewRequest(http.MethodGet, "/app.css", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Result()
	}
	a, b, other := serve("a.example"), serve("b.example"), serve("c.example")
	if got := body(t, a); !strings.Contains(got, "#f00") && !strings.Contains(got, "red") {
		t.Errorf("tenant a = %q", got)
	}
	if got := body(t, b); !strings.Contains(got, "#00f") && !strings.Contains(got, "blue") {
		t.Errorf("tenant b = %q", got)
	}
	if got := body(t, other); !strings.Contains(got, "#333") {
		t.Errorf("default tenant = %q", got)
	}
	if v := a.Header.Get("Vary"); v != "Host" {
		t.Errorf("Vary = %q, want Host", v)
	}

	// Without a vary list, responses are private to the client.
	hp := NewWithGlobals(stylserve.Options{Dir: dir}, func(*http.Request) map[string]any { return nil })
	rec := httptest.NewRecorder()
	hp.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app.css", nil))
	if cc := rec.Result().Header.Get("Cache-Control"); cc != "private" {
		t.Errorf("Cache-Control = %q, want private", cc)
	}
}
