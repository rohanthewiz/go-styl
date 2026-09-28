package eval

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rohanthewiz/go-styl/internal/value"
)

// ErrLimit is wrapped by every error a Sandbox limit produces, so callers can
// tell "this tenant's theme is too expensive or reached for something it may
// not" (errors.Is(err, ErrLimit)) apart from an ordinary compile error.
var ErrLimit = errors.New("sandbox limit exceeded")

// Sandbox holds the resolved (defaults already applied) limits for compiling
// untrusted source. A nil *Sandbox on Options means no sandbox: the
// evaluator behaves exactly as before, and none of the checks below run.
//
// Each limit is <= 0 for "unlimited"; the public styl.Sandbox maps its own
// zero values to defaults before building this, so here 0 really means off.
//
// Why these limits, and why here: the evaluator already caps call depth,
// selector fan-out and range length (eval.go), which stop the classic
// stack/fan-out bombs. What remains for untrusted input is
//
//   - reach:  @import touching files the tenant must not see (OS disk, or
//     FS paths outside an allowlist)
//   - time:   loops that are each bounded but multiply (nested 1..65536)
//   - memory: values that double per statement (s = s + s) and so blow up
//     in ~30 steps, long before a time budget would notice
//   - output: a sheet that is cheap to run but renders megabytes
//
// All checks are cooperative (no goroutine is killed): the step counter
// ticks once per executed statement, which every loop body, mixin and
// function body passes through, so a runaway compile always reaches a check.
type Sandbox struct {
	// Ctx, when non-nil, cancels the compile when done (request scope).
	Ctx context.Context
	// Deadline, when non-zero, is the wall-clock time the compile must
	// finish by.
	Deadline time.Time
	// AllowImport vets each resolved .styl import path (an fs.FS path) before
	// it is read; nil allows every file in Options.FS. Note that in a sandbox
	// Options.FS is the only filesystem: .styl imports without one fail.
	AllowImport func(path string) bool
	// MaxSteps bounds executed statements across the whole compile.
	MaxSteps int
	// MaxOutputBytes bounds the rendered CSS (and the selector text @extend
	// grafts, which is where a small sheet can multiply its output).
	MaxOutputBytes int
	// MaxSourceBytes bounds the total source read by imports. The public
	// layer counts the top-level source against the same budget and passes
	// the remainder here.
	MaxSourceBytes int
	// MaxImports bounds the number of .styl files imported.
	MaxImports int
	// MaxValueBytes bounds the rendered size of any single computed value.
	MaxValueBytes int
}

// sandboxState is the evaluator's running tally against a Sandbox.
type sandboxState struct {
	steps       int
	sourceBytes int
	imports     int
}

// clockEvery is how many steps pass between wall-clock/context checks.
// time.Now and ctx.Err are cheap but not free; statement execution is
// microseconds, so checking every 256 keeps overshoot of a deadline well
// under a millisecond while costing nothing measurable.
const clockEvery = 256

// limitErr builds an ErrLimit-wrapping error.
func limitErr(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrLimit, fmt.Sprintf(format, args...))
}

// LimitErr is limitErr for the public package, which checks the top-level
// source size before the evaluator runs.
func LimitErr(format string, args ...any) error { return limitErr(format, args...) }

// CancelErr wraps a context error so it matches both ErrLimit and the
// context sentinel (context.Canceled / context.DeadlineExceeded).
func CancelErr(err error) error {
	return fmt.Errorf("%w: compile cancelled: %w", ErrLimit, err)
}

// tick charges one executed statement and, periodically, checks the clock
// and context. It is a no-op outside a sandbox.
func (ev *evaluator) tick() error {
	sb := ev.opts.Sandbox
	if sb == nil {
		return nil
	}
	ev.sb.steps++
	if sb.MaxSteps > 0 && ev.sb.steps > sb.MaxSteps {
		return limitErr("more than %d statements executed", sb.MaxSteps)
	}
	if ev.sb.steps%clockEvery == 0 {
		return ev.checkClock()
	}
	return nil
}

// checkClock reports a passed deadline or a cancelled context.
func (ev *evaluator) checkClock() error {
	sb := ev.opts.Sandbox
	if sb == nil {
		return nil
	}
	if !sb.Deadline.IsZero() && time.Now().After(sb.Deadline) {
		return limitErr("compile timed out")
	}
	if sb.Ctx != nil {
		if err := sb.Ctx.Err(); err != nil {
			return CancelErr(err)
		}
	}
	return nil
}

// checkImport vets one resolved import before it is read: the allowlist and
// the import count. The source-size charge happens after the read
// (chargeSource), when the length is known.
func (ev *evaluator) checkImport(imp, resolved string) error {
	sb := ev.opts.Sandbox
	if sb == nil {
		return nil
	}
	if sb.AllowImport != nil && !sb.AllowImport(resolved) {
		return limitErr("@import %q: %q is not allowed", imp, resolved)
	}
	ev.sb.imports++
	if sb.MaxImports > 0 && ev.sb.imports > sb.MaxImports {
		return limitErr("more than %d imports", sb.MaxImports)
	}
	return nil
}

// chargeSource counts n bytes of imported source against the budget.
func (ev *evaluator) chargeSource(imp string, n int) error {
	sb := ev.opts.Sandbox
	if sb == nil {
		return nil
	}
	ev.sb.sourceBytes += n
	if sb.MaxSourceBytes > 0 && ev.sb.sourceBytes > sb.MaxSourceBytes {
		return limitErr("@import %q: total source exceeds %d bytes", imp, sb.MaxSourceBytes)
	}
	return nil
}

// checkOutput caps rendered CSS size.
func (ev *evaluator) checkOutput(css string) error {
	sb := ev.opts.Sandbox
	if sb == nil || sb.MaxOutputBytes <= 0 {
		return nil
	}
	if len(css) > sb.MaxOutputBytes {
		return limitErr("output exceeds %d bytes", sb.MaxOutputBytes)
	}
	return nil
}

// checkValue passes (v, err) through, failing when a sandboxed compile has
// produced a value whose rendered size exceeds MaxValueBytes. It is applied
// to the expression forms that can grow a value — binary ops (concat,
// sprintf), list literals (`l = l l`) and calls (join, push, merge, user
// functions) — which covers every way to double a value per statement.
func (ev *evaluator) checkValue(v value.Value, err error) (value.Value, error) {
	if err != nil || v == nil {
		return v, err
	}
	sb := ev.opts.Sandbox
	if sb == nil || sb.MaxValueBytes <= 0 {
		return v, nil
	}
	if valueSize(v, sb.MaxValueBytes, nil) > sb.MaxValueBytes {
		return nil, limitErr("value exceeds %d bytes", sb.MaxValueBytes)
	}
	return v, nil
}

// valueSize estimates v's rendered size in bytes, stopping as soon as the
// running total passes limit.
//
// The early exit matters for more than speed: lists share items by
// reference, so `l = l l` repeated k times builds a DAG of k nodes whose
// logical (rendered) size is 2^k. The walk counts logical size — that is
// what rendering would materialize — and the cap keeps the walk itself
// bounded by ~limit visits.
//
// Objects are the one mutable, reference-shared value and can contain
// themselves (`o.self = o`); seen counts each *Hash once so a cycle neither
// loops nor gets misreported as huge. (Rendering such an object is its own
// concern; here we only bound growth.)
func valueSize(v value.Value, limit int, seen map[*value.Hash]bool) int {
	switch x := v.(type) {
	case *value.Str:
		return len(x.Val) + 2
	case *value.Ident:
		return len(x.Name)
	case *value.Var:
		return valueSize(x.Inner, limit, seen)
	case *value.SlashList:
		n := valueSize(x.L, limit, seen)
		if n > limit {
			return n
		}
		return n + 1 + valueSize(x.R, limit-n, seen)
	case *value.List:
		total := 0
		for _, it := range x.Items {
			total += valueSize(it, limit-total, seen) + 1
			if total > limit {
				return total
			}
		}
		return total
	case *value.Hash:
		if seen == nil {
			seen = map[*value.Hash]bool{}
		}
		if seen[x] {
			return 0
		}
		seen[x] = true
		total := 2
		for _, k := range x.Keys() {
			item, _ := x.Get(k)
			total += len(k) + 2 + valueSize(item, limit-total, seen)
			if total > limit {
				return total
			}
		}
		return total
	default:
		// Numbers, colors, bools, null: small and fixed-size.
		return 16
	}
}
