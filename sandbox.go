package styl

import (
	"context"
	"time"

	"github.com/rohanthewiz/go-styl/internal/eval"
)

// ErrLimit is wrapped by every error a Sandbox produces — a budget ran out
// (steps, time, output, source, imports, value size), the context was
// cancelled, or an import was refused. Test with errors.Is(err, ErrLimit) to
// tell "reject this tenant's theme" apart from an ordinary compile error
// (which the author can fix by editing the sheet). Cancellation errors also
// match context.Canceled / context.DeadlineExceeded.
var ErrLimit = eval.ErrLimit

// Sandbox confines a compile of untrusted Stylus — user-submitted themes in a
// multi-tenant app. Set Options.Sandbox to enable it; a nil Sandbox leaves
// every check off and compiles exactly as before.
//
// Inside a sandbox:
//
//   - there is no OS filesystem. .styl imports resolve only through
//     Options.FS (which the host controls, e.g. an embed.FS of shared
//     partials or an fstest.MapFS of the tenant's files), and fail without
//     one; CompileFile & co. refuse to read the OS disk. fs.FS paths cannot
//     climb out with "..", so FS is the tenant's whole world.
//   - AllowImport, when set, vets each resolved import path.
//   - Timeout / Context bound wall-clock time, and MaxSteps the executed
//     statement count (a deterministic budget: the same sheet passes or fails
//     regardless of machine load).
//   - MaxValueBytes bounds any computed value, which stops doubling bombs
//     (s = s + s in a loop) in the ~20 steps they need to exhaust memory.
//   - MaxSourceBytes, MaxImports and MaxOutputBytes bound input and output.
//   - warn() output goes to Options.Warn when set and is otherwise dropped,
//     never to the host's stderr.
//
// These add to the limits every compile already has (call depth 256,
// 16384 selectors per rule, ranges of at most 65536 items), and
// JavaScript plugins (use()) are never supported. Checks are cooperative —
// no goroutine is started or killed — so a limit trips within one statement
// (or 256 statements for the clock).
//
// For each numeric field, 0 selects the default shown and a negative value
// means unlimited. A Sandbox holds no state, so one value can be shared by
// every compile.
//
// Literal imports (`@import "x.css"`, `@import url(…)`) are not file reads;
// they pass through to the output as-is, as in any compile.
type Sandbox struct {
	// Context, when set, cancels the compile when it is done — typically the
	// HTTP request's context.
	Context context.Context
	// Timeout bounds the compile's wall-clock time. Default 2s.
	Timeout time.Duration
	// AllowImport, when set, is called with each resolved .styl import path
	// (a slash-separated Options.FS path); returning false fails the compile.
	// nil allows every file in Options.FS.
	AllowImport func(path string) bool
	// MaxSteps bounds executed statements, counting each pass through loop,
	// mixin and function bodies. Default 1,000,000.
	MaxSteps int
	// MaxSourceBytes bounds the total source compiled: the top-level sheet
	// plus every import. Default 1 MiB.
	MaxSourceBytes int
	// MaxImports bounds the number of .styl files imported. Default 256.
	MaxImports int
	// MaxValueBytes bounds the rendered size of any single computed value.
	// Default 256 KiB.
	MaxValueBytes int
	// MaxOutputBytes bounds the generated CSS. Default 4 MiB.
	MaxOutputBytes int
}

// Sandbox defaults. They are generous for a real theme (a large framework
// stylesheet compiles in well under 1% of MaxSteps) while keeping a
// hostile one to about a CPU-second and a few MiB.
const (
	defaultSandboxTimeout = 2 * time.Second
	defaultMaxSteps       = 1_000_000
	defaultMaxSourceBytes = 1 << 20
	defaultMaxImports     = 256
	defaultMaxValueBytes  = 256 << 10
	defaultMaxOutputBytes = 4 << 20
)

// limitOr resolves a limit field: 0 means the default, negative unlimited
// (0 to the evaluator, which treats <= 0 as off).
func limitOr(v, def int) int {
	switch {
	case v == 0:
		return def
	case v < 0:
		return 0
	default:
		return v
	}
}

// sandbox resolves Options.Sandbox for one compile of srcLen bytes of
// top-level source. It returns nil (no sandbox) when none is configured.
//
// It runs at the start of every entry point, before parsing, for two
// reasons: the deadline is anchored to the moment this compile begins
// (a Sandbox value is shared config, so it can't carry one), and the
// top-level source is charged against MaxSourceBytes before the parser
// spends any time on an oversized input. The evaluator then receives only
// the remaining source budget for imports.
func (o Options) sandbox(srcLen int) (*eval.Sandbox, error) {
	s := o.Sandbox
	if s == nil {
		return nil, nil
	}
	sb := &eval.Sandbox{
		Ctx:            s.Context,
		AllowImport:    s.AllowImport,
		MaxSteps:       limitOr(s.MaxSteps, defaultMaxSteps),
		MaxImports:     limitOr(s.MaxImports, defaultMaxImports),
		MaxValueBytes:  limitOr(s.MaxValueBytes, defaultMaxValueBytes),
		MaxOutputBytes: limitOr(s.MaxOutputBytes, defaultMaxOutputBytes),
	}
	switch {
	case s.Timeout == 0:
		sb.Deadline = time.Now().Add(defaultSandboxTimeout)
	case s.Timeout > 0:
		sb.Deadline = time.Now().Add(s.Timeout)
	}
	if s.Context != nil {
		if err := s.Context.Err(); err != nil {
			return nil, eval.CancelErr(err)
		}
	}
	if maxSrc := limitOr(s.MaxSourceBytes, defaultMaxSourceBytes); maxSrc > 0 {
		if srcLen > maxSrc {
			return nil, eval.LimitErr("source exceeds %d bytes", maxSrc)
		}
		// Imports get what the top-level sheet left. The remainder is at
		// least 1 so that 0 keeps meaning "unlimited" to the evaluator:
		// a sheet that used the whole budget can still import nothing
		// non-empty.
		sb.MaxSourceBytes = max(maxSrc-srcLen, 1)
	}
	return sb, nil
}
