package eval

import (
	"errors"
	"fmt"
	"sort"

	"github.com/rohanthewiz/go-styl/internal/diag"
	"github.com/rohanthewiz/go-styl/internal/parser"
	"github.com/rohanthewiz/go-styl/internal/value"
)

// seedGlobals defines Options.Globals in the root scope before the stylesheet
// executes. Names are processed in sorted order so evaluation is
// deterministic; a global's expression may reference globals that sort before
// it, but should not rely on that.
func (ev *evaluator) seedGlobals(scope *Scope) error {
	if len(ev.opts.Globals) == 0 {
		return nil
	}
	names := make([]string, 0, len(ev.opts.Globals))
	for n := range ev.opts.Globals {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		v, err := ev.globalValue(name, ev.opts.Globals[name], scope)
		if err != nil {
			return err
		}
		scope.Set(name, ev.wrapVar(name, scope, v))
	}
	return nil
}

// globalValue converts one Go-supplied global to a Stylus value. Strings are
// parsed and evaluated as Stylus value expressions ("#0af", "10px",
// "1px solid red", "darken(#0af, 10%)"); numbers become unitless numbers.
func (ev *evaluator) globalValue(name string, gv any, scope *Scope) (value.Value, error) {
	switch g := gv.(type) {
	case string:
		e, err := parser.ParseExpr(g, 0)
		if err != nil {
			return nil, globalErr(name, err)
		}
		v, err := ev.evalExpr(e, scope)
		if err != nil {
			return nil, globalErr(name, err)
		}
		return v, nil
	case int:
		return &value.Number{Num: float64(g)}, nil
	case int8:
		return &value.Number{Num: float64(g)}, nil
	case int16:
		return &value.Number{Num: float64(g)}, nil
	case int32:
		return &value.Number{Num: float64(g)}, nil
	case int64:
		return &value.Number{Num: float64(g)}, nil
	case uint:
		return &value.Number{Num: float64(g)}, nil
	case uint8:
		return &value.Number{Num: float64(g)}, nil
	case uint16:
		return &value.Number{Num: float64(g)}, nil
	case uint32:
		return &value.Number{Num: float64(g)}, nil
	case uint64:
		return &value.Number{Num: float64(g)}, nil
	case float32:
		return &value.Number{Num: float64(g)}, nil
	case float64:
		return &value.Number{Num: g}, nil
	case bool:
		return &value.Bool{Val: g}, nil
	case nil:
		return value.Null{}, nil
	default:
		return nil, fmt.Errorf("global %q: unsupported Go type %T", name, gv)
	}
}

// globalErr rewraps a parse/eval error from a global's expression without its
// source position — the expression came from Go code, not the stylesheet, so
// a file:line:col prefix would mislead.
func globalErr(name string, err error) error {
	msg := err.Error()
	var de *diag.Error
	if errors.As(err, &de) {
		msg = de.Msg
	}
	return fmt.Errorf("global %q: %s", name, msg)
}
