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
	// Globals defines variables in the root scope before the stylesheet
	// executes (see the public styl.Options.Globals for value conversion).
	Globals map[string]any
	// CustomProperties lists root-level variables to expose as CSS custom
	// properties (see the public styl.Options.CustomProperties).
	CustomProperties []string
	// Warn receives warn() messages; nil writes "Warning: msg" to stderr,
	// as the stylus CLI does.
	Warn func(msg string)
	// Source-map inputs (used by EvaluateMap/EvaluateFull).
	SourceMap     bool   // build a source map (EvaluateFull)
	SourceFile    string // .styl path recorded in the map's "sources"
	SourceContent string // original source text, embedded as "sourcesContent"
	OutFile       string // generated filename recorded in the map's "file"
	MapFile       string // where the map is written; "sources" are named relative to it (see sourcemap.go)
	// Sandbox, when non-nil, confines the compile for untrusted source: no
	// OS filesystem, vetted imports, and step/time/size budgets (see
	// sandbox.go). nil leaves every check off.
	Sandbox *Sandbox
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
	required     map[string]bool // paths already pulled in by @require (import-once set)
	depth        int             // current function/mixin call depth
	deps         []string        // resolved paths of every inlined @import, in order
	customProps  map[string]bool // variable names exposed as CSS custom properties
	rootScope    *Scope          // the stylesheet's root scope (custom props bind here)
	// cur is the context of the statement executing now. Expression
	// evaluation only carries a scope, so the built-ins that read or change
	// where they are called from — selector(), current-media(),
	// add-property(), define() … (see context.go) — find it here. execStmt
	// sets and restores it around each statement.
	cur *execCtx
	// sb tallies usage against opts.Sandbox (unused when that is nil).
	sb sandboxState
	// sources holds the text of every imported file, in first-import order,
	// for the source map's sourcesContent (collected only when
	// opts.SourceMap is set; see sourcemap.go).
	sources []importedSource
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

	// The fields below describe where the block sits, for the context
	// built-ins. A function or mixin body inherits them from its call site,
	// as in Stylus, where selector() inside a function reports the caller's
	// selector.
	//
	// stack holds each nesting level's own selectors, outermost first, as
	// written (after interpolation and class prefixing) rather than combined:
	//
	//	.a, .b          stack [[.a .b]]           parents [.a .b]
	//	  .c &:hover    stack [[.a .b] [.c &:hover]]
	//	                parents [.c .a:hover  .c .b:hover]
	//
	// selectors() reports it level by level; selector() uses parents.
	stack [][]string
	// media is the innermost enclosing @media header ("" outside any), for
	// current-media().
	media string
	// prefix is the class prefix set by +prefix-classes(p): a '.' starting a
	// class name in this block's selectors gains it.
	prefix string
	// propRule is the rule add-property() appends to when rule is nil: a
	// pure function call has no rule of its own, but add-property() inside
	// it targets the rule of the declaration that called it.
	propRule *css.Rule
	// block is the block the executing mixin was called with (`+m(args)`
	// plus an indented body), run by `{block}`; nil when there is none.
	// Every nested context of the mixin body (rulesets, @media, …) carries
	// it, so a `{block}` at any depth of the body finds it.
	block *passedBlock
}

// passedBlock is the indented body handed to a user block mixin, together
// with the call site's lexical context. `{block}` runs the body with the
// call site's scope, file and import directory (the statements were written
// there, so their variables and @imports resolve there), but emits into the
// slot's rule and selectors (where the mixin placed it):
//
//	m()                        +m()           .wrap .a { color: red }
//	  .wrap                      .a
//	    {block}      ──►           color c    (c from the call site's scope)
//
// outer is the block of the mixin the call site itself sits in, so a
// `{block}` inside the passed body refers to that enclosing mixin's block
// rather than to itself.
type passedBlock struct {
	body  []ast.Stmt
	scope *Scope
	file  string
	dir   string
	mixin string
	outer *passedBlock
}

// newPassedBlock captures a block mixin call's body and its call-site
// context ctx; nil when the call has no block.
func newPassedBlock(body []ast.Stmt, ctx *execCtx) *passedBlock {
	if body == nil {
		return nil
	}
	return &passedBlock{body: body, scope: ctx.scope, file: ctx.file, dir: ctx.dir, mixin: ctx.mixin, outer: ctx.block}
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
	ev, nodes, err := evalNodes(sheet, opts)
	if err != nil {
		return "", "", nil, err
	}
	deps = ev.deps
	if !opts.SourceMap {
		cssOut = css.RenderSheet(nodes, opts.Pretty, nil)
		if err := ev.checkOutput(cssOut); err != nil {
			return "", "", nil, err
		}
		return cssOut, "", deps, nil
	}
	sm := ev.newSourceMap()
	cssOut = css.RenderSheet(nodes, opts.Pretty, sm)
	if err := ev.checkOutput(cssOut); err != nil {
		return "", "", nil, err
	}
	return cssOut, sm.JSON(), deps, nil
}

// evalNodes runs the evaluator and returns it along with the resolved
// top-level output nodes (after @extend resolution and the optional
// duplicate-merge pass). The evaluator carries the import deps and root scope.
func evalNodes(sheet *ast.Stylesheet, opts Options) (*evaluator, []css.Node, error) {
	ev := &evaluator{
		opts:         opts,
		placeholders: map[string]*css.Rule{},
		importing:    map[string]bool{},
		required:     map[string]bool{},
		customProps:  map[string]bool{},
	}
	for _, name := range opts.CustomProperties {
		ev.customProps[name] = true
	}
	ev.rootScope = NewScope()
	ctx := &execCtx{scope: ev.rootScope, dir: opts.BaseDir, file: opts.Filename}
	ctx.sink = &ev.out

	if err := ev.seedGlobals(ctx.scope); err != nil {
		return nil, nil, err
	}
	if err := ev.execBlock(sheet.Statements, ctx); err != nil {
		return nil, nil, err
	}

	if err := ev.applyExtends(); err != nil {
		return nil, nil, err
	}

	if root := ev.customPropsRule(); root != nil {
		ev.out = append([]css.Node{root}, ev.out...)
	}

	nodes := ev.out
	if opts.MergeDuplicates {
		nodes = css.MergeDuplicates(nodes)
	}
	return ev, nodes, nil
}

// applyExtends grafts each @extend's selectors onto every matching target rule.
//
// In a sandbox the grafted selector text is charged against MaxOutputBytes
// as it accrues: N extends of a selector shared by M rules graft N*M
// selector lists, so a sheet that ran in few steps could otherwise build a
// huge tree here, before the final output check ever sees it.
func (ev *evaluator) applyExtends() error {
	limit := 0
	if sb := ev.opts.Sandbox; sb != nil {
		limit = sb.MaxOutputBytes
	}
	grafted := 0
	for _, ex := range ev.extends {
		exBytes := 0
		for _, e := range ex.extenders {
			exBytes += len(e) + 1
		}
		targets := ev.findExtendTargets(ex.target)
		for _, t := range targets {
			t.Extenders = append(t.Extenders, ex.extenders...)
			grafted += exBytes
			if limit > 0 && grafted > limit {
				return limitErr("@extend output exceeds %d bytes", limit)
			}
		}
	}
	return nil
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
	if err := ev.tick(); err != nil {
		line, col := ast.Pos(stmt)
		return diag.WrapPos(err, ctx.file, line, col)
	}
	prev := ev.cur
	ev.cur = ctx
	defer func() { ev.cur = prev }()
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
	case *ast.MemberAssign:
		return ev.evalMemberAssign(s, ctx.scope)
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
		// Stylus prints an object as a JSON-ish blob, which is never valid
		// CSS; an error pointing at the key syntax is more useful.
		if _, isObj := value.Deref(v).(*value.Hash); isObj {
			return fmt.Errorf("property %q: an object is not a CSS value (read a key with obj.key or obj[key])", prop)
		}
		ctx.rule.Statements = append(ctx.rule.Statements, &css.Statement{
			Property:  prop,
			Value:     v.CSS(ev.opts.Pretty),
			Important: s.Important,
			Pos:       css.Pos{Line: s.Line, Col: s.Col, File: ctx.file},
			ValuePos:  css.Pos{Line: s.ValueLine, Col: s.ValueCol, File: ctx.file},
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
	case *ast.BlockSlot:
		child, body, err := ev.blockSlotCtx(ctx)
		if err != nil {
			return err
		}
		return ev.execBlock(body, child)
	case *ast.Comment:
		// Only ParseWithComments (the migration's parse) produces these;
		// the evaluator reaches them in function bodies and ignores them.
		return nil
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
	scope.Set(a.Name, ev.wrapVar(a.Name, scope, v))
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
		if ctx.prefix != "" {
			r = prefixClasses(r, ctx.prefix)
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
		Pos:       css.Pos{Line: rs.Line, Col: rs.Col, File: ctx.file},
	}
	if allPlaceholders(combined) {
		rule.Placeholder = true
		for _, s := range combined {
			ev.placeholders[s] = rule
		}
	}
	*ctx.sink = append(*ctx.sink, rule)
	ev.rules = append(ev.rules, rule)

	child := &execCtx{scope: ctx.scope.Child(), rule: rule, parents: combined, sink: ctx.sink, dir: ctx.dir, file: ctx.file, mixin: ctx.mixin,
		stack: append(ctx.stack[:len(ctx.stack):len(ctx.stack)], selfs), media: ctx.media, prefix: ctx.prefix, block: ctx.block}
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
	if h, ok := value.Deref(iter).(*value.Hash); ok {
		return ev.evalForHash(s, h, ctx)
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
	if s.Block != nil {
		return ev.evalBlockMixinCall(s, cl, ctx)
	}
	if !ok {
		// A bare identifier naming a variable is an expression statement: its
		// value becomes the implicit return (a function body ending in `n`).
		if len(s.Args) == 0 {
			if v, isVar := ctx.scope.Get(s.Name); isVar {
				ctx.ret = v
				return nil
			}
		}
		// A built-in called as a statement runs as an expression, as in
		// Stylus: its value is the implicit return (a function body ending
		// in `round(n)`), and its error stops compilation (`error('…')`
		// guarding a mixin's arguments).
		if isBuiltinName(s.Name) {
			v, err := ev.evalCall(&ast.Call{Name: s.Name, Args: s.Args}, ctx.scope)
			if err != nil {
				return err
			}
			ctx.ret = v
			return nil
		}
		candidates := ctx.scope.FuncNames()
		for name := range builtin.Registry {
			candidates = append(candidates, name)
		}
		for name := range ctxBuiltins {
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
	_, err = ev.invoke(cl, args, ctx, nil)
	return err
}

// invoke runs a closure's body. emit carries the caller's emission context (rule,
// selectors, sink, dir) so a mixin's declarations and nested rulesets land in the
// caller's output; pass nil for a pure function (expression) call. The return
// value is the body's `return` value, or Null if none. blk is the block a
// block mixin call passed (see passedBlock), or nil.
func (ev *evaluator) invoke(cl *Closure, args []value.Value, emit *execCtx, blk *passedBlock) (value.Value, error) {
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
	fctx := &execCtx{scope: fscope, file: cl.File, mixin: cl.Def.Name, block: blk}
	// The body sees its call site's selector stack, media and class prefix
	// (see execCtx). ev.cur is the calling statement's context.
	if caller := ev.cur; caller != nil {
		fctx.parents = caller.parents
		fctx.stack = caller.stack
		fctx.media = caller.media
		fctx.prefix = caller.prefix
		fctx.propRule = caller.rule
		if fctx.propRule == nil {
			fctx.propRule = caller.propRule
		}
	}
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
	if l, ok := value.Deref(v).(*value.List); ok {
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
		val, err := ev.interpolateString(x.Value, scope)
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
		// url(...) arrives as one raw token (see the lexer's rawCallNames).
		if v, ok := ev.evalURL(x.Name, scope); ok {
			return v, nil
		}
		// Variable reference inlines its value; otherwise it's a bare keyword.
		if v, ok := scope.Get(x.Name); ok {
			return v, nil
		}
		return &value.Ident{Name: x.Name}, nil
	case *ast.Unary:
		return ev.evalUnary(x, scope)
	case *ast.Binary:
		// checkValue is a no-op outside a sandbox; inside, it bounds the
		// value-growing forms (binary ops, list literals, calls). Ranges
		// are exempt: maxRangeLen already bounds them, and a loop over
		// 1..65536 (~500 KB rendered) is legitimate — it's what the loop
		// body does per step that MaxSteps meters.
		if x.Op == token.DOTDOT || x.Op == token.ELLIPSIS {
			return ev.evalBinary(x, scope)
		}
		return ev.checkValue(ev.evalBinary(x, scope))
	case *ast.List:
		items := make([]value.Value, len(x.Items))
		for i, it := range x.Items {
			v, err := ev.evalExpr(it, scope)
			if err != nil {
				return nil, err
			}
			items[i] = v
		}
		return ev.checkValue(&value.List{Items: items, Comma: x.Comma}, nil)
	case *ast.Call:
		return ev.checkValue(ev.evalCall(x, scope))
	case *ast.Index:
		return ev.evalIndex(x, scope)
	case *ast.Member:
		return ev.evalMember(x, scope)
	case *ast.Object:
		return ev.evalObject(x, scope)
	default:
		return nil, fmt.Errorf("cannot evaluate expression %T", e)
	}
}

// evalURL evaluates variables inside a raw `url(...)` token, as Stylus does:
// with p = "a.png", url(p) is url("a.png") and url(base + p) joins the two.
//
// Stylus parses every url() argument as an expression and always prints the
// result quoted. go-styl keeps url() contents verbatim (so url(/a.png),
// url(data:…) and url(a/b) pass through untouched, with no quoting churn)
// and only evaluates when the contents parse as an expression that
// references a defined variable. The output then follows Stylus's compiler:
// strings contribute their unquoted text, list items are joined with no
// separator, and the whole is wrapped in double quotes.
//
// ok is false when the token is not a url() call, or when it stays verbatim:
// no variable is referenced, the contents don't parse, or evaluating them
// fails (the verbatim text is the safer output for odd URLs).
func (ev *evaluator) evalURL(name string, scope *Scope) (value.Value, bool) {
	if len(name) < 5 || !strings.EqualFold(name[:4], "url(") || !strings.HasSuffix(name, ")") {
		return nil, false
	}
	inner := strings.TrimSpace(name[4 : len(name)-1])
	e, err := parser.ParseExpr(inner, 0)
	if err != nil || !refsVariable(e, scope) {
		return nil, false
	}
	v, err := ev.evalExpr(e, scope)
	if err != nil {
		return nil, false
	}
	return &value.Ident{Name: `url("` + urlText(value.Deref(v)) + `")`}, true
}

// refsVariable reports whether e mentions an identifier that names a
// variable in scope.
func refsVariable(e ast.Expr, scope *Scope) bool {
	switch x := e.(type) {
	case *ast.Ident:
		_, ok := scope.Get(x.Name)
		return ok
	case *ast.Unary:
		return refsVariable(x.X, scope)
	case *ast.Binary:
		return refsVariable(x.L, scope) || refsVariable(x.R, scope)
	case *ast.List:
		for _, it := range x.Items {
			if refsVariable(it, scope) {
				return true
			}
		}
	case *ast.Call:
		for _, a := range x.Args {
			if refsVariable(a, scope) {
				return true
			}
		}
	case *ast.Index:
		return refsVariable(x.X, scope) || refsVariable(x.Index, scope)
	}
	return false
}

// urlText is a value's text inside url("…"): a string without its quotes,
// a list's items run together (Stylus prints url() arguments with no
// spaces), anything else its CSS form.
func urlText(v value.Value) string {
	switch t := v.(type) {
	case *value.Str:
		return t.Val
	case *value.List:
		var b strings.Builder
		for _, it := range t.Items {
			b.WriteString(urlText(value.Deref(it)))
		}
		return b.String()
	}
	return v.CSS(true)
}

// evalIndex evaluates a subscript the way Stylus does (visitMember/
// visitIndex): indexes are 0-based and a negative one counts from the end
// (r[-1] is the last item). A single value indexes as a one-item list, so
// 5px[0] is 5px. An index past either end gives null, which prints as
// nothing. A list or range index (r[0 1], r[0..1]) selects each item in turn
// and returns them as a space-separated list.
func (ev *evaluator) evalIndex(ix *ast.Index, scope *Scope) (value.Value, error) {
	v, err := ev.evalExpr(ix.X, scope)
	if err != nil {
		return nil, err
	}
	idx, err := ev.evalExpr(ix.Index, scope)
	if err != nil {
		return nil, err
	}
	v, idx = value.Deref(v), value.Deref(idx)

	// obj[key]: a missing key is null, like obj.key.
	if h, ok := v.(*value.Hash); ok {
		if got, found := h.Get(value.KeyString(idx)); found {
			return got, nil
		}
		return value.Null{}, nil
	}

	var items []value.Value
	switch t := v.(type) {
	case *value.List:
		items = t.Items
	case value.Null, *value.Null:
		items = nil
	default:
		items = []value.Value{v}
	}

	pick := func(n value.Value) (value.Value, error) {
		num, ok := value.Deref(n).(*value.Number)
		if !ok {
			return nil, fmt.Errorf("list index must be a number, got %s", n.TypeName())
		}
		i := int(num.Num)
		if i < 0 {
			i += len(items)
		}
		if i < 0 || i >= len(items) {
			return value.Null{}, nil
		}
		return items[i], nil
	}

	if list, ok := idx.(*value.List); ok {
		out := make([]value.Value, 0, len(list.Items))
		for _, n := range list.Items {
			it, err := pick(n)
			if err != nil {
				return nil, err
			}
			out = append(out, it)
		}
		if len(out) == 1 {
			return out[0], nil
		}
		return &value.List{Items: out}, nil
	}
	return pick(idx)
}

func (ev *evaluator) evalUnary(u *ast.Unary, scope *Scope) (value.Value, error) {
	v, err := ev.evalExpr(u.X, scope)
	if err != nil {
		return nil, err
	}
	v = value.Deref(v)
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
	if b.InText {
		if v, ok, err := ev.inAsText(b, scope); ok || err != nil {
			return v, err
		}
	}
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
		// division does not (font: 14px/1.5). Custom-property refs stay
		// wrapped — font: 14px/var(--lh) is valid CSS.
		return &value.SlashList{L: l, R: r}, nil
	}
	// Operations below compute from concrete values, so custom-property
	// wrappers resolve to their compile-time value here.
	l, r = value.Deref(l), value.Deref(r)

	// Stylus's sprintf operator: `"calc(100vh - %s)" % x` is s(fmt, x), and a
	// list on the right spreads into one argument per item, so
	// `"%s and %s" % (1px 2px)` fills both placeholders. `%` binds like `*`,
	// so `"%s" % 1px 3px` formats only 1px, as in Stylus.
	if b.Op == token.PERCENT {
		if _, isStr := l.(*value.Str); isStr {
			args := []value.Value{l}
			if list, isList := r.(*value.List); isList {
				args = append(args, list.Items...)
			} else {
				args = append(args, r)
			}
			sprintf, _ := builtin.Lookup("s")
			return sprintf(args)
		}
	}

	// String concatenation: a string on the left of `+` joins the right
	// side's text into a new string, as in Stylus's String#operate:
	// "x" + "y" and "x" + y (an ident) both give 'xy'. Stylus quotes the
	// result with ' whatever the operands' quotes. An unquoted left side
	// (unquote(), s(), `%`) is a Literal in Stylus and stays unquoted:
	// s("%s", 1) + "px" gives 1px.
	if b.Op == token.PLUS {
		if ls, isStr := l.(*value.Str); isStr {
			quote := '\''
			if ls.Quote == 0 {
				quote = 0
			}
			return &value.Str{Val: ls.Val + concatText(r), Quote: quote}, nil
		}
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
	case token.IN:
		return &value.Bool{Val: contains(r, l)}, nil
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

	// Built-ins that need the evaluator (the call site's selector, scope or
	// rule; files; options) rather than just their argument values.
	if fn, ok := ctxBuiltins[c.Name]; ok {
		for i, a := range args {
			args[i] = value.Deref(a)
		}
		return fn(ev, c, args, scope)
	}

	if fn, ok := builtin.Lookup(c.Name); ok {
		// Built-ins type-switch on concrete values, so custom-property
		// wrappers resolve to their compile-time value. (User-defined
		// functions above receive the wrapper: uses inside their bodies
		// deref lazily, so emitted declarations keep var(--name).)
		for i, a := range args {
			args[i] = value.Deref(a)
		}
		v, err := fn(args)
		if err != nil {
			return nil, err
		}
		// push/pop/shift/unshift also update the list variable they were
		// given, as in Stylus (see mutateListVar).
		ev.mutateListVar(c, args, v, scope)
		return v, nil
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

// concatText is the text a value contributes to a string concatenation,
// following Stylus's String#coerce: a string gives its raw value (no quotes),
// a list joins its items' text with spaces (comma lists too), null is the
// word "null", and anything else is its CSS form.
func concatText(v value.Value) string {
	switch t := v.(type) {
	case *value.Str:
		return t.Val
	case *value.List:
		parts := make([]string, len(t.Items))
		for i, it := range t.Items {
			parts[i] = concatText(it)
		}
		return strings.Join(parts, " ")
	case value.Null, *value.Null:
		return "null"
	}
	return v.CSS(true)
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
