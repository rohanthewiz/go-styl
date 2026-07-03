package eval

import (
	"github.com/rohanthewiz/go-styl/internal/css"
	"github.com/rohanthewiz/go-styl/internal/value"
)

// wrapVar wraps a value bound to a CustomProperties-listed name in the root
// scope, so direct references render var(--name). Bindings in nested scopes
// (rule-local variables, function parameters) stay concrete — only root-level
// tokens become custom properties.
func (ev *evaluator) wrapVar(name string, scope *Scope, v value.Value) value.Value {
	if scope != ev.rootScope || !ev.customProps[name] {
		return v
	}
	return &value.Var{Name: name, Inner: value.Deref(v)}
}

// customPropsRule builds the :root rule declaring each exposed variable as a
// CSS custom property, in Options.CustomProperties order. Returns nil when
// none of the listed names ended up defined.
func (ev *evaluator) customPropsRule() *css.Rule {
	rule := &css.Rule{
		Selector:  ":root",
		Selectors: []string{":root"},
		Pos:       css.Pos{Line: 1, Col: 1},
	}
	for _, name := range ev.opts.CustomProperties {
		v, ok := ev.rootScope.Get(name)
		if !ok {
			continue
		}
		rule.Statements = append(rule.Statements, &css.Statement{
			Property: "--" + name,
			Value:    value.Deref(v).CSS(ev.opts.Pretty),
			Pos:      css.Pos{Line: 1, Col: 1},
		})
	}
	if len(rule.Statements) == 0 {
		return nil
	}
	return rule
}
