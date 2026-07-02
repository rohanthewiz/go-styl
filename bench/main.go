// Command bench compares go-styl compile performance against the reference
// Node.js stylus compiler over the shared difftest corpus plus synthetic
// sheets. Run from the repo root:
//
//	go run ./bench
//
// go-styl is timed in-process; reference stylus is timed inside a single
// node process (bench_stylus.mjs), so node startup is excluded from per-file
// numbers and reported separately. Files that fail to compile on either side
// (go-styl extensions, reference-stylus crashes) are skipped. Requires node
// and `npm install --prefix difftest`.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"text/tabwriter"
	"time"

	styl "github.com/rohanthewiz/go-styl"
	"github.com/rohanthewiz/go-styl/internal/benchgen"
)

// corpusGlobs mirrors difftest: top-level corpus files only (imports/ holds
// partials).
var corpusGlobs = []string{
	"examples/*.styl",
	"testdata/*.styl",
	"difftest/corpus/*.styl",
}

type entry struct {
	id    string // display name
	path  string // absolute path
	bytes int
	iters int
	goNS  int64
	refNS int64
	skip  string // non-empty: excluded from the table, with reason
}

func main() {
	target := flag.Duration("time", 250*time.Millisecond, "target measuring time per file per compiler")
	blocks := flag.Int("blocks", 400, "component blocks in the largest synthetic sheet")
	flag.Parse()

	if _, err := os.Stat("go.mod"); err != nil {
		fatal("run from the repository root (go run ./bench)")
	}
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		fatal("node not found in PATH (needed for the reference compiler)")
	}
	if _, err := os.Stat(filepath.Join("difftest", "node_modules", "stylus", "package.json")); err != nil {
		fatal("reference stylus not installed; run: npm install --prefix difftest")
	}

	entries := corpus()
	entries = append(entries, synthetics(*blocks)...)

	// go-styl side: verify each file compiles, calibrate iterations to the
	// target time, then measure. Sources are compiled from memory with the
	// filename set — the same shape the node side gives stylus — so both
	// pay import I/O per compile but neither re-reads the top-level file.
	for i := range entries {
		e := &entries[i]
		src, err := os.ReadFile(e.path)
		if err != nil {
			e.skip = "read: " + err.Error()
			continue
		}
		opts := styl.Options{Filename: e.path}
		if _, err := styl.Compile(string(src), opts); err != nil {
			e.skip = "go-styl: " + err.Error()
			continue
		}
		start := time.Now()
		styl.Compile(string(src), opts)
		est := time.Since(start)
		e.iters = clamp(int(*target/est), 10, 5000)
		start = time.Now()
		for range e.iters {
			styl.Compile(string(src), opts)
		}
		e.goNS = time.Since(start).Nanoseconds() / int64(e.iters)
	}

	measureStylus(nodeBin, entries)

	report(entries, nodeStartup(nodeBin))
}

// corpus returns the difftest corpus as bench entries.
func corpus() []entry {
	var out []entry
	for _, g := range corpusGlobs {
		matches, err := filepath.Glob(g)
		if err != nil || len(matches) == 0 {
			fatal(fmt.Sprintf("bad or empty corpus glob %q: %v", g, err))
		}
		for _, m := range matches {
			abs, err := filepath.Abs(m)
			if err != nil {
				fatal(err.Error())
			}
			data, err := os.ReadFile(m)
			if err != nil {
				fatal(err.Error())
			}
			out = append(out, entry{id: filepath.ToSlash(m), path: abs, bytes: len(data)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out
}

// synthetics writes generated sheets of increasing size to a temp dir (not
// cleaned up until process exit; the OS temp dir is fine for that).
func synthetics(blocks int) []entry {
	dir, err := os.MkdirTemp("", "styl-bench")
	if err != nil {
		fatal(err.Error())
	}
	var out []entry
	for _, s := range []struct {
		name string
		n    int
	}{{"synthetic-20", 20}, {"synthetic-100", 100}, {fmt.Sprintf("synthetic-%d", blocks), blocks}} {
		src := benchgen.Sheet(s.n)
		p := filepath.Join(dir, s.name+".styl")
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			fatal(err.Error())
		}
		out = append(out, entry{id: s.name + ".styl", path: p, bytes: len(src)})
	}
	return out
}

// measureStylus times the reference compiler for every non-skipped entry in
// one node process, using the same per-file iteration counts as the go side.
func measureStylus(nodeBin string, entries []entry) {
	type job struct {
		ID    string `json:"id"`
		Path  string `json:"path"`
		Iters int    `json:"iters"`
	}
	var jobs []job
	for _, e := range entries {
		if e.skip == "" {
			jobs = append(jobs, job{e.id, e.path, e.iters})
		}
	}
	in, err := json.Marshal(jobs)
	if err != nil {
		fatal(err.Error())
	}
	cmd := exec.Command(nodeBin, "bench/bench_stylus.mjs")
	cmd.Stdin = bytes.NewReader(in)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		fatal(fmt.Sprintf("reference stylus bench failed: %v\n%s", err, stderr.String()))
	}
	var res map[string]struct {
		NS    int64  `json:"ns"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		fatal(fmt.Sprintf("bad JSON from bench_stylus.mjs: %v\n%s", err, out))
	}
	for i := range entries {
		e := &entries[i]
		if e.skip != "" {
			continue
		}
		r, ok := res[e.id]
		switch {
		case !ok:
			e.skip = "stylus: no result"
		case r.Error != "":
			e.skip = "stylus: " + r.Error
		default:
			e.refNS = r.NS
		}
	}
}

// nodeStartup times a bare `node -e ""` — the fixed cost every stylus CLI
// invocation pays that the per-file numbers exclude.
func nodeStartup(nodeBin string) time.Duration {
	start := time.Now()
	if err := exec.Command(nodeBin, "-e", "").Run(); err != nil {
		return 0
	}
	return time.Since(start)
}

func report(entries []entry, startup time.Duration) {
	w := tabwriter.NewWriter(os.Stdout, 2, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "file\tbytes\tgo-styl\tstylus\tstylus/go\t")
	logSum, benched := 0.0, 0
	var goTotal, refTotal time.Duration
	for _, e := range entries {
		if e.skip != "" {
			continue
		}
		ratio := float64(e.refNS) / float64(e.goNS)
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%.1f×\t\n", e.id, e.bytes, us(e.goNS), us(e.refNS), ratio)
		logSum += math.Log(ratio)
		benched++
		goTotal += time.Duration(e.goNS)
		refTotal += time.Duration(e.refNS)
	}
	w.Flush()
	if benched == 0 {
		fatal("no file compiled on both sides")
	}
	fmt.Printf("\n%d files benched; one compile of everything: go-styl %s, stylus %s\n",
		benched, goTotal.Round(10*time.Microsecond), refTotal.Round(10*time.Microsecond))
	fmt.Printf("geomean speedup: %.1f× (per-file, excludes node startup: ~%s)\n",
		math.Exp(logSum/float64(benched)), startup.Round(time.Millisecond))
	var skipped []entry
	for _, e := range entries {
		if e.skip != "" {
			skipped = append(skipped, e)
		}
	}
	if len(skipped) > 0 {
		fmt.Printf("\nskipped %d files (compile error on one side):\n", len(skipped))
		for _, e := range skipped {
			fmt.Printf("  %s — %s\n", e.id, firstLine(e.skip))
		}
	}
}

// us renders nanoseconds as microseconds with a thousands-friendly width.
func us(ns int64) string {
	v := float64(ns) / 1e3
	if v >= 1000 {
		return fmt.Sprintf("%.2fms", v/1e3)
	}
	return fmt.Sprintf("%.1fµs", v)
}

func clamp(v, lo, hi int) int {
	return max(lo, min(v, hi))
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "bench:", msg)
	os.Exit(1)
}
