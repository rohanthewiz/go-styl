// Package eval walks the AST, resolving variables/expressions to values and
// flattening nested rulesets into a CSS rule tree.
package eval

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/builtin"
	"github.com/rohanthewiz/go-styl/internal/css"
	"github.com/rohanthewiz/go-styl/internal/parser"
	"github.com/rohanthewiz/go-styl/internal/token"
	"github.com/rohanthewiz/go-styl/internal/value"
)

// Options controls evaluation/rendering.
type Options struct {
	Pretty          bool
	MergeDuplicates bool
	BaseDir         string   // directory @import paths resolve against
	IncludePaths    []string // extra directories searched for @import
}

// extendReq records a pending @extend: graft Extenders onto every rule matching
// Target (a selector or "$placeholder" name).
type extendReq struct {
	extenders []string
	target    string
}

type evaluator struct {
	opts         Options
	rules        []*css.Rule          // output rules in encounter order
	raws         []css.Node           // passthrough nodes (literal @imports), emitted first
	placeholders map[string]*css.Rule // $name -> placeholder rule
	extends      []extendReq
	importing    map[string]bool // absolute paths currently being imported (cycle guard)
}

// execCtx captures where statements emit while a block executes: the active
// variable/function scope, the rule that declarations append to (nil when not
// inside a selector), and the selector context for nested rulesets. ret/returned
// carry a function body's return value.
type execCtx struct {
	scope    *Scope
	rule     *css.Rule
	parents  []string
	dir      string // directory @import paths in this block resolve against
	ret      value.Value
	returned bool
}

// Evaluate evaluates a stylesheet and returns the rendered CSS.
func Evaluate(sheet *ast.Stylesheet, opts Options) (string, error) {
	ev := &evaluator{
		opts:         opts,
		placeholders: map[string]*css.Rule{},
		importing:    map[string]bool{},
	}
	ctx := &execCtx{scope: NewScope(), dir: opts.BaseDir}

	if err := ev.execBlock(sheet.Statements, ctx); err != nil {
		return "", err
	}

	ev.applyExtends()

	rules := ev.rules
	if opts.MergeDuplicates {
		rules = css.MergeDuplicates(rules)
	}

	// Passthrough @imports come first (CSS requires @import before other rules).
	sheetOut := &css.Stylesheet{Nodes: make([]css.Node, 0, len(ev.raws)+len(rules))}
	sheetOut.Nodes = append(sheetOut.Nodes, ev.raws...)
	for _, r := range rules {
		sheetOut.Nodes = append(sheetOut.Nodes, r)
	}
	return sheetOut.Render(opts.Pretty), nil
}

// applyExtends grafts each @extend's selectors onto every matching target rule.
func (ev *evaluator) applyExtends() {
	for _, ex := range ev.extends {
		targets := ev.findExtendTargets(ex.target)
		for _, t := range targets {
			t.Extenders = append(t.Extenders, ex.extenders...)
		}
	}
}

// findExtendTargets returns the rules an @extend should attach to: the registered
// placeholder for a "$name" target, otherwise every rule carrying that selector.
func (ev *evaluator) findExtendTargets(target string) []*css.Rule {
	if strings.HasPrefix(target, "$") {
		if r, ok := ev.placeholders[target]; ok {
			return []*css.Rule{r}
		}
		return nil
	}
	var out []*css.Rule
	for _, r := range ev.rules {
		if slices.Contains(r.Selectors, target) {
			out = append(out, r)
		}
	}
	return out
}

// execBlock executes a list of statements within ctx, stopping early if a return
// fires.
func (ev *evaluator) execBlock(stmts []ast.Stmt, ctx *execCtx) error {
	for _, stmt := range stmts {
		if ctx.returned {
			return nil
		}
		if err := ev.execStmt(stmt, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (ev *evaluator) execStmt(stmt ast.Stmt, ctx *execCtx) error {
	switch s := stmt.(type) {
	case *ast.Assignment:
		return ev.evalAssignment(s, ctx.scope)
	case *ast.Declaration:
		if ctx.rule == nil {
			return fmt.Errorf("property %q must appear inside a selector", s.Property)
		}
		prop, err := ev.interpolate(s.Property, ctx.scope)
		if err != nil {
			return err
		}
		v, err := ev.evalExpr(s.Value, ctx.scope)
		if err != nil {
			return err
		}
		ctx.rule.Statements = append(ctx.rule.Statements, &css.Statement{
			Property: prop,
			Value:    v.CSS(ev.opts.Pretty),
		})
		return nil
	case *ast.RuleSet:
		return ev.evalRuleSet(s, ctx)
	case *ast.FuncDef:
		ctx.scope.SetFunc(s.Name, &Closure{Def: s, Scope: ctx.scope})
		return nil
	case *ast.MixinCall:
		return ev.evalMixinCall(s, ctx)
	case *ast.If:
		return ev.evalIf(s, ctx)
	case *ast.For:
		return ev.evalFor(s, ctx)
	case *ast.Return:
		if s.Value == nil {
			ctx.ret = value.Null{}
		} else {
			v, err := ev.evalExpr(s.Value, ctx.scope)
			if err != nil {
				return err
			}
			ctx.ret = v
		}
		ctx.returned = true
		return nil
	case *ast.Extend:
		return ev.evalExtend(s, ctx)
	case *ast.Import:
		return ev.evalImport(s, ctx)
	default:
		return fmt.Errorf("unsupported statement %T", stmt)
	}
}

func (ev *evaluator) evalAssignment(a *ast.Assignment, scope *Scope) error {
	if a.Op == token.ASSIGNQ && scope.Has(a.Name) {
		return nil // ?= only defines when absent
	}
	v, err := ev.evalExpr(a.Value, scope)
	if err != nil {
		return err
	}
	scope.Set(a.Name, v)
	return nil
}

// evalRuleSet resolves a ruleset's selectors against its parents, emits a rule
// for its own declarations, and recurses into nested rulesets. Selectors carrying
// `{...}` interpolation are resolved here; a `$name` selector marks a placeholder
// rule (emitted only when extended).
func (ev *evaluator) evalRuleSet(rs *ast.RuleSet, ctx *execCtx) error {
	selfs := make([]string, len(rs.Selectors))
	for i, s := range rs.Selectors {
		r, err := ev.interpolate(s, ctx.scope)
		if err != nil {
			return err
		}
		selfs[i] = r
	}
	combined := combineSelectors(ctx.parents, selfs, ev.opts.Pretty)

	rule := &css.Rule{
		Selector:  joinSelectors(combined, ev.opts.Pretty),
		Selectors: combined,
	}
	if allPlaceholders(combined) {
		rule.Placeholder = true
		for _, s := range combined {
			ev.placeholders[s] = rule
		}
	}
	ev.rules = append(ev.rules, rule)

	child := &execCtx{scope: ctx.scope.Child(), rule: rule, parents: combined, dir: ctx.dir}
	if err := ev.execBlock(rs.Body, child); err != nil {
		return err
	}
	// Propagate a return that fired inside the ruleset body (e.g. within a function).
	if child.returned {
		ctx.ret = child.ret
		ctx.returned = true
	}
	return nil
}

// evalIf evaluates the first true branch (or the else body) in the current
// context, so its declarations/assignments take effect where the if appears.
func (ev *evaluator) evalIf(s *ast.If, ctx *execCtx) error {
	for _, br := range s.Branches {
		cond, err := ev.evalExpr(br.Cond, ctx.scope)
		if err != nil {
			return err
		}
		if value.Truthy(cond) {
			return ev.execBlock(br.Body, ctx)
		}
	}
	if s.Else != nil {
		return ev.execBlock(s.Else, ctx)
	}
	return nil
}

// evalFor iterates the value list, binding the loop variable(s) and running the
// body once per item in the current context.
func (ev *evaluator) evalFor(s *ast.For, ctx *execCtx) error {
	iter, err := ev.evalExpr(s.Iterable, ctx.scope)
	if err != nil {
		return err
	}
	for idx, item := range iterItems(iter) {
		if s.Index != "" {
			ctx.scope.Set(s.Index, &value.Number{Num: float64(idx)})
		}
		ctx.scope.Set(s.Value, item)
		if err := ev.execBlock(s.Body, ctx); err != nil {
			return err
		}
		if ctx.returned {
			break
		}
	}
	return nil
}

// evalMixinCall invokes a function/mixin in statement position, emitting its body
// into the current rule and selector context.
func (ev *evaluator) evalMixinCall(s *ast.MixinCall, ctx *execCtx) error {
	cl, ok := ctx.scope.GetFunc(s.Name)
	if !ok {
		return fmt.Errorf("undefined mixin %q", s.Name)
	}
	args, err := ev.evalArgs(s.Args, ctx.scope)
	if err != nil {
		return err
	}
	_, err = ev.invoke(cl, args, ctx.rule, ctx.parents)
	return err
}

// invoke runs a closure's body. emitRule/emitParents are the caller's emission
// context for mixin (statement) use; pass nil for a pure function (expression)
// call. The return value is the body's `return` value, or Null if none.
func (ev *evaluator) invoke(cl *Closure, args []value.Value, emitRule *css.Rule, emitParents []string) (value.Value, error) {
	fscope := cl.Scope.Child()
	if err := ev.bindParams(fscope, cl.Def.Params, args); err != nil {
		return nil, err
	}
	fctx := &execCtx{scope: fscope, rule: emitRule, parents: emitParents}
	if err := ev.execBlock(cl.Def.Body, fctx); err != nil {
		return nil, err
	}
	if fctx.ret != nil {
		return fctx.ret, nil
	}
	return value.Null{}, nil
}

// bindParams binds call arguments to parameters, honoring defaults and a trailing
// rest parameter.
func (ev *evaluator) bindParams(scope *Scope, params []ast.Param, args []value.Value) error {
	ai := 0
	for _, p := range params {
		switch {
		case p.Rest:
			var rest []value.Value
			if ai < len(args) {
				rest = args[ai:]
			}
			scope.Set(p.Name, &value.List{Items: rest, Comma: true})
			ai = len(args)
		case ai < len(args):
			scope.Set(p.Name, args[ai])
			ai++
		case p.Default != nil:
			dv, err := ev.evalExpr(p.Default, scope)
			if err != nil {
				return err
			}
			scope.Set(p.Name, dv)
		default:
			scope.Set(p.Name, value.Null{})
		}
	}
	return nil
}

func (ev *evaluator) evalArgs(exprs []ast.Expr, scope *Scope) ([]value.Value, error) {
	args := make([]value.Value, len(exprs))
	for i, e := range exprs {
		v, err := ev.evalExpr(e, scope)
		if err != nil {
			return nil, err
		}
		args[i] = v
	}
	return args, nil
}

// iterItems returns the items a for-loop should iterate: a list's elements, or
// the single value itself.
func iterItems(v value.Value) []value.Value {
	if l, ok := v.(*value.List); ok {
		return l.Items
	}
	return []value.Value{v}
}

// --- expression evaluation ---

func (ev *evaluator) evalExpr(e ast.Expr, scope *Scope) (value.Value, error) {
	switch x := e.(type) {
	case *ast.NumberLit:
		return value.ParseNumber(x.Text)
	case *ast.ColorLit:
		return value.ParseColor(x.Text)
	case *ast.StringLit:
		val, err := ev.interpolate(x.Value, scope)
		if err != nil {
			return nil, err
		}
		return &value.Str{Val: val, Quote: x.Quote}, nil
	case *ast.Ident:
		// Boolean/null literals.
		switch x.Name {
		case "true":
			return &value.Bool{Val: true}, nil
		case "false":
			return &value.Bool{Val: false}, nil
		case "null":
			return value.Null{}, nil
		}
		// Interpolated identifier: a lone `{expr}` yields the value itself; a mixed
		// form like `Arial-{weight}` yields a substituted bare keyword.
		if strings.Contains(x.Name, "{") {
			if inner, ok := wholeInterp(x.Name); ok {
				e, err := parser.ParseExpr(strings.TrimSpace(inner), 0)
				if err != nil {
					return nil, err
				}
				return ev.evalExpr(e, scope)
			}
			s, err := ev.interpolate(x.Name, scope)
			if err != nil {
				return nil, err
			}
			return &value.Ident{Name: s}, nil
		}
		// Variable reference inlines its value; otherwise it's a bare keyword.
		if v, ok := scope.Get(x.Name); ok {
			return v, nil
		}
		return &value.Ident{Name: x.Name}, nil
	case *ast.Unary:
		return ev.evalUnary(x, scope)
	case *ast.Binary:
		return ev.evalBinary(x, scope)
	case *ast.List:
		items := make([]value.Value, len(x.Items))
		for i, it := range x.Items {
			v, err := ev.evalExpr(it, scope)
			if err != nil {
				return nil, err
			}
			items[i] = v
		}
		return &value.List{Items: items, Comma: x.Comma}, nil
	case *ast.Call:
		return ev.evalCall(x, scope)
	default:
		return nil, fmt.Errorf("cannot evaluate expression %T", e)
	}
}

func (ev *evaluator) evalUnary(u *ast.Unary, scope *Scope) (value.Value, error) {
	v, err := ev.evalExpr(u.X, scope)
	if err != nil {
		return nil, err
	}
	switch u.Op {
	case token.MINUS:
		n, ok := v.(*value.Number)
		if !ok {
			return nil, fmt.Errorf("cannot negate %s", v.TypeName())
		}
		return &value.Number{Num: -n.Num, Unit: n.Unit}, nil
	case token.PLUS:
		return v, nil
	case token.NOT:
		return &value.Bool{Val: !value.Truthy(v)}, nil
	default:
		return nil, fmt.Errorf("unknown unary operator")
	}
}

func (ev *evaluator) evalBinary(b *ast.Binary, scope *Scope) (value.Value, error) {
	l, err := ev.evalExpr(b.L, scope)
	if err != nil {
		return nil, err
	}
	r, err := ev.evalExpr(b.R, scope)
	if err != nil {
		return nil, err
	}

	switch b.Op {
	case token.PLUS, token.MINUS, token.STAR, token.SLASH, token.PERCENT:
		ln, lok := l.(*value.Number)
		rn, rok := r.(*value.Number)
		if lok && rok {
			return value.Arith(opText(b.Op), ln, rn)
		}
		return nil, fmt.Errorf("cannot apply %q to %s and %s", opText(b.Op), l.TypeName(), r.TypeName())
	case token.EQ:
		return &value.Bool{Val: l.CSS(true) == r.CSS(true)}, nil
	case token.NEQ:
		return &value.Bool{Val: l.CSS(true) != r.CSS(true)}, nil
	case token.LT, token.GT, token.LE, token.GE:
		return ev.compareNumbers(b.Op, l, r)
	case token.AND:
		return &value.Bool{Val: value.Truthy(l) && value.Truthy(r)}, nil
	case token.OR:
		if value.Truthy(l) {
			return l, nil
		}
		return r, nil
	default:
		return nil, fmt.Errorf("unknown binary operator")
	}
}

func (ev *evaluator) compareNumbers(op token.Kind, l, r value.Value) (value.Value, error) {
	ln, lok := l.(*value.Number)
	rn, rok := r.(*value.Number)
	if !lok || !rok {
		return nil, fmt.Errorf("cannot compare %s and %s", l.TypeName(), r.TypeName())
	}
	var res bool
	switch op {
	case token.LT:
		res = ln.Num < rn.Num
	case token.GT:
		res = ln.Num > rn.Num
	case token.LE:
		res = ln.Num <= rn.Num
	case token.GE:
		res = ln.Num >= rn.Num
	}
	return &value.Bool{Val: res}, nil
}

func (ev *evaluator) evalCall(c *ast.Call, scope *Scope) (value.Value, error) {
	args := make([]value.Value, len(c.Args))
	for i, a := range c.Args {
		v, err := ev.evalExpr(a, scope)
		if err != nil {
			return nil, err
		}
		args[i] = v
	}

	// User-defined function takes precedence (it can shadow a built-in).
	if cl, ok := scope.GetFunc(c.Name); ok {
		return ev.invoke(cl, args, nil, nil)
	}

	if fn, ok := builtin.Lookup(c.Name); ok {
		return fn(args)
	}

	// Unknown function: pass through as a literal CSS function call,
	// e.g. translateX(10px) or url(...).
	sep := ","
	if ev.opts.Pretty {
		sep = ", "
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = a.CSS(ev.opts.Pretty)
	}
	return &value.Ident{Name: c.Name + "(" + strings.Join(parts, sep) + ")"}, nil
}

func opText(k token.Kind) string {
	switch k {
	case token.PLUS:
		return "+"
	case token.MINUS:
		return "-"
	case token.STAR:
		return "*"
	case token.SLASH:
		return "/"
	case token.PERCENT:
		return "%"
	}
	return "?"
}
