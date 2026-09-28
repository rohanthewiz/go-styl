package eval

// Stylus → modern CSS migration (`styl migrate`).
//
// Compile flattens nesting and inlines every variable. Migrate keeps the
// author's structure instead, so the result can be edited as source:
//
//   - nesting stays nesting (native CSS nesting, `&` included)
//   - root-level variables become custom properties on :root, and a direct
//     reference compiles to var(--name)
//   - Stylus-only constructs (mixins, functions, loops, conditionals,
//     @extend, @import of .styl files) are resolved at migrate time, and
//     every place that happens is flagged with a `/* styl-migrate: … */`
//     comment for manual review
//
// It reuses the evaluator for everything semantic — scopes, expressions,
// built-ins, import resolution — and replaces only the statement walk.
// The evaluator's walk writes flat css.Rules with fully combined selectors;
// the walk here builds a tree of mnodes that mirrors the source nesting:
//
//	evaluator:  .card .title:hover { … }        migrate:  .card {
//	                                                        .title {
//	                                                          &:hover { … }
//
// Each migrate step keeps a real execCtx alongside the output node, with
// the same scope/parents/stack/media fields the evaluator would have. That
// is what lets expression evaluation, the context built-ins (selector(),
// current-media(), add-property()) and mixin argument binding work
// unchanged.
//
// Where CSS nesting cannot say what Stylus says, the rule is "hoisted": it
// is written with its absolute selector after the enclosing top-level
// block, wrapped in the same at-rules. The cases are `&` concatenation
// (BEM `&__el`, `&-mod`: CSS `&` is a whole selector, never a name prefix)
// and nesting under a pseudo-element (`::before` cannot appear in the
// :is() that CSS `&` stands for).

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/css"
	"github.com/rohanthewiz/go-styl/internal/diag"
	"github.com/rohanthewiz/go-styl/internal/parser"
	"github.com/rohanthewiz/go-styl/internal/token"
	"github.com/rohanthewiz/go-styl/internal/value"
)

// MigrateOptions configures Migrate. The embedded Options supply import
// resolution (Filename, BaseDir, IncludePaths, FS), Globals and Warn; the
// output is always pretty.
type MigrateOptions struct {
	Options
	// Notes writes each MigrateNote into the CSS as a
	// `/* styl-migrate: … */` comment above the code it concerns.
	Notes bool
	// NoVars inlines every variable, as Compile does, instead of turning
	// root-level variables into custom properties.
	NoVars bool
}

// MigrateNote marks a place where the migration made a judgment call that
// a person should review.
type MigrateNote struct {
	File      string
	Line, Col int
	// Kind is a short category: mixin, loop, condition, frozen, hoisted,
	// extend, import, variable, prefix.
	Kind string
	Msg  string
}

// MigrateResult is the migrated CSS and what the migration had to decide.
type MigrateResult struct {
	CSS   string
	Notes []MigrateNote
	// Deps lists every inlined .styl import, as Result.Deps does.
	Deps []string
}

// mKind discriminates mnode.
type mKind int

const (
	mRule mKind = iota // a style rule: sels + kids
	mAt                // a block at-rule: head + kids (also the root container)
	mDecl              // a declaration: prop, val, important
	mRaw               // a verbatim line (leaf at-rules, literal @import)
)

// mnode is one node of the migrated output tree.
type mnode struct {
	kind      mKind
	sels      []string // mRule: selectors as written in the output
	head      string   // mAt: header ("@media …"); mRaw: the text
	prop, val string   // mDecl
	important bool     // mDecl
	lead      []string // comments rendered on the lines before the node
	kids      []*mnode

	// @extend bookkeeping (mRule only). abs is the rule's fully combined
	// selector list, what an @extend target is matched against. rootLevel
	// means sels are absolute too (no enclosing style rule), so extenders
	// can be added to the list. ext holds the extenders added.
	abs         []string
	rootLevel   bool
	placeholder bool
	ext         []string
}

// hoist is a node lifted out of nesting: it goes to the root after the
// current top-level statement, wrapped in the at-rules of chain.
type hoist struct {
	chain []string
	node  *mnode
}

// mExtend is a pending @extend, resolved once the whole tree exists.
type mExtend struct {
	target    string
	extenders []string
	file      string
	line, col int
}

// mctx is where a migrate step writes: the evaluator context (scope,
// selector parents, …), the output node that receives children, the
// headers of the enclosing at-rules (for hoisting), and whether a style
// rule encloses the node in the output (so selectors are relative).
type mctx struct {
	ec     *execCtx
	node   *mnode
	chain  []string
	inRule bool
}

type migrator struct {
	ev      *evaluator
	opts    MigrateOptions
	root    *mnode
	pending []hoist
	notes   []MigrateNote
	extends []mExtend
	rules   []*mnode // every mRule, for @extend lookup

	// Root variables exposed as custom properties, in definition order.
	// varOf is keyed by the Stylus name; only a name's first root binding
	// becomes a custom property (see assign).
	vars      []*value.Var
	varOf     map[string]*value.Var
	cssNames  map[string]bool
	reassigns map[string]bool // names already noted as reassigned
	// defs holds the live :root text for a variable derived from others
	// (calc(var(--a) + var(--b))); others render their value.
	defs map[*value.Var]string
	// frozenDefs holds the note for a variable whose value had to be
	// computed from other properties (dark = darken(brand, 10%)).
	frozenDefs map[*value.Var]MigrateNote
	seen       map[MigrateNote]bool
}

// Migrate converts a parsed stylesheet to modern, nested CSS.
func Migrate(sheet *ast.Stylesheet, opts MigrateOptions) (MigrateResult, error) {
	ev := &evaluator{
		opts:         opts.Options,
		placeholders: map[string]*css.Rule{},
		importing:    map[string]bool{},
		required:     map[string]bool{},
		customProps:  map[string]bool{},
	}
	ev.rootScope = NewScope()
	m := &migrator{
		ev:         ev,
		opts:       opts,
		root:       &mnode{kind: mAt},
		varOf:      map[string]*value.Var{},
		cssNames:   map[string]bool{},
		reassigns:  map[string]bool{},
		seen:       map[MigrateNote]bool{},
		defs:       map[*value.Var]string{},
		frozenDefs: map[*value.Var]MigrateNote{},
	}
	// Globals are seeded as plain values: they are migrate-time inputs,
	// not part of the sheet, so they inline.
	if err := ev.seedGlobals(ev.rootScope); err != nil {
		return MigrateResult{}, err
	}
	ec := &execCtx{scope: ev.rootScope, dir: opts.BaseDir, file: opts.Filename}
	var sink []css.Node
	ec.sink = &sink
	if err := m.walk(sheet.Statements, &mctx{ec: ec, node: m.root}); err != nil {
		return MigrateResult{}, err
	}
	m.flush()
	m.applyExtends()
	// render can add notes (frozen :root values), so it runs before
	// m.notes is read.
	cssOut := m.render()
	if err := ev.checkOutput(cssOut); err != nil {
		return MigrateResult{}, err
	}
	return MigrateResult{CSS: cssOut, Notes: m.notes, Deps: ev.deps}, nil
}

// walk runs statements in mc, stopping at a return. At the top level,
// hoisted nodes are flushed after each statement so they land right
// after the block they came from.
func (m *migrator) walk(stmts []ast.Stmt, mc *mctx) error {
	for _, stmt := range stmts {
		if mc.ec.returned {
			return nil
		}
		if err := m.exec(stmt, mc); err != nil {
			return err
		}
		if mc.node == m.root {
			m.flush()
		}
	}
	return nil
}

// exec runs one statement with ev.cur pointing at its context (the context
// built-ins read it) and anchors errors at the statement, as execStmt does.
func (m *migrator) exec(stmt ast.Stmt, mc *mctx) error {
	if err := m.ev.tick(); err != nil {
		line, col := ast.Pos(stmt)
		return diag.WrapPos(err, mc.ec.file, line, col)
	}
	prev := m.ev.cur
	m.ev.cur = mc.ec
	defer func() { m.ev.cur = prev }()
	if err := m.execInner(stmt, mc); err != nil {
		line, col := ast.Pos(stmt)
		return diag.WrapPos(err, mc.ec.file, line, col)
	}
	return nil
}

func (m *migrator) execInner(stmt ast.Stmt, mc *mctx) error {
	switch s := stmt.(type) {
	case *ast.Assignment:
		return m.assign(s, mc.ec)
	case *ast.Declaration:
		return m.decl(s, mc)
	case *ast.RuleSet:
		return m.ruleSet(s, mc)
	case *ast.MixinCall:
		return m.mixinCall(s, mc)
	case *ast.If:
		return m.ifStmt(s, mc)
	case *ast.For:
		return m.forStmt(s, mc)
	case *ast.Import:
		return m.importStmt(s, mc)
	case *ast.AtRule:
		return m.atRule(s, mc)
	case *ast.Extend:
		return m.extend(s, mc)
	case *ast.BlockSlot:
		// The passed block is walked in place, so its rules nest in the
		// slot's output node like the mixin body around it.
		child, body, err := m.ev.blockSlotCtx(mc.ec)
		if err != nil {
			return err
		}
		return m.walk(body, &mctx{ec: child, node: mc.node, chain: mc.chain, inRule: mc.inRule})
	default:
		// FuncDef, MemberAssign, ExprStmt, Return: nothing structural, so
		// the evaluator runs them as is.
		if err := m.ev.execStmtInner(stmt, mc.ec); err != nil {
			return err
		}
		m.drain(mc)
		return nil
	}
}

// note records a MigrateNote at a statement's position. The same note at
// the same place is recorded once: a declaration inside a loop body or a
// mixin runs many times, and one flag per source line is enough. The
// returned bool is false for a repeat, so callers skip its inline comment.
func (m *migrator) note(ec *execCtx, line, col int, kind, msg string) (MigrateNote, bool) {
	n := MigrateNote{File: ec.file, Line: line, Col: col, Kind: kind, Msg: msg}
	if m.seen[n] {
		return n, false
	}
	m.seen[n] = true
	m.notes = append(m.notes, n)
	return n, true
}

// annotate records a note for a construct that emitted container.kids[mark:]
// and, with Notes on, puts its comment above the first node emitted. A
// construct that emitted nothing (a mixin that only sets variables) is
// not worth reviewing and gets no note.
func (m *migrator) annotate(ec *execCtx, container *mnode, mark, line, col int, kind, msg string) {
	if len(container.kids) <= mark {
		return
	}
	n, first := m.note(ec, line, col, kind, msg)
	m.lead(container.kids[mark], n, first)
}

// lead adds a note's comment above node, when inline notes are on and the
// note is not a repeat (see note).
func (m *migrator) lead(node *mnode, n MigrateNote, first bool) {
	if m.opts.Notes && first {
		node.lead = append(node.lead, n.Kind+": "+n.Msg)
	}
}

// --- variables ---

// assign binds a variable like evalAssignment, and turns a root-level
// binding into a custom property.
//
// Only the first root binding of a name becomes --name. A later root
// reassignment binds a plain value, so references after it inline the new
// value while references before it keep reading --name, whose :root value
// is the first binding. That keeps both halves correct without a second
// pass; the reassignment is noted because the sheet's intent (one token,
// two values) no longer shows.
func (m *migrator) assign(a *ast.Assignment, ec *execCtx) error {
	if a.Op == token.ASSIGNQ && ec.scope.Has(a.Name) {
		return nil
	}
	v, err := m.ev.evalExpr(a.Value, ec.scope)
	if err != nil {
		return err
	}
	if ec.scope == m.ev.rootScope && !m.opts.NoVars {
		if prev, done := m.varOf[a.Name]; done {
			if !m.reassigns[a.Name] {
				m.reassigns[a.Name] = true
				m.note(ec, a.Line, a.Col, "variable",
					fmt.Sprintf("%s is reassigned; later uses inline the new value, earlier ones read var(--%s)", a.Name, prev.Name))
			}
		} else if cssable(v) {
			w := &value.Var{Name: m.cssName(a.Name), Inner: v}
			// A variable derived from others keeps the relation where CSS
			// can: mid = top + bottom gives
			// --mid: calc(var(--top) + var(--bottom)). Only side-effect-free
			// expressions qualify, since this evaluates them a second time
			// (push(list, x) would push twice).
			if refs := varRefs(a.Value, ec.scope); len(refs) > 0 {
				frozen := missingVars(v.CSS(true), refs)
				if isPureExpr(a.Value) {
					def, fz, err := m.liveValue(a.Value, ec.scope, "--"+w.Name)
					if err != nil {
						return err
					}
					if len(fz) == 0 {
						m.defs[w], frozen = def, nil
					}
				}
				if len(frozen) > 0 {
					// Noted by rootRule, only if --name makes it to :root.
					m.frozenDefs[w] = MigrateNote{File: ec.file, Line: a.Line, Col: a.Col, Kind: "frozen",
						Msg: fmt.Sprintf("--%s computed at migrate time from %s; it won't follow runtime changes",
							w.Name, strings.Join(frozen, ", "))}
				}
			}
			m.varOf[a.Name] = w
			m.vars = append(m.vars, w)
			v = w
		}
	}
	ec.scope.Set(a.Name, v)
	return nil
}

// isPureExpr reports whether evaluating e has no side effects: literals,
// variables and operators only, no calls.
func isPureExpr(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.NumberLit, *ast.ColorLit, *ast.StringLit:
		return true
	case *ast.Ident:
		// url(…) and {interpolation} arrive as idents but evaluate more.
		return !strings.ContainsAny(x.Name, "({")
	case *ast.Unary:
		return isPureExpr(x.X)
	case *ast.Binary:
		return isPureExpr(x.L) && isPureExpr(x.R)
	case *ast.List:
		for _, it := range x.Items {
			if !isPureExpr(it) {
				return false
			}
		}
		return true
	}
	return false
}

// cssable reports whether a value can be a custom property's value. Logic
// values (booleans, null, objects) stay compile-time only.
func cssable(v value.Value) bool {
	switch t := value.Deref(v).(type) {
	case *value.Number, *value.Color, *value.Str, *value.Ident:
		return true
	case *value.SlashList:
		return cssable(t.L) && cssable(t.R)
	case *value.List:
		if len(t.Items) == 0 {
			return false
		}
		for _, it := range t.Items {
			if !cssable(it) {
				return false
			}
		}
		return true
	}
	return false
}

// cssName maps a Stylus variable name to a unique custom property name:
// a leading '$' is dropped ($primary → --primary) and any character that
// is not valid in a CSS identifier becomes '-'.
func (m *migrator) cssName(name string) string {
	base := strings.TrimLeft(name, "$")
	if base == "" {
		base = "var"
	}
	var b strings.Builder
	for _, r := range base {
		if isClassRune(r) || r > 0x7f {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := b.String()
	for i := 2; m.cssNames[out]; i++ {
		out = fmt.Sprintf("%s-%d", b.String(), i)
	}
	m.cssNames[out] = true
	return out
}

// varRefs returns the custom property names that e references directly
// through a variable (root variables, and locals or parameters holding
// one), in first-seen order.
func varRefs(e ast.Expr, scope *Scope) []string {
	var out []string
	var visit func(ast.Expr)
	visit = func(e ast.Expr) {
		switch x := e.(type) {
		case *ast.Ident:
			if v, ok := scope.Get(x.Name); ok {
				if w, isVar := v.(*value.Var); isVar && !slices.Contains(out, w.Name) {
					out = append(out, w.Name)
				}
			}
		case *ast.Unary:
			visit(x.X)
		case *ast.Binary:
			visit(x.L)
			visit(x.R)
		case *ast.List:
			for _, it := range x.Items {
				visit(it)
			}
		case *ast.Call:
			for _, a := range x.Args {
				visit(a)
			}
		case *ast.Index:
			// A lookup into a list or object variable (sizes[0]) reads a
			// value that never becomes a property of its own, so only
			// the subscript counts.
			visit(x.Index)
		}
	}
	visit(e)
	return out
}

// --- declarations and rules ---

// decl mirrors the evaluator's Declaration case, writing an mDecl.
//
// A value that reads no custom-property variable renders exactly as
// Compile would. One that does goes through liveValue, which keeps
// arithmetic on variables live as calc() and reports the parts that had
// to be computed at migrate time (darken(primary, 10%)): those are correct
// today but won't follow the property at runtime, so they are flagged.
func (m *migrator) decl(s *ast.Declaration, mc *mctx) error {
	ec := mc.ec
	if ec.rule == nil {
		return fmt.Errorf("property %q must appear inside a selector", s.Property)
	}
	if s.Property != ec.mixin {
		if _, ok := ec.scope.GetFunc(s.Property); ok {
			call := &ast.MixinCall{Name: s.Property, Args: transparentArgs(s.Value), Line: s.Line, Col: s.Col}
			return m.mixinCall(call, mc)
		}
	}
	prop, err := m.ev.interpolate(s.Property, ec.scope)
	if err != nil {
		return err
	}
	var val string
	var frozen []string
	if len(varRefs(s.Value, ec.scope)) == 0 {
		v, err := m.evalCSS(s.Value, ec.scope, prop)
		if err != nil {
			return err
		}
		val = v
	} else {
		val, frozen, err = m.liveValue(s.Value, ec.scope, prop)
		if err != nil {
			return err
		}
	}
	// add-property() inside a function in the value lands before the
	// declaration, as in the evaluator.
	m.drain(mc)
	node := &mnode{kind: mDecl, prop: prop, val: val, important: s.Important}
	if len(frozen) > 0 {
		n, first := m.note(ec, s.Line, s.Col, "frozen",
			fmt.Sprintf("%s computed at migrate time from %s; it won't follow runtime changes", prop, strings.Join(frozen, ", ")))
		m.lead(node, n, first)
	}
	mc.node.kids = append(mc.node.kids, node)
	return nil
}

// evalCSS evaluates a declaration value (or part of one) to CSS text,
// rejecting objects as the evaluator does.
func (m *migrator) evalCSS(e ast.Expr, scope *Scope, prop string) (string, error) {
	v, err := m.ev.evalExpr(e, scope)
	if err != nil {
		return "", err
	}
	if _, isObj := value.Deref(v).(*value.Hash); isObj {
		return "", fmt.Errorf("property %q: an object is not a CSS value (read a key with obj.key or obj[key])", prop)
	}
	return v.CSS(true), nil
}

// liveValue renders a value that reads custom-property variables, keeping
// them live where CSS can: list items render one by one, arithmetic
// becomes calc() when calcTerm accepts it, and everything else evaluates
// normally (a bare variable renders var(--name) by itself). frozen lists
// the properties, as "--name", that some item read but did not keep.
func (m *migrator) liveValue(e ast.Expr, scope *Scope, prop string) (val string, frozen []string, err error) {
	addFrozen := func(names []string) {
		for _, n := range names {
			if !slices.Contains(frozen, n) {
				frozen = append(frozen, n)
			}
		}
	}
	if l, ok := e.(*ast.List); ok {
		parts := make([]string, len(l.Items))
		for i, it := range l.Items {
			p, fz, err := m.liveValue(it, scope, prop)
			if err != nil {
				return "", nil, err
			}
			parts[i] = p
			addFrozen(fz)
		}
		sep := " "
		if l.Comma {
			sep = ", "
		}
		return strings.Join(parts, sep), frozen, nil
	}
	refs := varRefs(e, scope)
	if b, ok := e.(*ast.Binary); ok && isCalcOp(b.Op) && !b.Literal && len(refs) > 0 {
		if inner, _, ok := m.calcTerm(b, scope); ok {
			return "calc(" + inner + ")", nil, nil
		}
	}
	val, err = m.evalCSS(e, scope, prop)
	if err != nil {
		return "", nil, err
	}
	addFrozen(missingVars(val, refs))
	return val, frozen, nil
}

// drain moves output the evaluator produced directly — declarations from
// add-property() in ec.rule, rules or at-rules in ec.sink — into the tree.
// Sink nodes carry absolute selectors, so they are hoisted as raw text.
func (m *migrator) drain(mc *mctx) {
	ec := mc.ec
	if ec.rule != nil && len(ec.rule.Statements) > 0 {
		for _, st := range ec.rule.Statements {
			mc.node.kids = append(mc.node.kids, &mnode{kind: mDecl, prop: st.Property, val: st.Value, important: st.Important})
		}
		ec.rule.Statements = nil
	}
	if ec.sink != nil && len(*ec.sink) > 0 {
		text := css.RenderSheet(*ec.sink, true, nil)
		*ec.sink = (*ec.sink)[:0]
		if strings.TrimSpace(text) != "" {
			m.hoistNode(mc, &mnode{kind: mRaw, head: text})
		}
	}
}

// scratchRule is the css.Rule a style-rule context carries. Declarations
// go to the mnode tree, but ctx.rule must be non-nil for the evaluator's
// "inside a selector" checks and add-property().
func scratchRule(sels []string) *css.Rule {
	return &css.Rule{Selector: joinSelectors(sels, true), Selectors: sels}
}

// ruleSet mirrors evalRuleSet. The rule nests under the current node when
// its selectors can be written relative to the parent; otherwise it is
// hoisted with its combined selectors.
func (m *migrator) ruleSet(rs *ast.RuleSet, mc *mctx) error {
	ec := mc.ec
	selfs := make([]string, len(rs.Selectors))
	for i, s := range rs.Selectors {
		r, err := m.ev.interpolate(s, ec.scope)
		if err != nil {
			return err
		}
		if ec.prefix != "" {
			r = prefixClasses(r, ec.prefix)
		}
		selfs[i] = r
	}
	combined := combineSelectors(ec.parents, selfs, true)
	if len(combined) > maxSelectors {
		return fmt.Errorf("combined selector count exceeds %d — runaway selector nesting?", maxSelectors)
	}

	node := &mnode{kind: mRule, abs: combined, placeholder: allPlaceholders(combined)}
	m.rules = append(m.rules, node)
	switch rel, ok := relativize(selfs, ec.parents); {
	case !mc.inRule:
		// No enclosing style rule: combined is what the source means.
		node.sels, node.rootLevel = combined, true
		mc.node.kids = append(mc.node.kids, node)
	case ok:
		node.sels = rel
		mc.node.kids = append(mc.node.kids, node)
	default:
		node.sels, node.rootLevel = combined, true
		n, first := m.note(ec, rs.Line, rs.Col, "hoisted",
			fmt.Sprintf("%q can't nest in CSS; moved after the enclosing block as %s (check cascade order)",
				strings.Join(selfs, ", "), joinSelectors(combined, true)))
		m.lead(node, n, first)
		m.hoistNode(mc, node)
	}

	child := &execCtx{scope: ec.scope.Child(), rule: scratchRule(combined), parents: combined, sink: ec.sink,
		dir: ec.dir, file: ec.file, mixin: ec.mixin,
		stack: append(ec.stack[:len(ec.stack):len(ec.stack)], selfs), media: ec.media, prefix: ec.prefix, block: ec.block}
	if err := m.walk(rs.Body, &mctx{ec: child, node: node, chain: mc.chain, inRule: true}); err != nil {
		return err
	}
	if child.returned {
		ec.ret = child.ret
		ec.returned = true
	}
	return nil
}

// hoistNode queues node for the root, wrapped in mc's enclosing at-rules.
func (m *migrator) hoistNode(mc *mctx, node *mnode) {
	m.pending = append(m.pending, hoist{chain: slices.Clone(mc.chain), node: node})
}

// flush appends queued hoists to the root. Consecutive hoists under the
// same at-rule chain share one wrapper.
func (m *migrator) flush() {
	var lastChain []string
	var lastInner *mnode
	for _, h := range m.pending {
		if lastInner != nil && slices.Equal(h.chain, lastChain) {
			lastInner.kids = append(lastInner.kids, h.node)
			continue
		}
		parent := m.root
		for _, head := range h.chain {
			w := &mnode{kind: mAt, head: head}
			parent.kids = append(parent.kids, w)
			parent = w
		}
		parent.kids = append(parent.kids, h.node)
		lastChain, lastInner = h.chain, parent
		if len(h.chain) == 0 {
			// Bare root: nothing to share.
			lastInner = nil
		}
	}
	m.pending = m.pending[:0]
}

// relativize rewrites a nested rule's own selectors for CSS nesting, or
// reports false when some selector cannot be written relative to parents:
//
//	:hover     → &:hover   go-styl attaches a bare pseudo (CSS nesting
//	                       would read it as a descendant `& :hover`)
//	> li, .x   → as is     relative selectors are valid nested CSS
//	.a &, & + & → as is    & means the parent in both languages
//	&__el      → false     & concatenation has no CSS form
//	$ph        → false     placeholders are resolved at the root
//
// Any parent with a pseudo-element also gives false: CSS & is :is(parent),
// and :is() cannot contain pseudo-elements.
func relativize(selfs, parents []string) ([]string, bool) {
	for _, p := range parents {
		if hasPseudoElement(p) {
			return nil, false
		}
	}
	out := make([]string, len(selfs))
	for i, s := range selfs {
		switch {
		case s == "":
			return nil, false
		case strings.HasPrefix(s, "$"):
			return nil, false
		case strings.Contains(s, "&"):
			for j := 0; j < len(s); j++ {
				if s[j] == '&' && j+1 < len(s) && isClassRune(rune(s[j+1])) {
					return nil, false
				}
			}
			out[i] = s
		case s[0] == ':':
			out[i] = "&" + s
		default:
			out[i] = s
		}
	}
	return out, true
}

// hasPseudoElement reports whether a selector contains a pseudo-element:
// `::x`, or a legacy single-colon :before/:after/:first-line/:first-letter.
func hasPseudoElement(sel string) bool {
	if strings.Contains(sel, "::") {
		return true
	}
	low := strings.ToLower(sel)
	for _, pe := range []string{":before", ":after", ":first-line", ":first-letter"} {
		for i := 0; ; {
			j := strings.Index(low[i:], pe)
			if j < 0 {
				break
			}
			end := i + j + len(pe)
			if end == len(low) || !isClassRune(rune(low[end])) {
				return true
			}
			i = end
		}
	}
	return false
}

// --- mixins and control flow ---

// mixinCall expands a user mixin in place by walking its body in the
// caller's output node, so rules inside the mixin nest like any others.
// It mirrors evalMixinCall/invoke; built-ins called as statements and the
// error paths go to the evaluator.
func (m *migrator) mixinCall(s *ast.MixinCall, mc *mctx) error {
	ec := mc.ec
	cl, ok := ec.scope.GetFunc(s.Name)
	if s.Block != nil && !ok {
		return m.blockMixinCall(s, mc)
	}
	if !ok {
		if err := m.ev.evalMixinCall(s, ec); err != nil {
			return err
		}
		m.drain(mc)
		return nil
	}
	args, err := m.ev.evalArgs(s.Args, ec.scope)
	if err != nil {
		return err
	}
	if m.ev.depth >= maxCallDepth {
		return fmt.Errorf("call depth exceeds %d in %q — unbounded recursion?", maxCallDepth, cl.Def.Name)
	}
	m.ev.depth++
	defer func() { m.ev.depth-- }()

	fscope := cl.Scope.Child()
	if err := m.ev.bindParams(fscope, cl.Def.Params, args); err != nil {
		return err
	}
	fctx := &execCtx{scope: fscope, file: cl.File, mixin: cl.Def.Name, block: newPassedBlock(s.Block, ec),
		rule: ec.rule, parents: ec.parents, sink: ec.sink, dir: ec.dir,
		stack: ec.stack, media: ec.media, prefix: ec.prefix, propRule: ec.rule}
	if fctx.propRule == nil {
		fctx.propRule = ec.propRule
	}
	mark := len(mc.node.kids)
	if err := m.walk(cl.Def.Body, &mctx{ec: fctx, node: mc.node, chain: mc.chain, inRule: mc.inRule}); err != nil {
		return err
	}
	m.annotate(ec, mc.node, mark, s.Line, s.Col, "mixin", "expanded "+callText(s.Name, args))
	return nil
}

// blockMixinCall handles +prefix-classes(p) with its block: the prefix is
// baked into the class names of the block's selectors. (A block passed to a
// user mixin goes through mixinCall and is walked at its {block}.)
func (m *migrator) blockMixinCall(s *ast.MixinCall, mc *mctx) error {
	ec := mc.ec
	if s.Name != "prefix-classes" {
		return fmt.Errorf("undefined block mixin %q", s.Name)
	}
	args, err := m.ev.evalArgs(s.Args, ec.scope)
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return fmt.Errorf("prefix-classes() expects 1 argument, got %d", len(args))
	}
	prefix, err := nameArg("prefix-classes", value.Deref(args[0]))
	if err != nil {
		return err
	}
	saved := ec.prefix
	ec.prefix = prefix
	defer func() { ec.prefix = saved }()
	mark := len(mc.node.kids)
	if err := m.walk(s.Block, mc); err != nil {
		return err
	}
	m.annotate(ec, mc.node, mark, s.Line, s.Col, "prefix", fmt.Sprintf("class prefix %q applied by +prefix-classes", prefix))
	return nil
}

// callText renders a mixin call with its evaluated arguments for a note,
// e.g. button(var(--primary), 4px).
func callText(name string, args []value.Value) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = a.CSS(true)
	}
	s := name + "(" + strings.Join(parts, ", ") + ")"
	if len(s) > 80 {
		s = s[:77] + "..."
	}
	return s
}

// ifStmt runs the first true branch in place, like evalIf. The choice is
// made at migrate time; when the condition reads a custom-property
// variable that is worth a note, since changing the property at runtime
// won't switch branches.
func (m *migrator) ifStmt(s *ast.If, mc *mctx) error {
	ec := mc.ec
	body := s.Else
	var refs []string
	for _, br := range s.Branches {
		refs = append(refs, varRefs(br.Cond, ec.scope)...)
		cond, err := m.ev.evalExpr(br.Cond, ec.scope)
		if err != nil {
			return err
		}
		if value.Truthy(cond) {
			body = br.Body
			break
		}
	}
	mark := len(mc.node.kids)
	if err := m.walk(body, mc); err != nil {
		return err
	}
	if len(refs) > 0 {
		for i := range refs {
			refs[i] = "--" + refs[i]
		}
		m.annotate(ec, mc.node, mark, s.Line, s.Col, "condition",
			"branch chosen at migrate time from "+strings.Join(refs, ", ")+"; it won't switch at runtime")
	}
	return nil
}

// forStmt unrolls a loop in place, like evalFor/evalForHash.
func (m *migrator) forStmt(s *ast.For, mc *mctx) error {
	ec := mc.ec
	iter, err := m.ev.evalExpr(s.Iterable, ec.scope)
	if err != nil {
		return err
	}
	mark := len(mc.node.kids)
	n := 0
	if h, ok := value.Deref(iter).(*value.Hash); ok {
		keys := append([]string(nil), h.Keys()...)
		for _, k := range keys {
			ec.scope.Set(s.Value, &value.Str{Val: k, Quote: '\''})
			if s.Index != "" {
				v, _ := h.Get(k)
				ec.scope.Set(s.Index, v)
			}
			n++
			if err := m.walk(s.Body, mc); err != nil {
				return err
			}
			if ec.returned {
				break
			}
		}
	} else {
		for idx, item := range iterItems(iter) {
			if s.Index != "" {
				ec.scope.Set(s.Index, &value.Number{Num: float64(idx)})
			}
			ec.scope.Set(s.Value, item)
			n++
			if err := m.walk(s.Body, mc); err != nil {
				return err
			}
			if ec.returned {
				break
			}
		}
	}
	m.annotate(ec, mc.node, mark, s.Line, s.Col, "loop", fmt.Sprintf("unrolled for-loop (%d iterations)", n))
	return nil
}

// --- @import, at-rules, @extend ---

// importStmt mirrors evalImport: literal imports pass through, .styl
// imports are inlined (they share scope, so their variables and mixins
// are needed right here).
func (m *migrator) importStmt(s *ast.Import, mc *mctx) error {
	ec := mc.ec
	if s.Literal {
		if s.Once {
			key := "\x00literal:" + s.Path
			if m.ev.required[key] {
				return nil
			}
			m.ev.required[key] = true
		}
		mc.node.kids = append(mc.node.kids, &mnode{kind: mRaw, head: importStmt(s.Path)})
		return nil
	}
	// Same sandbox rule as evalImport: no OS-disk resolution at all.
	if m.ev.opts.Sandbox != nil && m.ev.opts.FS == nil {
		return limitErr("@import %q: imports need Options.FS in a sandbox", s.Path)
	}
	files, err := resolveImport(m.ev.opts.FS, ec.dir, s.Path, m.ev.opts.IncludePaths)
	if err != nil {
		return err
	}
	mark := len(mc.node.kids)
	for _, abs := range files {
		if s.Once {
			if m.ev.required[abs] {
				continue
			}
			m.ev.required[abs] = true
		}
		if m.ev.importing[abs] {
			return fmt.Errorf("import cycle detected at %q", abs)
		}
		if err := m.ev.checkImport(s.Path, abs); err != nil {
			return err
		}
		m.ev.deps = append(m.ev.deps, abs)
		data, err := m.ev.readFile(abs)
		if err != nil {
			return fmt.Errorf("@import %q: %w", s.Path, err)
		}
		if err := m.ev.chargeSource(s.Path, len(data)); err != nil {
			return err
		}
		sheet, err := parser.Parse(string(data))
		if err != nil {
			return diag.SetFile(err, abs)
		}
		m.ev.importing[abs] = true
		importCtx := *ec
		importCtx.dir = dirOf(m.ev.opts.FS, abs)
		importCtx.file = abs
		err = m.walk(sheet.Statements, &mctx{ec: &importCtx, node: mc.node, chain: mc.chain, inRule: mc.inRule})
		delete(m.ev.importing, abs)
		if err != nil {
			return err
		}
		if importCtx.returned {
			ec.ret, ec.returned = importCtx.ret, true
		}
	}
	m.annotate(ec, mc.node, mark, s.Line, s.Col, "import", fmt.Sprintf("inlined @import %q", s.Path))
	return nil
}

// atRule mirrors evalAtRule. Conditional group rules (@media, @supports,
// @container, @layer …) stay where they are: CSS nesting allows them
// inside a style rule, declarations included. @keyframes, @font-face and
// @page can't sit inside a style rule and are hoisted from one.
func (m *migrator) atRule(s *ast.AtRule, mc *mctx) error {
	ec := mc.ec
	params, err := m.ev.interpolate(s.Params, ec.scope)
	if err != nil {
		return err
	}
	head := func(p string) string {
		if p == "" {
			return "@" + s.Name
		}
		return "@" + s.Name + " " + p
	}
	if s.Body == nil {
		mc.node.kids = append(mc.node.kids, &mnode{kind: mRaw, head: head(params) + ";"})
		return nil
	}
	node := &mnode{kind: mAt}
	place := func() {
		if mc.inRule {
			n, first := m.note(ec, s.Line, s.Col, "hoisted", node.head+" can't nest in a style rule; moved after the enclosing block")
			m.lead(node, n, first)
			m.hoistNode(mc, node)
			return
		}
		mc.node.kids = append(mc.node.kids, node)
	}

	switch atKind(s.Name) {
	case "font-face", "page", "viewport":
		node.head = head(params)
		place()
		child := &execCtx{scope: ec.scope.Child(), rule: scratchRule([]string{node.head}), sink: ec.sink, dir: ec.dir, file: ec.file, block: ec.block}
		return m.walk(s.Body, &mctx{ec: child, node: node, chain: mc.chain})
	case "keyframes":
		node.head = head(params)
		place()
		// Frame selectors (from, 50%) never combine with a parent.
		child := &execCtx{scope: ec.scope.Child(), sink: ec.sink, dir: ec.dir, file: ec.file, block: ec.block}
		return m.walk(s.Body, &mctx{ec: child, node: node, chain: mc.chain})
	default:
		// var() is invalid in media queries, so variables resolve to their
		// migrate-time values here, as in the evaluator.
		node.head = head(m.ev.evalAtVars(params, ec.scope))
		mc.node.kids = append(mc.node.kids, node)
		child := &execCtx{scope: ec.scope.Child(), parents: ec.parents, sink: ec.sink, dir: ec.dir,
			file: ec.file, mixin: ec.mixin, stack: ec.stack, media: ec.media, prefix: ec.prefix, block: ec.block}
		if strings.HasPrefix(node.head, "@media") {
			child.media = node.head
		}
		if len(ec.parents) > 0 {
			child.rule = scratchRule(ec.parents)
		}
		chain := append(slices.Clone(mc.chain), node.head)
		return m.walk(s.Body, &mctx{ec: child, node: node, chain: chain, inRule: mc.inRule})
	}
}

// extend queues an @extend; see applyExtends.
func (m *migrator) extend(s *ast.Extend, mc *mctx) error {
	ec := mc.ec
	if ec.rule == nil {
		return fmt.Errorf("@extend %q must appear inside a selector", s.Target)
	}
	target, err := m.ev.interpolate(s.Target, ec.scope)
	if err != nil {
		return err
	}
	m.extends = append(m.extends, mExtend{target: target, extenders: slices.Clone(ec.parents),
		file: ec.file, line: s.Line, col: s.Col})
	return nil
}

// applyExtends resolves @extend the way CSS would have been written by
// hand: the extending selectors join the target rule's selector list
// (`.message, .warning { … }`), and a $placeholder rule takes the
// extenders as its whole list. That needs a target whose output selectors
// are absolute; a target nested inside another rule is left for manual
// work, noted.
func (m *migrator) applyExtends() {
	for _, ex := range m.extends {
		ec := &execCtx{file: ex.file}
		var targets []*mnode
		for _, r := range m.rules {
			if slices.Contains(r.abs, ex.target) && (r.placeholder || !strings.HasPrefix(ex.target, "$")) {
				targets = append(targets, r)
			}
		}
		if len(targets) == 0 {
			m.note(ec, ex.line, ex.col, "extend", fmt.Sprintf("@extend %s: no matching rule; dropped", ex.target))
			continue
		}
		for _, t := range targets {
			if !t.rootLevel {
				n, first := m.note(ec, ex.line, ex.col, "extend",
					fmt.Sprintf("@extend %s targets a nested rule; add %s to it by hand",
						ex.target, joinSelectors(ex.extenders, true)))
				m.lead(t, n, first)
				continue
			}
			for _, e := range ex.extenders {
				if !slices.Contains(t.ext, e) {
					t.ext = append(t.ext, e)
				}
			}
			if hasRuleKids(t) {
				n, first := m.note(ec, ex.line, ex.col, "extend",
					fmt.Sprintf("@extend %s: rules nested under it now also match %s", ex.target, joinSelectors(ex.extenders, true)))
				m.lead(t, n, first)
			}
		}
	}
}

// hasRuleKids reports whether a rule nests other rules.
func hasRuleKids(n *mnode) bool {
	for _, k := range n.kids {
		if k.kind == mRule || k.kind == mAt {
			return true
		}
	}
	return false
}

// --- rendering ---

// render prints the tree, with a :root rule for the custom properties the
// output actually uses. Variables only read by arithmetic, conditions or
// media queries never appear as var(), so they don't get a property.
func (m *migrator) render() string {
	body := m.renderKids(m.root.kids, 0)
	if rootRule := m.rootRule(body); rootRule != nil {
		// @charset and @import must stay ahead of every rule.
		i := 0
		for i < len(m.root.kids) && m.root.kids[i].kind == mRaw && isPreamble(m.root.kids[i].head) {
			i++
		}
		m.root.kids = slices.Insert(m.root.kids, i, rootRule)
		body = m.renderKids(m.root.kids, 0)
	}
	if m.opts.Notes && len(m.notes) > 0 {
		src := m.opts.Filename
		if src == "" {
			src = "Stylus source"
		}
		body = fmt.Sprintf("/* Migrated from %s by styl migrate. Review each styl-migrate note. */\n\n", commentSafe(src)) + body
	}
	return body
}

// isPreamble reports whether raw text is an at-rule that must precede
// style rules.
func isPreamble(s string) bool {
	return strings.HasPrefix(s, "@charset") || strings.HasPrefix(s, "@import")
}

// rootRule builds `:root { --x: … }` for every custom property referenced
// by body, following references between the properties themselves
// (link = primary gives --link: var(--primary), which keeps --primary).
func (m *migrator) rootRule(body string) *mnode {
	used := map[string]bool{}
	for _, v := range m.vars {
		if strings.Contains(body, "var(--"+v.Name+")") {
			used[v.Name] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, v := range m.vars {
			if !used[v.Name] {
				continue
			}
			def := m.varDef(v)
			for _, w := range m.vars {
				if !used[w.Name] && strings.Contains(def, "var(--"+w.Name+")") {
					used[w.Name] = true
					changed = true
				}
			}
		}
	}
	rule := &mnode{kind: mRule, sels: []string{":root"}}
	for _, v := range m.vars {
		if used[v.Name] {
			node := &mnode{kind: mDecl, prop: "--" + v.Name, val: m.varDef(v)}
			if fz, ok := m.frozenDefs[v]; ok {
				n, first := m.note(&execCtx{file: fz.File}, fz.Line, fz.Col, fz.Kind, fz.Msg)
				m.lead(node, n, first)
			}
			rule.kids = append(rule.kids, node)
		}
	}
	if len(rule.kids) == 0 {
		return nil
	}
	return rule
}

// varDef is a custom property's :root value: the live form recorded by
// assign, else the migrate-time value (which renders var(--x) for an
// alias of another property).
func (m *migrator) varDef(v *value.Var) string {
	if d, ok := m.defs[v]; ok {
		return d
	}
	return v.Inner.CSS(true)
}

// selectors returns a rule's output selector list: its own (none for a
// placeholder) plus any @extend additions.
func (n *mnode) selectors() []string {
	var out []string
	if !n.placeholder {
		out = append(out, n.sels...)
	}
	return append(out, n.ext...)
}

// hasOutput reports whether a node renders anything; empty rules and
// at-rules are dropped, as in compiled output.
func (n *mnode) hasOutput() bool {
	switch n.kind {
	case mDecl:
		return true
	case mRaw:
		return strings.TrimSpace(n.head) != ""
	case mRule:
		if len(n.selectors()) == 0 {
			return false
		}
	}
	for _, k := range n.kids {
		if k.hasOutput() {
			return true
		}
	}
	return false
}

// renderKids prints sibling nodes at depth, blank-line separated between
// blocks, with two-space indentation.
func (m *migrator) renderKids(kids []*mnode, depth int) string {
	var b strings.Builder
	ind := strings.Repeat("  ", depth)
	prevBlock := false
	for _, k := range kids {
		if !k.hasOutput() {
			continue
		}
		block := k.kind != mDecl
		if b.Len() > 0 && (block || prevBlock) {
			b.WriteString("\n")
		}
		prevBlock = block
		for _, c := range k.lead {
			b.WriteString(ind + "/* styl-migrate: " + commentSafe(c) + " */\n")
		}
		switch k.kind {
		case mDecl:
			b.WriteString(ind + k.prop + ": " + k.val)
			if k.important {
				b.WriteString(" !important")
			}
			b.WriteString(";\n")
		case mRaw:
			for _, line := range strings.Split(strings.TrimRight(k.head, "\n"), "\n") {
				b.WriteString(ind + line + "\n")
			}
		case mRule, mAt:
			head := k.head
			if k.kind == mRule {
				head = joinSelectors(k.selectors(), true)
			}
			b.WriteString(ind + head + " {\n")
			b.WriteString(m.renderKids(k.kids, depth+1))
			b.WriteString(ind + "}\n")
		}
	}
	return b.String()
}

// commentSafe keeps note text from closing its comment early.
func commentSafe(s string) string {
	return strings.ReplaceAll(s, "*/", "* /")
}

// missingVars returns, as "--name", each custom property in refs that val
// does not read through var().
func missingVars(val string, refs []string) []string {
	var out []string
	for _, name := range refs {
		if !strings.Contains(val, "var(--"+name+")") {
			out = append(out, "--"+name)
		}
	}
	return out
}

// --- calc() ---

// calcTerm and friends turn Stylus arithmetic on variables into calc():
// `base * 10` → calc(var(--base) * 10). Stylus arithmetic and calc() only
// agree when the units line up, so each operation is checked against the
// operands' migrate-time values:
//
//	a + b, a - b   same unit on both sides (Stylus lets 8px + 2 mean
//	               10px and 1em + 8px mean 9em; calc() rejects the first
//	               and means something else by the second)
//	a * b          at least one side unitless (px * px is px in Stylus,
//	               invalid in calc())
//	a / b          b unitless and non-zero

func isCalcOp(op token.Kind) bool {
	return op == token.PLUS || op == token.MINUS || op == token.STAR || op == token.SLASH
}

// calcPrec is an operator's binding strength inside calc().
func calcPrec(op token.Kind) int {
	if op == token.STAR || op == token.SLASH {
		return 2
	}
	return 1
}

// calcTerm renders one calc() operand and returns its migrate-time number
// for the unit checks. A variable renders as var(--name) (or, for a
// variable that isn't a custom property, its value); a nested operation
// renders bare, parenthesized by the caller when precedence needs it.
func (m *migrator) calcTerm(e ast.Expr, scope *Scope) (string, *value.Number, bool) {
	if b, ok := e.(*ast.Binary); ok && isCalcOp(b.Op) && !b.Literal {
		l, ln, ok := m.calcTerm(b.L, scope)
		if !ok {
			return "", nil, false
		}
		r, rn, ok := m.calcTerm(b.R, scope)
		if !ok {
			return "", nil, false
		}
		switch b.Op {
		case token.PLUS, token.MINUS:
			if ln.Unit != rn.Unit {
				return "", nil, false
			}
		case token.STAR:
			if ln.Unit != "" && rn.Unit != "" {
				return "", nil, false
			}
		case token.SLASH:
			if rn.Unit != "" || rn.Num == 0 {
				return "", nil, false
			}
		}
		// Parenthesize a looser child, and a same-strength right child of
		// a non-associative operator: a - (b + c), a / (b * c).
		if lb, isB := b.L.(*ast.Binary); isB && calcPrec(lb.Op) < calcPrec(b.Op) {
			l = "(" + l + ")"
		}
		if rb, isB := b.R.(*ast.Binary); isB {
			if calcPrec(rb.Op) < calcPrec(b.Op) ||
				(calcPrec(rb.Op) == calcPrec(b.Op) && (b.Op == token.MINUS || b.Op == token.SLASH)) {
				r = "(" + r + ")"
			}
		}
		v, err := m.ev.evalExpr(b, scope)
		if err != nil {
			return "", nil, false
		}
		n, isNum := value.Deref(v).(*value.Number)
		if !isNum {
			return "", nil, false
		}
		return l + " " + opText(b.Op) + " " + r, n, true
	}
	switch e.(type) {
	case *ast.Ident, *ast.NumberLit:
	default:
		// Calls, colors, strings: not calc() material.
		return "", nil, false
	}
	v, err := m.ev.evalExpr(e, scope)
	if err != nil {
		return "", nil, false
	}
	n, isNum := value.Deref(v).(*value.Number)
	if !isNum {
		return "", nil, false
	}
	return v.CSS(true), n, true
}
