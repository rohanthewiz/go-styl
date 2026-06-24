package eval

import (
	"strings"

	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/css"
)

// evalAtRule dispatches an at-rule by kind:
//
//	leaf (no body)            -> verbatim passthrough line (@charset, @namespace…)
//	@font-face / @page        -> a declaration block keyed by the at-rule header
//	@keyframes (+ vendored)   -> frame rules, never combined with the parent
//	@media / @supports / …    -> nested rules, bubbling the enclosing selector
func (ev *evaluator) evalAtRule(s *ast.AtRule, ctx *execCtx) error {
	params, err := ev.interpolate(s.Params, ctx.scope)
	if err != nil {
		return err
	}
	header := "@" + s.Name
	if params != "" {
		header += " " + params
	}

	if s.Body == nil {
		*ctx.sink = append(*ctx.sink, &css.RawNode{Text: header + ";"})
		return nil
	}

	switch atKind(s.Name) {
	case "font-face", "page", "viewport":
		return ev.evalAtDeclBlock(header, s.Body, ctx)
	case "keyframes":
		return ev.evalKeyframes(header, s.Body, ctx)
	default:
		return ev.evalAtNested(header, s.Body, ctx)
	}
}

// evalAtDeclBlock renders an at-rule whose body is a set of declarations, e.g.
// @font-face. The header itself stands in for the selector.
func (ev *evaluator) evalAtDeclBlock(header string, body []ast.Stmt, ctx *execCtx) error {
	rule := &css.Rule{Selector: header, Selectors: []string{header}}
	*ctx.sink = append(*ctx.sink, rule)
	ev.rules = append(ev.rules, rule)

	child := &execCtx{scope: ctx.scope.Child(), rule: rule, sink: ctx.sink, dir: ctx.dir}
	return ev.execBlock(body, child)
}

// evalKeyframes renders @keyframes: its frame selectors (0%, from, to, …) are
// emitted as-is, never combined with an enclosing selector.
func (ev *evaluator) evalKeyframes(header string, body []ast.Stmt, ctx *execCtx) error {
	atr := &css.AtRule{Header: header}
	*ctx.sink = append(*ctx.sink, atr)

	child := &execCtx{scope: ctx.scope.Child(), sink: &atr.Nodes, dir: ctx.dir}
	return ev.execBlock(body, child)
}

// evalAtNested renders @media/@supports-style blocks. When nested inside a
// selector, the enclosing selectors bubble inward so bare declarations attach to
// them (Stylus media bubbling).
func (ev *evaluator) evalAtNested(header string, body []ast.Stmt, ctx *execCtx) error {
	atr := &css.AtRule{Header: ev.compactAtHeader(header)}
	*ctx.sink = append(*ctx.sink, atr)

	child := &execCtx{scope: ctx.scope.Child(), parents: ctx.parents, sink: &atr.Nodes, dir: ctx.dir}
	if len(ctx.parents) > 0 {
		rule := &css.Rule{Selector: joinSelectors(ctx.parents, ev.opts.Pretty), Selectors: ctx.parents}
		*child.sink = append(*child.sink, rule)
		ev.rules = append(ev.rules, rule)
		child.rule = rule
	}
	return ev.execBlock(body, child)
}

// atKind strips a vendor prefix (e.g. -webkit-keyframes -> keyframes) so at-rules
// dispatch on their canonical name.
func atKind(name string) string {
	if strings.HasPrefix(name, "-") {
		if i := strings.IndexByte(name[1:], '-'); i >= 0 {
			return name[i+2:]
		}
	}
	return name
}

// compactAtHeader compresses whitespace inside the parenthesised parts of an
// at-rule header for compressed output (e.g. "@media (min-width: 768px)" ->
// "@media (min-width:768px)"). Pretty output is returned unchanged.
func (ev *evaluator) compactAtHeader(header string) string {
	if ev.opts.Pretty {
		return header
	}
	var b strings.Builder
	depth := 0
	for _, c := range header {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
		case ' ':
			// Drop a space that follows ':' or ',' inside parentheses.
			if depth > 0 && b.Len() > 0 {
				if prev := b.String()[b.Len()-1]; prev == ':' || prev == ',' {
					continue
				}
			}
		}
		b.WriteRune(c)
	}
	return b.String()
}
