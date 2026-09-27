package stylhttp

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rohanthewiz/go-styl/stylserve"
)

// liveServer serves dir under /css/ with live reload on and a fast poll.
func liveServer(t *testing.T, dir string, live bool) *httptest.Server {
	t.Helper()
	old := livePoll
	livePoll = 20 * time.Millisecond
	t.Cleanup(func() { livePoll = old })
	mux := http.NewServeMux()
	mux.Handle("/css/", http.StripPrefix("/css/", New(stylserve.Options{Dir: dir, LiveReload: live})))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// writeSource writes a stylesheet and bumps its mtime, so a rewrite within
// the filesystem's timestamp granularity still reads as a change.
func writeSource(t *testing.T, p, src string, age time.Duration) {
	t.Helper()
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Add(age)
	if err := os.Chtimes(p, ts, ts); err != nil {
		t.Fatal(err)
	}
}

func TestLiveScriptServed(t *testing.T) {
	srv := liveServer(t, t.TempDir(), true)
	res, err := http.Get(srv.URL + "/css/_live.js")
	if err != nil {
		t.Fatal(err)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("Content-Type = %q", ct)
	}
	if got := body(t, res); !strings.Contains(got, "EventSource") {
		t.Errorf("script = %q", got)
	}
}

func TestLiveOffByDefault(t *testing.T) {
	srv := liveServer(t, t.TempDir(), false)
	res, err := http.Get(srv.URL + "/css/_live.js")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 with LiveReload off", res.StatusCode)
	}
}

func TestLiveStreamReportsChangesAndErrors(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "app.styl")
	writeSource(t, src, ".a\n  color red\n", -time.Hour)
	srv := liveServer(t, dir, true)

	res, err := http.Get(srv.URL + "/css/_live?css=app.css")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}

	// Read "event:" lines in the background.
	events := make(chan string, 8)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			if ev, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
				events <- ev
			}
		}
		close(events)
	}()
	next := func() string {
		select {
		case ev := <-events:
			return ev
		case <-time.After(3 * time.Second):
			t.Fatal("no event within 3s")
			return ""
		}
	}

	time.Sleep(60 * time.Millisecond) // let the stream record the first ETag
	writeSource(t, src, ".a\n  color blue\n", -30*time.Minute)
	if ev := next(); ev != "change" {
		t.Errorf("after an edit got %q, want change", ev)
	}
	writeSource(t, src, ".a\n  nope()\n", -20*time.Minute)
	if ev := next(); ev != "error" {
		t.Errorf("after a broken edit got %q, want error", ev)
	}
	writeSource(t, src, ".a\n  color green\n", -10*time.Minute)
	if ev := next(); ev != "change" {
		t.Errorf("after the fix got %q, want change", ev)
	}
}

func TestLiveStreamNeedsCSS(t *testing.T) {
	srv := liveServer(t, t.TempDir(), true)
	res, err := http.Get(srv.URL + "/css/_live")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", res.StatusCode)
	}
}
