// Package difftest differentially tests go-styl against the reference
// Node.js Stylus compiler over a shared corpus, tracking a compatibility
// score. Known divergences live in known_diffs.txt; the test fails when a
// matching file regresses to a diff AND when a known diff starts matching
// (a ratchet — remove the entry to lock in the improvement).
//
// Requirements: node on PATH and `npm install` run in this directory
// (the test skips otherwise). Refresh known_diffs.txt with:
//
//	UPDATE_KNOWN_DIFFS=1 go test ./difftest
package difftest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	styl "github.com/rohanthewiz/go-styl"
)

// repoRoot is the repository root relative to this package directory.
const repoRoot = ".."

// corpusGlobs are the repo-root-relative patterns gathered into the corpus.
// Files under examples/imports/ and testdata/imports/ are partials pulled in
// by @import, so only top-level files are compiled directly.
var corpusGlobs = []string{
	"examples/*.styl",
	"testdata/*.styl",
	"difftest/corpus/*.styl",
}

type stylusResult struct {
	CSS   string `json:"css"`
	Error string `json:"error"`
}

type outcome struct {
	file   string
	status string // "match", "both-error", "diff", "go-error", "stylus-error"
	detail string
}

// agrees reports whether the two compilers agree on this file: identical
// normalized CSS, or both rejecting the input.
func (o outcome) agrees() bool { return o.status == "match" || o.status == "both-error" }

func TestDifferential(t *testing.T) {
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found in PATH; skipping differential test")
	}
	if _, err := os.Stat(filepath.Join("node_modules", "stylus", "package.json")); err != nil {
		t.Skip("stylus npm package not installed; run: npm install --prefix difftest")
	}

	files := corpusFiles(t)
	if len(files) == 0 {
		t.Fatal("empty corpus")
	}
	ref := compileStylus(t, nodeBin, files)

	var outcomes []outcome
	for _, f := range files {
		outcomes = append(outcomes, compare(f, ref[f]))
	}

	known := loadKnownDiffs(t)
	if os.Getenv("UPDATE_KNOWN_DIFFS") != "" {
		writeKnownDiffs(t, outcomes, known)
	} else {
		checkAgainstBaseline(t, outcomes, known)
	}

	agree := 0
	for _, o := range outcomes {
		if o.agrees() {
			agree++
		}
	}
	t.Logf("compatibility score: %d/%d (%.0f%%) vs reference stylus", agree, len(outcomes), 100*float64(agree)/float64(len(outcomes)))
}

// compare compiles one corpus file with go-styl and classifies it against the
// reference compiler's result.
func compare(file string, ref stylusResult) outcome {
	got, goErr := styl.CompileFile(filepath.Join(repoRoot, filepath.FromSlash(file)), styl.Options{})
	switch {
	case goErr != nil && ref.Error != "":
		return outcome{file, "both-error", ""}
	case goErr != nil:
		return outcome{file, "go-error", "go-styl: " + goErr.Error()}
	case ref.Error != "":
		return outcome{file, "stylus-error", "stylus: " + ref.Error}
	}
	g, r := normalize(got), normalize(ref.CSS)
	if g == r {
		return outcome{file, "match", ""}
	}
	return outcome{file, "diff", diffExcerpt(g, r)}
}

// corpusFiles returns the corpus as sorted repo-root-relative slash paths.
func corpusFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, g := range corpusGlobs {
		matches, err := filepath.Glob(filepath.Join(repoRoot, filepath.FromSlash(g)))
		if err != nil {
			t.Fatalf("bad corpus glob %q: %v", g, err)
		}
		for _, m := range matches {
			rel, err := filepath.Rel(repoRoot, m)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, filepath.ToSlash(rel))
		}
	}
	sort.Strings(out)
	return out
}

// compileStylus runs the whole corpus through the reference compiler in one
// node process and returns results keyed by repo-relative path.
func compileStylus(t *testing.T, nodeBin string, files []string) map[string]stylusResult {
	t.Helper()
	type job struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	}
	jobs := make([]job, 0, len(files))
	for _, f := range files {
		abs, err := filepath.Abs(filepath.Join(repoRoot, filepath.FromSlash(f)))
		if err != nil {
			t.Fatal(err)
		}
		jobs = append(jobs, job{ID: f, Path: abs})
	}
	in, err := json.Marshal(jobs)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(nodeBin, "compile_stylus.mjs")
	cmd.Stdin = bytes.NewReader(in)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("reference stylus run failed: %v\n%s", err, stderr.String())
	}
	ref := make(map[string]stylusResult)
	if err := json.Unmarshal(out, &ref); err != nil {
		t.Fatalf("bad JSON from compile_stylus.mjs: %v\n%s", err, out)
	}
	for _, f := range files {
		if _, ok := ref[f]; !ok {
			t.Fatalf("compile_stylus.mjs returned no result for %s", f)
		}
	}
	return ref
}

// checkAgainstBaseline enforces the ratchet: unexpected divergences fail, and
// so do known_diffs entries that now agree (remove them to lock the gain in).
func checkAgainstBaseline(t *testing.T, outcomes []outcome, known map[string]string) {
	t.Helper()
	for _, o := range outcomes {
		note, isKnown := known[o.file]
		switch {
		case !o.agrees() && !isKnown:
			t.Errorf("%s: output diverges from reference stylus (%s)\n%s\nIf this divergence is intended, add the file to difftest/known_diffs.txt.",
				o.file, o.status, o.detail)
		case o.agrees() && isKnown:
			t.Errorf("%s: now agrees with reference stylus — remove it from difftest/known_diffs.txt (was: %s)", o.file, note)
		}
	}
	for f := range known {
		found := false
		for _, o := range outcomes {
			if o.file == f {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("known_diffs.txt lists %s, which is not in the corpus", f)
		}
	}
}

// loadKnownDiffs parses known_diffs.txt: '#' comments and blank lines are
// skipped; otherwise the first field is the file path and the rest is a note.
func loadKnownDiffs(t *testing.T) map[string]string {
	t.Helper()
	known := make(map[string]string)
	data, err := os.ReadFile("known_diffs.txt")
	if os.IsNotExist(err) {
		return known
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		known[fields[0]] = strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
	}
	return known
}

// writeKnownDiffs rewrites known_diffs.txt from the current outcomes,
// preserving notes for entries that still diverge and the file's existing
// leading comment block.
func writeKnownDiffs(t *testing.T, outcomes []outcome, old map[string]string) {
	t.Helper()
	var b strings.Builder
	b.WriteString(knownDiffsHeader(t))
	for _, o := range outcomes {
		if o.agrees() {
			continue
		}
		note := old[o.file]
		if note == "" {
			note = o.status + ": " + firstLine(o.detail)
		}
		fmt.Fprintf(&b, "%s %s\n", o.file, note)
	}
	if err := os.WriteFile("known_diffs.txt", []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Log("known_diffs.txt updated")
}

// knownDiffsHeader returns the leading comment block of the existing
// known_diffs.txt, or a default header when the file doesn't exist yet.
func knownDiffsHeader(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("known_diffs.txt")
	if os.IsNotExist(err) {
		return "# Corpus files where go-styl currently diverges from reference stylus (npm).\n" +
			"# Format: <path> <note>. Remove a line once the file matches; regenerate with\n" +
			"#   UPDATE_KNOWN_DIFFS=1 go test ./difftest\n"
	}
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "#") {
			break
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// diffExcerpt pinpoints the first divergence between the two outputs with a
// window of context on each side.
func diffExcerpt(got, want string) string {
	i := 0
	for i < len(got) && i < len(want) && got[i] == want[i] {
		i++
	}
	start := max(i-40, 0)
	window := func(s string) string {
		return s[start:min(i+80, len(s))]
	}
	return fmt.Sprintf("first divergence at byte %d:\n  go-styl: %q\n  stylus:  %q", i, window(got), window(want))
}
