package styl_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/rohanthewiz/go-styl"
)

// TestSandboxOrdinarySheet: a normal theme compiles identically with and
// without the default sandbox.
func TestSandboxOrdinarySheet(t *testing.T) {
	src := "primary = #0af\n" +
		"pad(n)\n  padding n\n" +
		"for i in 1..10\n  .m-{i}\n    margin (i * 4px)\n" +
		".btn\n  color primary\n  pad(4px)\n  background darken(primary, 10%)\n"
	plain, err := styl.Compile(src, styl.Options{})
	if err != nil {
		t.Fatal(err)
	}
	sandboxed, err := styl.Compile(src, styl.Options{Sandbox: &styl.Sandbox{}})
	if err != nil {
		t.Fatalf("sandboxed compile: %v", err)
	}
	if plain != sandboxed {
		t.Errorf("sandbox changed output:\n%s\nvs\n%s", plain, sandboxed)
	}
}

// TestSandboxLimits: each hostile pattern trips its limit with an error
// matching ErrLimit, and the same sheet without a sandbox would not (or would
// not quickly) fail.
func TestSandboxLimits(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		sb      styl.Sandbox
		wantMsg string
	}{
		{
			// Each loop is within maxRangeLen, but they multiply to 4e9.
			name:    "nested loops hit MaxSteps",
			src:     "for i in 1..65536\n  for j in 1..65536\n    x = i\n",
			wantMsg: "statements executed",
		},
		{
			name:    "custom MaxSteps",
			src:     "for i in 1..100\n  x = i\n",
			sb:      styl.Sandbox{MaxSteps: 50},
			wantMsg: "more than 50 statements",
		},
		{
			name: "timeout",
			src:  "for i in 1..65536\n  for j in 1..65536\n    x = i\n",
			// Steps unlimited so only the clock can stop it.
			sb:      styl.Sandbox{MaxSteps: -1, Timeout: 20 * time.Millisecond},
			wantMsg: "timed out",
		},
		{
			// Doubling: 2^40 bytes if unchecked.
			name:    "string doubling hits MaxValueBytes",
			src:     "s = 'ab'\nfor i in 1..40\n  s = s + s\n",
			wantMsg: "value exceeds",
		},
		{
			// Shared sub-lists: tiny in memory, 2^40 items rendered.
			name:    "list doubling hits MaxValueBytes",
			src:     "l = a b\nfor i in 1..40\n  l = l l\n",
			wantMsg: "value exceeds",
		},
		{
			name:    "recursive function doubling",
			src:     "f(s, n)\n  if n > 0\n    return f(s + s, n - 1)\n  return s\n.a\n  b f('x', 60)\n",
			wantMsg: "value exceeds",
		},
		{
			name:    "output cap",
			src:     "for i in 1..2000\n  .c{i}\n    color red\n",
			sb:      styl.Sandbox{MaxOutputBytes: 1000},
			wantMsg: "output exceeds",
		},
		{
			// 200 extends x 200 shared rules = 40,000 grafted selectors.
			name: "extend fan-out charged to output",
			src: "for i in 1..200\n  .t\n    color red\n" +
				"for i in 1..200\n  .e{i}\n    @extend .t\n",
			sb:      styl.Sandbox{MaxOutputBytes: 10000},
			wantMsg: "@extend output exceeds",
		},
		{
			name:    "source cap",
			src:     strings.Repeat(".a\n  color red\n", 100),
			sb:      styl.Sandbox{MaxSourceBytes: 100},
			wantMsg: "source exceeds",
		},
		{
			name:    "no OS imports",
			src:     "@import 'anything'\n",
			wantMsg: "imports need Options.FS",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sb := c.sb
			start := time.Now()
			_, err := styl.Compile(c.src, styl.Options{Sandbox: &sb})
			if err == nil {
				t.Fatal("expected a sandbox error")
			}
			if !errors.Is(err, styl.ErrLimit) {
				t.Errorf("error does not match ErrLimit: %v", err)
			}
			if !strings.Contains(err.Error(), c.wantMsg) {
				t.Errorf("error %q does not mention %q", err, c.wantMsg)
			}
			if d := time.Since(start); d > 3*time.Second {
				t.Errorf("limit took %v to trip", d)
			}
		})
	}
}

// TestSandboxErrorPositioned: a limit error inside the sheet carries the
// statement's position like any compile error.
func TestSandboxErrorPositioned(t *testing.T) {
	_, err := styl.Compile("x = 1\ns = 'ab'\nfor i in 1..40\n  s = s + s\n",
		styl.Options{Filename: "theme.styl", Sandbox: &styl.Sandbox{}})
	if err == nil || !strings.Contains(err.Error(), "theme.styl:4:") {
		t.Fatalf("want error positioned at theme.styl:4, got %v", err)
	}
}

// TestSandboxContext: a cancelled context stops the compile, and the error
// matches both ErrLimit and context.Canceled.
func TestSandboxContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := styl.Compile(".a\n  color red\n",
		styl.Options{Sandbox: &styl.Sandbox{Context: ctx}})
	if !errors.Is(err, styl.ErrLimit) || !errors.Is(err, context.Canceled) {
		t.Fatalf("want ErrLimit + context.Canceled, got %v", err)
	}

	// Cancelled mid-compile.
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = styl.Compile("for i in 1..65536\n  for j in 1..65536\n    x = i\n",
		styl.Options{Sandbox: &styl.Sandbox{Context: ctx, MaxSteps: -1, Timeout: -1}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want context.DeadlineExceeded, got %v", err)
	}
}

// TestSandboxImports: imports resolve through Options.FS only, AllowImport
// vets them, and imported source counts against the budgets.
func TestSandboxImports(t *testing.T) {
	fsys := fstest.MapFS{
		"shared/vars.styl":   {Data: []byte("primary = #0af\n")},
		"shared/mixins.styl": {Data: []byte("pad(n)\n  padding n\n")},
		"secret/keys.styl":   {Data: []byte("k = 1\n")},
		"big.styl":           {Data: []byte(strings.Repeat("// padding\n", 200))},
	}
	allowShared := func(p string) bool { return strings.HasPrefix(p, "shared/") }

	t.Run("allowed", func(t *testing.T) {
		out, err := styl.Compile("@import 'shared/vars'\n@import 'shared/mixins'\n.a\n  color primary\n  pad(2px)\n",
			styl.Options{FS: fsys, Sandbox: &styl.Sandbox{AllowImport: allowShared}})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "color:#0af") || !strings.Contains(out, "padding:2px") {
			t.Errorf("unexpected output %q", out)
		}
	})
	t.Run("denied by AllowImport", func(t *testing.T) {
		_, err := styl.Compile("@import 'secret/keys'\n",
			styl.Options{FS: fsys, Sandbox: &styl.Sandbox{AllowImport: allowShared}})
		if !errors.Is(err, styl.ErrLimit) || !strings.Contains(err.Error(), "not allowed") {
			t.Fatalf("want not-allowed error, got %v", err)
		}
	})
	t.Run("glob denied if any match is", func(t *testing.T) {
		_, err := styl.Compile("@import '*/*'\n",
			styl.Options{FS: fsys, Sandbox: &styl.Sandbox{AllowImport: allowShared}})
		if !errors.Is(err, styl.ErrLimit) {
			t.Fatalf("want ErrLimit, got %v", err)
		}
	})
	t.Run("parent escape", func(t *testing.T) {
		_, err := styl.Compile("@import '../etc/passwd'\n",
			styl.Options{FS: fsys, BaseDir: "shared", Sandbox: &styl.Sandbox{}})
		if err == nil {
			t.Fatal("expected an error")
		}
	})
	t.Run("MaxImports", func(t *testing.T) {
		_, err := styl.Compile("@import 'shared/vars'\n@import 'shared/vars'\n",
			styl.Options{FS: fsys, Sandbox: &styl.Sandbox{MaxImports: 1}})
		if !errors.Is(err, styl.ErrLimit) || !strings.Contains(err.Error(), "more than 1 imports") {
			t.Fatalf("want MaxImports error, got %v", err)
		}
	})
	t.Run("import source counts", func(t *testing.T) {
		_, err := styl.Compile("@import 'big'\n",
			styl.Options{FS: fsys, Sandbox: &styl.Sandbox{MaxSourceBytes: 500}})
		if !errors.Is(err, styl.ErrLimit) || !strings.Contains(err.Error(), "total source exceeds") {
			t.Fatalf("want source budget error, got %v", err)
		}
	})
	t.Run("CompileFile from FS", func(t *testing.T) {
		if _, err := styl.CompileFile("shared/vars.styl",
			styl.Options{FS: fsys, Sandbox: &styl.Sandbox{}}); err != nil {
			t.Fatal(err)
		}
	})
}

// TestSandboxNoOSFiles: with no FS, neither the entry file nor an import may
// touch the OS disk — even one that exists.
func TestSandboxNoOSFiles(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "real.styl")
	if err := os.WriteFile(p, []byte(".a\n  color red\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := styl.CompileFile(p, styl.Options{Sandbox: &styl.Sandbox{}}); !errors.Is(err, styl.ErrLimit) {
		t.Errorf("CompileFile read the OS disk: %v", err)
	}
	_, err := styl.Compile("@import 'real'\n", styl.Options{BaseDir: dir, Sandbox: &styl.Sandbox{}})
	if !errors.Is(err, styl.ErrLimit) {
		t.Errorf("@import read the OS disk: %v", err)
	}
	// Control: without a sandbox the import works.
	if _, err := styl.Compile("@import 'real'\n", styl.Options{BaseDir: dir}); err != nil {
		t.Errorf("control compile: %v", err)
	}
}

// TestSandboxWarn: warn() never reaches stderr in a sandbox, but still
// reaches an explicit Warn hook.
func TestSandboxWarn(t *testing.T) {
	var got []string
	_, err := styl.Compile("warn('hi')\n", styl.Options{
		Sandbox: &styl.Sandbox{},
		Warn:    func(m string) { got = append(got, m) },
	})
	if err != nil || len(got) != 1 || got[0] != "hi" {
		t.Fatalf("Warn hook: got %v, err %v", got, err)
	}
	// No hook: dropped (no stderr assertion is practical; just no error).
	if _, err := styl.Compile("warn('hi')\n", styl.Options{Sandbox: &styl.Sandbox{}}); err != nil {
		t.Fatal(err)
	}
}

// TestSandboxOtherEntryPoints: the sandbox applies to every compile entry
// point, not just Compile.
func TestSandboxOtherEntryPoints(t *testing.T) {
	bomb := "s = 'ab'\nfor i in 1..40\n  s = s + s\n"
	opts := styl.Options{Sandbox: &styl.Sandbox{}}
	check := func(name string, err error) {
		t.Helper()
		if !errors.Is(err, styl.ErrLimit) {
			t.Errorf("%s: want ErrLimit, got %v", name, err)
		}
	}
	_, err := styl.Build(bomb, opts)
	check("Build", err)
	_, _, err = styl.CompileMap(bomb, opts)
	check("CompileMap", err)
	_, err = styl.Extract(bomb, opts)
	check("Extract", err)
	_, err = styl.Prune(bomb, styl.Used{}, opts)
	check("Prune", err)
	_, err = styl.Component(bomb, opts)
	check("Component", err)
	_, err = styl.Migrate(bomb, opts, styl.MigrateOptions{})
	check("Migrate", err)
}

// TestSandboxSelfReferentialObject: a cyclic object doesn't hang the value
// size check.
func TestSandboxSelfReferentialObject(t *testing.T) {
	_, err := styl.Compile("o = {a: 1}\no.self = o\nk = keys(o)\n",
		styl.Options{Sandbox: &styl.Sandbox{}})
	if err != nil {
		t.Fatal(err)
	}
}
