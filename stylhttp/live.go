package stylhttp

import (
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"
)

// Live reload (stylserve.Options.LiveReload, development only).
//
// With it on, the handler serves two extra paths next to the stylesheets:
//
//	_live.js              a script the page includes once:
//	                      <script src="/css/_live.js"></script>
//	_live?css=<name>.css  a Server-Sent Events stream for one stylesheet
//
// The flow:
//
//	page ──<script _live.js>──▶ finds each <link rel=stylesheet> under the
//	                            script's own prefix (/css/…) and opens
//	                            EventSource(/css/_live?css=app.css)
//	server ── every 500ms ────▶ AssetWith(app.css): rebuilds if a source or
//	                            @import changed; ETag moved → "change",
//	                            compile error → "error" (the positioned message)
//	page ◀── change ──────────  swaps the link's href (?v=<etag>): the new CSS
//	                            applies without a reload, keeping page state
//	page ◀── error ───────────  console.error, keeping the last good CSS
//
// Polling the engine keeps this dependency-free (no fs watcher) and reuses
// the exact invalidation the handler already does. Keep LiveReload off in
// production: each open page holds a connection and polls.

// livePoll is how often a live stream re-checks its stylesheet.
var livePoll = 500 * time.Millisecond

// liveJS is the client half. It locates stylesheets by the prefix its own
// src was served under, so it works behind any mux prefix.
const liveJS = `(function () {
  var me = document.currentScript;
  if (!me || !window.EventSource) return;
  var base = me.src.replace(/_live\.js(\?.*)?$/, '');
  var links = document.querySelectorAll('link[rel~="stylesheet"]');
  Array.prototype.forEach.call(links, function (link) {
    if (link.href.indexOf(base) !== 0) return;
    var name = link.href.slice(base.length).split(/[?#]/)[0];
    var es = new EventSource(base + '_live?css=' + encodeURIComponent(name));
    es.addEventListener('change', function (e) {
      // Swap in the new stylesheet before removing the old one, so the page
      // never renders unstyled in between.
      var next = link.cloneNode();
      next.href = base + name + '?v=' + encodeURIComponent(e.data);
      next.onload = function () { link.remove(); link = next; };
      link.after(next);
      console.info('[styl] reloaded ' + name);
    });
    es.addEventListener('error', function (e) {
      if (e.data) console.error('[styl] ' + name + ': ' + e.data);
    });
  });
})();
`

// serveLive handles the live-reload paths; it reports whether it did.
func (h *handler) serveLive(w http.ResponseWriter, r *http.Request) bool {
	switch strings.TrimPrefix(r.URL.Path, "/") {
	case "_live.js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(liveJS))
		return true
	case "_live":
		h.liveStream(w, r)
		return true
	}
	return false
}

// liveStream sends a "change" event whenever the stylesheet's ETag moves and
// an "error" event when it stops compiling, until the client goes away.
func (h *handler) liveStream(w http.ResponseWriter, r *http.Request) {
	name := path.Clean("/" + r.URL.Query().Get("css"))[1:]
	flusher, ok := w.(http.Flusher)
	if name == "" || !ok {
		http.Error(w, "live reload needs ?css=<name>.css and a flushing writer", http.StatusBadRequest)
		return
	}
	vars := h.requestGlobals(r)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	// A comment line opens the stream at once, so the browser's EventSource
	// reports it connected before the first change.
	fmt.Fprint(w, ": styl live reload\n\n")
	flusher.Flush()

	last, lastErr := "", ""
	if a, err := h.eng.AssetWith(name, vars); err == nil {
		last = a.ETag
	} else {
		lastErr = err.Error()
	}

	tick := time.NewTicker(livePoll)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
		}
		a, err := h.eng.AssetWith(name, vars)
		switch {
		case err != nil:
			if msg := err.Error(); msg != lastErr {
				lastErr = msg
				writeEvent(w, "error", msg)
			}
		case a.ETag != last || lastErr != "":
			last, lastErr = a.ETag, ""
			writeEvent(w, "change", strings.Trim(a.ETag, `"`))
		default:
			continue
		}
		flusher.Flush()
	}
}

// writeEvent writes one SSE event; a multi-line payload becomes several
// data lines, which the browser joins back with newlines.
func writeEvent(w http.ResponseWriter, event, data string) {
	fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range strings.Split(data, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
}
