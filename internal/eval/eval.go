// Package eval walks the AST, resolving variables/expressions to values and
// flattening nested rulesets into a CSS rule tree.
package eval

import (
	"fmt"
	"io/fs"
	"math"
	"slices"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/builtin"
	"github.com/rohanthewiz/go-styl/internal/css"
	"github.com/rohanthewiz/go-styl/internal/diag"
	"github.com/rohanthewiz/go-styl/internal/parser"
	"github.com/rohanthewiz/go-styl/internal/token"
	"github.com/rohanthewiz/go-styl/internal/value"
)

// Options controls evaluation/rendering.
type Options struct {
	Pretty          bool
	MergeDuplicates bool
	Filename        string   // source path, used to position error messages
	BaseDir         string   // directory @import paths resolve against
	IncludePaths    []string // extra directories searched for @import
	FS              fs.FS    // when set, @import resolves through it instead of the OS
	// Source-map inputs (used by EvaluateMap/EvaluateFull).
	SourceMap     bool   // build a source map (EvaluateFull)
	SourceFile    string // .styl path recorded in the map's "sources"
	SourceContent string // original source text, embedded as "sourcesContent"
	OutFile       string // generated filename recorded in the map's "file"
}

// extendReq records a pending @extend: graft Extenders onto every rule matching
// Target (a selector or "$placeholder" name).
type extendReq struct {
	extenders []string
	target    string
}

// maxCallDepth bounds function/mixin call nesting so runaway recursion
// (a mixin calling itself without a base case) errors instead of exhausting
// the process stack.
const maxCallDepth = 256

// maxSelectors bounds a single rule's combined selector list. Nested
// comma-separated selector groups multiply, and recursive mixins can drive
// that growth exponentially.
const maxSelectors = 16384

type evaluator struct {
	opts         Options
	out          []css.Node           // top-level output nodes in encounter order
	rules        []*css.Rule          // every rule created (any nesting), for @extend lookup
	placeholders map[string]*css.Rule // $name -> placeholder rule
	extends      []extendReq
	importing    map[string]bool // absolute paths currently being imported (cycle guard)
	depth        int             // current function/mixin call depth
	deps         []string        // resolved paths of every inlined @import, in order
}

// execCtx captures where statements emit while a block executes: the active
// variable/function scope, the rule that declarations append to (nil when not
// inside a selector), and the selector context for nested rulesets. ret/returned
// carry a function body's return value.
type execCtx struct {
	scope    *Scope
	rule     *css.Rule
	parents  []string
	sink     *[]css.Node // where rulesets/at-rules in this block are emitted
	dir      string      // directory @import paths in this block resolve against
	file     string      // source file this block's statements came from
	ret      value.Value
	returned bool
	// mixin is the name of the mixin whose body is executing, if any. A
	// declaration matching it stays a plain property instead of a transparent
	// call, so the canonical `border-radius(n)` / `border-radius n` mixin
	// pattern does not recurse (stylus behaves the same).
	mixin string
}

// Evaluate evaluates a stylesheet and returns the rendered CSS.
func Evaluate(sheet *ast.Stylesheet, opts Options) (string, error) {
	opts.SourceMap = false
	cssOut, _, _, err := EvaluateFull(sheet, opts)
	return cssOut, err
}

// EvaluateMap evaluates a stylesheet and returns the rendered CSS together with a
// Source Map v3 document (JSON) mapping output positions back to the source.
func EvaluateMap(sheet *ast.Stylesheet, opts Options) (cssOut, mapJSON string, err error) {
	opts.SourceMap = true
	cssOut, mapJSON, _, err = EvaluateFull(sheet, opts)
	return cssOut, mapJSON, err
}

// EvaluateFull evaluates a stylesheet, returning the rendered CSS, a source map
// (when opts.SourceMap is set, else ""), and the resolved paths of every
// inlined @import (for build-cache invalidation).
func EvaluateFull(sheet *ast.Stylesheet, opts Options) (cssOut, mapJSON string, deps []string, err error) {
	nodes, deps, err := evalNodes(sheet, opts)
	if err != nil {
		return "", "", nil, err
	}
	if !opts.SourceMap {
		return css.RenderSheet(nodes, opts.Pretty, nil), "", deps, nil
	}
	sm := css.NewSourceMap(opts.OutFile, opts.SourceFile, opts.SourceContent)
	cssOut = css.RenderSheet(nodes, opts.Pretty, sm)
	return cssOut, sm.JSON(), deps, nil
}

// evalNodes runs the evaluator and returns the resolved top-level output nodes
// (after @extend resolution and the optional duplicate-merge pass) plus the
// resolved import paths.
func evalNodes(sheet *ast.Stylesheet, opts Options) ([]css.Node, []string, error) {
	ev := &evaluator{
		opts:         opts,
		placeholders: map[string]*css.Rule{},
		importing:    map[string]bool{},
	}
	ctx := &execCtx{scope: NewScope(), dir: opts.BaseDir, file: opts.Filename}
	ctx.sink = &ev.out

	if err := ev.execBlock(sheet.Statements, ctx); err != nil {
		return nil, nil, err
	}

	ev.applyExtends()

	nodes := ev.out
	if opts.MergeDuplicates {
		nodes = css.MergeDuplicates(nodes)
	}
	return nodes, ev.deps, nil
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

// execStmt executes one statement, anchoring any resulting error at the
// statement's source position (an error already positioned deeper — e.g. inside
// a mixin body — keeps its inner position).
func (ev *evaluator) execStmt(stmt ast.Stmt, ctx *execCtx) error {
	if err := ev.execStmtInner(stmt, ctx); err != nil {
		line, col := ast.Pos(stmt)
		return diag.WrapPos(err, ctx.file, line, col)
	}
	return nil
}

func (ev *evaluator) execStmtInner(stmt ast.Stmt, ctx *execCtx) error {
	switch s := stmt.(type) {
	case *ast.Assignment:
		return ev.evalAssignment(s, ctx.scope)
	case *ast.Declaration:
		if ctx.rule == nil {
			return fmt.Errorf("property %q must appear inside a selector", s.Property)
		}
		// Transparent mixin call: a declaration whose property names a mixin
		// in scope invokes it (`border-radius 3px`), list values becoming the
		// arguments — except the executing mixin's own name, which stays a
		// plain property.
		if s.Property != ctx.mixin {
			if _, ok := ctx.scope.GetFunc(s.Property); ok {
				call := &ast.MixinCall{Name: s.Property, Args: transparentArgs(s.Value), Line: s.Line, Col: s.Col}
				return ev.evalMixinCall(call, ctx)
			}
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
			Property:  prop,
			Value:     v.CSS(ev.opts.Pretty),
			Important: s.Important,
			Pos:       css.Pos{Line: s.Line, Col: s.Col},
		})
		return nil
	case *ast.RuleSet:
		return ev.evalRuleSet(s, ctx)
	case *ast.FuncDef:
		ctx.scope.SetFunc(s.Name, &Closure{Def: s, Scope: ctx.scope, File: ctx.file})
		return nil
	case *ast.MixinCall:
		return ev.evalMixinCall(s, ctx)
	case *ast.If:
		return ev.evalIf(s, ctx)
	case *ast.For:
		return ev.evalFor(s, ctx)
	case *ast.ExprStmt:
		// A bare expression: its value becomes the enclosing function's
		// implicit return (the last one evaluated wins); elsewhere it is
		// evaluated and dropped.
		v, err := ev.evalExpr(s.X, ctx.scope)
		if err != nil {
			return err
		}
		ctx.ret = v
		return nil
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
	case *ast.AtRule:
		return ev.evalAtRule(s, ctx)
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
	// Nested comma groups multiply (parents × selfs); recursion can make that
	// exponential, so bail out before memory does.
	if len(combined) > maxSelectors {
		return fmt.Errorf("combined selector count exceeds %d — runaway selector nesting?", maxSelectors)
	}

	rule := &css.Rule{
		Selector:  joinSelectors(combined, ev.opts.Pretty),
		Selectors: combined,
		Pos:       css.Pos{Line: rs.Line, Col: rs.Col},
	}
	if allPlaceholders(combined) {
		rule.Placeholder = true
		for _, s := range combined {
			ev.placeholders[s] = rule
		}
	}
	*ctx.sink = append(*ctx.sink, rule)
	ev.rules = append(ev.rules, rule)

	child := &execCtx{scope: ctx.scope.Child(), rule: rule, parents: combined, sink: ctx.sink, dir: ctx.dir, file: ctx.file, mixin: ctx.mixin}
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

// transparentArgs converts a declaration value into mixin-call arguments:
// list items (space- or comma-separated) become separate arguments, as in
// reference Stylus (`m 1px 2px` and `m 1px, 2px` both call m(1px, 2px)).
func transparentArgs(v ast.Expr) []ast.Expr {
	if l, ok := v.(*ast.List); ok {
		return l.Items
	}
	return []ast.Expr{v}
}

// evalMixinCall invokes a function/mixin in statement position, emitting its body
// into the current rule and selector context.
func (ev *evaluator) evalMixinCall(s *ast.MixinCall, ctx *execCtx) error {
	cl, ok := ctx.scope.GetFunc(s.Name)
	if !ok {
		// A bare identifier naming a variable is an expression statement: its
		// value becomes the implicit return (a function body ending in `n`).
		if len(s.Args) == 0 {
			if v, isVar := ctx.scope.Get(s.Name); isVar {
				ctx.ret = v
				return nil
			}
		}
		candidates := ctx.scope.FuncNames()
		for name := range builtin.Registry {
			candidates = append(candidates, name)
		}
		if hint := suggest(s.Name, candidates); hint != "" {
			return fmt.Errorf("undefined mixin %q (did you mean %q?)", s.Name, hint)
		}
		return fmt.Errorf("undefined mixin %q", s.Name)
	}
	args, err := ev.evalArgs(s.Args, ctx.scope)
	if err != nil {
		return err
	}
	_, err = ev.invoke(cl, args, ctx)
	return err
}

// invoke runs a closure's body. emit carries the caller's emission context (rule,
// selectors, sink, dir) so a mixin's declarations and nested rulesets land in the
// caller's output; pass nil for a pure function (expression) call. The return
// value is the body's `return` value, or Null if none.
func (ev *evaluator) invoke(cl *Closure, args []value.Value, emit *execCtx) (value.Value, error) {
	if ev.depth >= maxCallDepth {
		return nil, fmt.Errorf("call depth exceeds %d in %q — unbounded recursion?", maxCallDepth, cl.Def.Name)
	}
	ev.depth++
	defer func() { ev.depth-- }()

	fscope := cl.Scope.Child()
	if err := ev.bindParams(fscope, cl.Def.Params, args); err != nil {
		return nil, err
	}
	// Body statements' positions refer to the definition site, so error
	// positioning uses the closure's file rather than the caller's.
	fctx := &execCtx{scope: fscope, file: cl.File, mixin: cl.Def.Name}
	if emit != nil {
		fctx.rule = emit.rule
		fctx.parents = emit.parents
		fctx.sink = emit.sink
		fctx.dir = emit.dir
	} else {
		// Pure function call: rulesets are unexpected, but route any to a scratch
		// sink so emission never dereferences a nil pointer.
		var scratch []css.Node
		fctx.sink = &scratch
	}
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

	if b.Literal {
		// Unparenthesized `/` in a property value: operands evaluate, the
		// division does not (font: 14px/1.5).
		return &value.SlashList{L: l, R: r}, nil
	}

	switch b.Op {
	case token.PLUS, token.MINUS, token.STAR, token.POW, token.SLASH, token.PERCENT:
		ln, lok := l.(*value.Number)
		rn, rok := r.(*value.Number)
		if lok && rok {
			return value.Arith(opText(b.Op), ln, rn)
		}
		if lc, isColor := l.(*value.Color); isColor {
			return value.ColorArith(opText(b.Op), lc, r)
		}
		return nil, fmt.Errorf("cannot apply %q to %s and %s", opText(b.Op), l.TypeName(), r.TypeName())
	case token.DOTDOT, token.ELLIPSIS:
		return evalRange(b.Op, l, r)
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
		return ev.invoke(cl, args, nil)
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
	case token.POW:
		return "**"
	case token.SLASH:
		return "/"
	case token.PERCENT:
		return "%"
	}
	return "?"
}

// maxRangeLen bounds materialized ranges so a fuzzer's 1..1e9 errors out
// instead of exhausting memory.
const maxRangeLen = 65536

// evalRange materializes `a..b` (inclusive) or `a...b` (excludes b) as a list
// stepping by 1, descending when a > b. The left operand's unit wins.
func evalRange(op token.Kind, l, r value.Value) (value.Value, error) {
	ln, lok := l.(*value.Number)
	rn, rok := r.(*value.Number)
	if !lok || !rok {
		return nil, fmt.Errorf("range bounds must be numbers, got %s and %s", l.TypeName(), r.TypeName())
	}
	if math.Abs(rn.Num-ln.Num) > maxRangeLen {
		return nil, fmt.Errorf("range %v..%v exceeds %d elements", ln.Num, rn.Num, maxRangeLen)
	}
	unit := ln.Unit
	if unit == "" {
		unit = rn.Unit
	}
	step := 1.0
	if rn.Num < ln.Num {
		step = -1
	}
	var items []value.Value
	for n := ln.Num; (step > 0 && n <= rn.Num) || (step < 0 && n >= rn.Num); n += step {
		if op == token.ELLIPSIS && n == rn.Num {
			break
		}
		items = append(items, &value.Number{Num: n, Unit: unit})
	}
	return &value.List{Items: items}, nil
}
