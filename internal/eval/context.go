package eval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/builtin"
	"github.com/rohanthewiz/go-styl/internal/css"
	"github.com/rohanthewiz/go-styl/internal/parser"
	"github.com/rohanthewiz/go-styl/internal/value"
)

// Context built-ins: Stylus functions that need more than their argument
// values — the call site's selector stack, media query, scope or rule, the
// filesystem, or the compile options. They live here rather than in the
// builtin package (whose functions are pure) and read the call site from
// ev.cur, which execStmt keeps pointing at the executing statement's context.
//
// Each gets the call's AST (for argument expressions), its evaluated
// arguments (custom-property wrappers already resolved) and the scope the
// call evaluates in.
type ctxBuiltin func(ev *evaluator, c *ast.Call, args []value.Value, scope *Scope) (value.Value, error)

// ctxBuiltins is filled in init: the functions call back into the evaluator,
// and a map literal referring to them would be an initialization cycle.
var ctxBuiltins map[string]ctxBuiltin

func init() {
	ctxBuiltins = map[string]ctxBuiltin{
		"selector":        biSelector,
		"selectors":       biSelectors,
		"selector-exists": biSelectorExists,
		"current-media":   biCurrentMedia,
		"define":          biDefine,
		"lookup":          biLookup,
		"add-property":    biAddProperty,
		"warn":            biWarn,
		"use":             biUse,
		"json":            biJSON,
		"prefix-classes":  biPrefixClassesNoBlock,
	}
}

// ctxBuiltinSigs holds each context built-in's parameter list, in the form
// builtin.register takes (see builtin.Signatures). ctxBuiltins can't carry
// them in its values without every caller unwrapping a struct, so they sit
// here beside it; TestBuiltinSignatures checks the two stay in step.
var ctxBuiltinSigs = map[string]string{
	"selector":        "selector(selectors...)",
	"selectors":       "selectors()",
	"selector-exists": "selector-exists(selector)",
	"current-media":   "current-media()",
	"define":          "define(name, value, global = false)",
	"lookup":          "lookup(name)",
	"add-property":    "add-property(name, expr)",
	"warn":            "warn(msg)",
	"use":             "use(path)",
	"json":            "json(path, options?, prefix?)",
	"prefix-classes":  "prefix-classes(prefix)",
}

// isBuiltinName reports whether name is a built-in of either kind.
func isBuiltinName(name string) bool {
	if _, ok := ctxBuiltins[name]; ok {
		return true
	}
	_, ok := builtin.Lookup(name)
	return ok
}

// callSite returns the executing statement's context, or an empty root
// context when there is none (while seeding Options.Globals).
func (ev *evaluator) callSite() *execCtx {
	if ev.cur != nil {
		return ev.cur
	}
	return &execCtx{scope: ev.rootScope, dir: ev.opts.BaseDir, file: ev.opts.Filename}
}

// --- selector(), selectors(), selector-exists() ---

// biSelector implements selector(): the current compiled selector as a quoted
// string, '&' at the root. Arguments nest further selectors below the current
// one without emitting a rule:
//
//	selector('.x')            '.x' — no parent reference, returned as given
//	selector('&:hover')       the current selector(s) with :hover
//	selector('.x', '.y')      current › .x › .y (each argument is a level)
//	selector('.x' '.y')       the same, as one level `.x .y`
//	selector(list)            a comma list: each item is a level
//
// A selector group joins with "," (no space) whatever the output style, as in
// Stylus.
func biSelector(ev *evaluator, _ *ast.Call, args []value.Value, _ *Scope) (value.Value, error) {
	site := ev.callSite()
	var levels []string
	switch len(args) {
	case 0:
	case 1:
		switch a := args[0].(type) {
		case *value.Str:
			// Stylus returns a selector without a parent reference unchanged,
			// rather than nesting it under the current one.
			if !strings.Contains(a.Val, "&") && !strings.Contains(a.Val, "^[") {
				return &value.Str{Val: a.Val, Quote: '\''}, nil
			}
			levels = []string{a.Val}
		case *value.List:
			parts, err := selectorStrings(a.Items)
			if err != nil {
				return nil, err
			}
			if a.Comma {
				levels = parts
			} else {
				levels = []string{strings.Join(parts, " ")}
			}
		default:
			return nil, fmt.Errorf("selector() expects a string, got %s", args[0].TypeName())
		}
	default:
		parts, err := selectorStrings(args)
		if err != nil {
			return nil, err
		}
		levels = parts
	}

	sels := site.parents
	for _, lvl := range levels {
		sels = combineSelectors(sels, parser.SplitSelectors(lvl), ev.opts.Pretty)
	}
	if len(sels) == 0 {
		return &value.Str{Val: "&", Quote: '\''}, nil
	}
	return &value.Str{Val: strings.Join(sels, ","), Quote: '\''}, nil
}

// selectorStrings requires every value to be a string and returns their
// contents.
func selectorStrings(vs []value.Value) ([]string, error) {
	out := make([]string, len(vs))
	for i, v := range vs {
		s, ok := value.Deref(v).(*value.Str)
		if !ok {
			return nil, fmt.Errorf("selector() expects strings, got %s", v.TypeName())
		}
		out[i] = s.Val
	}
	return out, nil
}

// biSelectors implements selectors(): a comma list with one quoted string per
// nesting level, outermost first. Below the first level, a selector without a
// parent reference gets an explicit "& " prefix, so each string reads as it
// would be nested:
//
//	.a, .b
//	  .c &:hover
//	    .d          selectors() → '.a,.b', '.c &:hover', '& .d'
//
// At the root it is the single string '&'.
func biSelectors(ev *evaluator, _ *ast.Call, args []value.Value, _ *Scope) (value.Value, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("selectors() takes no arguments, got %d", len(args))
	}
	stack := ev.callSite().stack
	if len(stack) == 0 {
		return &value.List{Items: []value.Value{&value.Str{Val: "&", Quote: '\''}}, Comma: true}, nil
	}
	items := make([]value.Value, len(stack))
	for i, level := range stack {
		parts := make([]string, len(level))
		for j, sel := range level {
			if i > 0 && !strings.Contains(sel, "&") {
				sel = "& " + sel
			}
			parts[j] = sel
		}
		items[i] = &value.Str{Val: strings.Join(parts, ","), Quote: '\''}
	}
	return &value.List{Items: items, Comma: true}, nil
}

// biSelectorExists implements selector-exists(sel): whether a rule with that
// compiled selector has been created so far. Stylus's docs describe the same
// check; its 0.64 implementation normalizes a copy of the whole stylesheet,
// and crashes on nested rules. go-styl only sees rules compiled before the
// call, so a rule defined further down the file does not count yet.
// Whitespace, including around combinators, is ignored in the comparison.
func biSelectorExists(ev *evaluator, _ *ast.Call, args []value.Value, _ *Scope) (value.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("selector-exists() expects 1 argument, got %d", len(args))
	}
	s, ok := args[0].(*value.Str)
	if !ok {
		return nil, fmt.Errorf("selector-exists() expects a string, got %s", args[0].TypeName())
	}
	want := normalizeSelector(s.Val)
	for _, r := range ev.rules {
		for _, sel := range r.Selectors {
			if normalizeSelector(sel) == want {
				return &value.Bool{Val: true}, nil
			}
		}
	}
	return &value.Bool{Val: false}, nil
}

// normalizeSelector collapses whitespace runs to one space and drops spaces
// around the combinators > + ~, so `.a > .b`, `.a>.b` and `.a  >.b` compare
// equal (pretty and compressed output space combinators differently).
func normalizeSelector(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	for _, comb := range []string{">", "+", "~"} {
		s = strings.ReplaceAll(s, " "+comb, comb)
		s = strings.ReplaceAll(s, comb+" ", comb)
	}
	return s
}

// --- current-media() ---

// biCurrentMedia returns the innermost enclosing @media query as a quoted
// string ('@media screen and (max-width: 100px)'), or ” outside one.
// Stylus 0.64 wraps each query part in extra parentheses here
// ('@media (screen and (max-width: (100px)))'); go-styl returns the query as
// written, as the Stylus docs show it.
func biCurrentMedia(ev *evaluator, _ *ast.Call, args []value.Value, _ *Scope) (value.Value, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("current-media() takes no arguments, got %d", len(args))
	}
	return &value.Str{Val: ev.callSite().media, Quote: '\''}, nil
}

// --- define(), lookup() ---

// biDefine implements define(name, value, [global]): binds a variable whose
// name is computed, in the calling scope or, when global is true, the root
// scope.
func biDefine(ev *evaluator, _ *ast.Call, args []value.Value, scope *Scope) (value.Value, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("define() expects 2 or 3 arguments, got %d", len(args))
	}
	name, err := nameArg("define", args[0])
	if err != nil {
		return nil, err
	}
	target := scope
	if len(args) == 3 && value.Truthy(args[2]) {
		target = ev.rootScope
	}
	target.Set(name, ev.wrapVar(name, target, args[1]))
	return value.Null{}, nil
}

// biLookup implements lookup(name): the value of the variable with that
// name, or null when there is none.
func biLookup(_ *evaluator, _ *ast.Call, args []value.Value, scope *Scope) (value.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("lookup() expects 1 argument, got %d", len(args))
	}
	name, err := nameArg("lookup", args[0])
	if err != nil {
		return nil, err
	}
	if v, ok := scope.Get(name); ok {
		return v, nil
	}
	return value.Null{}, nil
}

// nameArg reads a variable or property name argument. Stylus requires a
// string; an unquoted ident is accepted too.
func nameArg(fn string, v value.Value) (string, error) {
	switch x := v.(type) {
	case *value.Str:
		return x.Val, nil
	case *value.Ident:
		return x.Name, nil
	}
	return "", fmt.Errorf("%s() name must be a string, got %s", fn, v.TypeName())
}

// --- add-property() ---

// biAddProperty implements add-property(name, expr): appends `name: expr` to
// the rule the call sits in. Called from a function used in a declaration
// value, it targets that declaration's rule, and since the declaration is
// appended after its value is evaluated, the added property lands just
// before it, as in Stylus:
//
//	k()                    .a
//	  add-property(bar, 1)   bar: 1
//	  10                     width: 10
//	.a
//	  width k()
func biAddProperty(ev *evaluator, _ *ast.Call, args []value.Value, _ *Scope) (value.Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("add-property() expects 2 arguments, got %d", len(args))
	}
	name, err := nameArg("add-property", args[0])
	if err != nil {
		return nil, err
	}
	site := ev.callSite()
	rule := site.rule
	if rule == nil {
		rule = site.propRule
	}
	if rule == nil {
		return nil, fmt.Errorf("add-property() must be used inside a selector")
	}
	rule.Statements = append(rule.Statements, &css.Statement{
		Property: name,
		Value:    args[1].CSS(ev.opts.Pretty),
		Pos:      rule.Pos,
	})
	return value.Null{}, nil
}

// --- warn(), use() ---

// biWarn implements warn(msg): hands msg to Options.Warn, or prints
// "Warning: msg" to stderr like the stylus CLI. Compilation continues.
func biWarn(ev *evaluator, _ *ast.Call, args []value.Value, _ *Scope) (value.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("warn() expects 1 argument, got %d", len(args))
	}
	msg := args[0].CSS(true)
	if s, ok := args[0].(*value.Str); ok {
		msg = s.Val
	}
	if ev.opts.Warn != nil {
		ev.opts.Warn(msg)
	} else if ev.opts.Sandbox != nil {
		// A sandboxed (tenant) sheet must not write to the host's stderr;
		// without a Warn hook its warnings are dropped.
	} else {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
	}
	return value.Null{}, nil
}

// biUse rejects use(): it loads a JavaScript plugin file, which a Go
// compiler cannot run.
func biUse(*evaluator, *ast.Call, []value.Value, *Scope) (value.Value, error) {
	return nil, fmt.Errorf("use() loads a JavaScript plugin, which go-styl cannot run (pass Go values in with Options.Globals instead)")
}

// --- prefix-classes ---

// biPrefixClassesNoBlock reports prefix-classes() called without the block it
// applies to.
func biPrefixClassesNoBlock(*evaluator, *ast.Call, []value.Value, *Scope) (value.Value, error) {
	return nil, fmt.Errorf("prefix-classes() applies to an indented block: +prefix-classes('p-') with nested rules below it")
}

// evalBlockMixinCall runs a `+name(args)` call that carries an indented
// block. A user mixin of that name (cl) receives the block and runs it at
// its `{block}` (see passedBlock); a user definition takes precedence over
// the built-in, as for ordinary calls. Otherwise the only block mixin is the
// built-in prefix-classes(prefix), which runs the block with a class prefix:
//
//	+prefix-classes('ui-')          .ui-btn { … }
//	  .btn                    →     .ui-btn.ui-big { … }
//	    &.big
//
// Selectors from enclosing levels are not prefixed (they are already
// compiled), and a nested +prefix-classes replaces the prefix, as in Stylus.
// cl is the user mixin of that name, if one is defined.
func (ev *evaluator) evalBlockMixinCall(s *ast.MixinCall, cl *Closure, ctx *execCtx) error {
	if cl != nil {
		args, err := ev.evalArgs(s.Args, ctx.scope)
		if err != nil {
			return err
		}
		_, err = ev.invoke(cl, args, ctx, newPassedBlock(s.Block, ctx))
		return err
	}
	if s.Name != "prefix-classes" {
		return fmt.Errorf("undefined block mixin %q", s.Name)
	}
	args, err := ev.evalArgs(s.Args, ctx.scope)
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
	saved := ctx.prefix
	ctx.prefix = prefix
	defer func() { ctx.prefix = saved }()
	return ev.execBlock(s.Block, ctx)
}

// blockSlotCtx resolves a `{block}` executed in ctx: the passed block's body
// and the context to run it in, which joins the call site's lexical side
// (scope, file, import dir, enclosing mixin and its block) with the slot's
// emission side (rule, selectors, sink, media, class prefix). The block gets
// a child scope, as a ruleset body does, so its assignments stay local.
//
// A mixin called without a block (`m()`) sees an empty slot, as in Stylus,
// so a mixin can take an optional block: body is nil and nothing runs.
// Outside any mixin Stylus also ignores `{block}`, but there it can only be
// a mistake, so it is reported.
func (ev *evaluator) blockSlotCtx(ctx *execCtx) (*execCtx, []ast.Stmt, error) {
	blk := ctx.block
	if blk == nil {
		if ctx.mixin == "" {
			return nil, nil, fmt.Errorf("{block} is only valid inside a mixin body")
		}
		return ctx, nil, nil
	}
	child := &execCtx{scope: blk.scope.Child(), file: blk.file, dir: blk.dir, mixin: blk.mixin, block: blk.outer,
		rule: ctx.rule, parents: ctx.parents, sink: ctx.sink, stack: ctx.stack, media: ctx.media,
		prefix: ctx.prefix, propRule: ctx.propRule}
	return child, blk.body, nil
}

// prefixClasses inserts prefix after every '.' that begins a class name
// (followed by a name character), the same textual rule as Stylus's
// /\.(?=[\w-])/ replacement: `.btn:not(.x)` → `.ui-btn:not(.ui-x)`.
func prefixClasses(sel, prefix string) string {
	var b strings.Builder
	runes := []rune(sel)
	for i, c := range runes {
		b.WriteRune(c)
		if c == '.' && i+1 < len(runes) && isClassRune(runes[i+1]) {
			b.WriteString(prefix)
		}
	}
	return b.String()
}

func isClassRune(c rune) bool {
	return c == '_' || c == '-' || (c >= '0' && c <= '9') ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// --- list mutation ---

// mutateListVar gives push/append/unshift/prepend/pop/shift Stylus's
// in-place effect: when the list argument is a variable, the variable is
// rebound to the list the call leaves behind (with the item added, or the
// popped/shifted item removed), in the scope that defines it.
//
// The return values stay go-styl's: push/unshift return the new list (Stylus
// returns its length), so `l = push(l, x)` keeps working. And go-styl lists
// are still values: after `b = a`, push(b, x) changes b only (in Stylus both
// names share one mutated list).
func (ev *evaluator) mutateListVar(c *ast.Call, args []value.Value, result value.Value, scope *Scope) {
	if len(c.Args) == 0 || len(args) == 0 {
		return
	}
	id, ok := c.Args[0].(*ast.Ident)
	if !ok {
		return
	}
	var next value.Value
	switch c.Name {
	case "push", "append", "unshift", "prepend":
		next = result
	case "pop", "shift":
		items, comma := listParts(args[0])
		if len(items) == 0 {
			return
		}
		if c.Name == "pop" {
			items = items[:len(items)-1]
		} else {
			items = items[1:]
		}
		next = &value.List{Items: append([]value.Value(nil), items...), Comma: comma}
	default:
		return
	}
	owner := scope.Owner(id.Name)
	if owner == nil {
		return // a bare word, not a variable
	}
	owner.Set(id.Name, ev.wrapVar(id.Name, owner, next))
}

// listParts returns the items a value represents as a list (null is empty, a
// lone value one item) and whether it is comma-separated.
func listParts(v value.Value) ([]value.Value, bool) {
	switch x := v.(type) {
	case *value.List:
		return x.Items, x.Comma
	case value.Null:
		return nil, false
	}
	return []value.Value{v}, false
}

// --- json() ---

// biJSON implements json(path, [options | local], [prefix]), reading a JSON
// file found like an @import (relative to the importing file, then the
// include paths; through Options.FS when set). The file becomes a build
// dependency. It has two modes, chosen by the second argument:
//
//   - An object (`json('theme.json', { hash: true })`) returns the file as
//     an object; nested JSON objects become nested objects. Options:
//     `optional: true` returns null for a missing file, and
//     `leave-strings: true` keeps string values as quoted strings instead of
//     parsing them as Stylus values ("#f00" → a color, "10px" → a number).
//   - Otherwise it defines one variable per leaf, named by joining the key
//     path with '-' (`{"a": {"b": 1}}` → a-b) after an optional name prefix,
//     in the root scope, or the calling scope when local is true. Strings are
//     always parsed.
func biJSON(ev *evaluator, _ *ast.Call, args []value.Value, scope *Scope) (value.Value, error) {
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("json() expects 1 to 3 arguments, got %d", len(args))
	}
	p, ok := args[0].(*value.Str)
	if !ok {
		return nil, fmt.Errorf("json() path must be a string, got %s", args[0].TypeName())
	}
	var opts *value.Hash
	if len(args) >= 2 {
		opts, _ = args[1].(*value.Hash)
	}
	optTrue := func(key string) bool {
		if opts == nil {
			return false
		}
		v, ok := opts.Get(key)
		return ok && value.Truthy(v)
	}

	site := ev.callSite()
	files, err := resolveImport(ev.opts.FS, site.dir, p.Val, ev.opts.IncludePaths)
	if err != nil || len(files) == 0 {
		if optTrue("optional") {
			return value.Null{}, nil
		}
		return nil, fmt.Errorf("json(): failed to locate %q", p.Val)
	}
	data, err := ev.readFile(files[0])
	if err != nil {
		return nil, fmt.Errorf("json(%q): %w", p.Val, err)
	}
	ev.deps = append(ev.deps, files[0])
	doc, err := decodeOrderedJSON(data)
	if err != nil {
		return nil, fmt.Errorf("json(%q): %w", p.Val, err)
	}

	if opts != nil {
		h, ok := ev.jsonToValue(doc, !optTrue("leave-strings")).(*value.Hash)
		if !ok {
			return nil, fmt.Errorf("json(%q): the file must hold a JSON object", p.Val)
		}
		return h, nil
	}

	target := ev.rootScope
	if len(args) >= 2 && value.Truthy(args[1]) {
		target = scope
	}
	namePrefix := ""
	if len(args) == 3 {
		if namePrefix, err = nameArg("json", args[2]); err != nil {
			return nil, err
		}
	}
	root, ok := doc.(*jsonObject)
	if !ok {
		return nil, fmt.Errorf("json(%q): the file must hold a JSON object", p.Val)
	}
	ev.defineJSONVars(root, namePrefix, "", target)
	return value.Null{}, nil
}

// defineJSONVars binds one variable per leaf of obj: prefix + the key path
// joined with '-'.
func (ev *evaluator) defineJSONVars(obj *jsonObject, namePrefix, path string, target *Scope) {
	for i, k := range obj.keys {
		name := k
		if path != "" {
			name = path + "-" + k
		}
		if nested, ok := obj.vals[i].(*jsonObject); ok {
			ev.defineJSONVars(nested, namePrefix, name, target)
			continue
		}
		full := namePrefix + name
		target.Set(full, ev.wrapVar(full, target, ev.jsonToValue(obj.vals[i], true)))
	}
}

// jsonToValue converts a decoded JSON value. Arrays become objects keyed by
// index ("0", "1", …), as they do in Stylus (which walks them with for…in).
// With parse set, a string is read as a Stylus value (see parseJSONString).
func (ev *evaluator) jsonToValue(v any, parse bool) value.Value {
	switch x := v.(type) {
	case *jsonObject:
		h := value.NewHash()
		for i, k := range x.keys {
			h.Set(k, ev.jsonToValue(x.vals[i], parse))
		}
		return h
	case []any:
		h := value.NewHash()
		for i, it := range x {
			h.Set(strconv.Itoa(i), ev.jsonToValue(it, parse))
		}
		return h
	case string:
		if parse {
			return ev.parseJSONString(x)
		}
		return &value.Str{Val: x, Quote: '\''}
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return &value.Ident{Name: x.String()}
		}
		return &value.Number{Num: f}
	case bool:
		return &value.Bool{Val: x}
	}
	return value.Null{}
}

// parseJSONString reads a JSON string as a Stylus value, as Stylus's
// utils.parseString does: "#f00" is a color, "10px" a number, "a b" a list of
// idents. It evaluates in an empty scope, so a string can't pick up the
// sheet's variables; one that doesn't parse stays literal text.
func (ev *evaluator) parseJSONString(s string) value.Value {
	e, err := parser.ParseExpr(s, 0)
	if err == nil {
		if v, err := ev.evalExpr(e, NewScope()); err == nil {
			return v
		}
	}
	return &value.Ident{Name: s}
}

// jsonObject is a decoded JSON object that keeps its keys in file order
// (encoding/json's map decoding would lose it, and Stylus keeps it).
type jsonObject struct {
	keys []string
	vals []any
}

// decodeOrderedJSON decodes a JSON document into *jsonObject, []any, string,
// json.Number, bool or nil values.
func decodeOrderedJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := decodeJSONValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err == nil {
		return nil, fmt.Errorf("unexpected data after the JSON value")
	}
	return v, nil
}

// decodeJSONValue reads one value from the token stream, recursing into
// objects and arrays.
func decodeJSONValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			obj := &jsonObject{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, _ := kt.(string)
				val, err := decodeJSONValue(dec)
				if err != nil {
					return nil, err
				}
				// A repeated key keeps its first position and the last value,
				// like a JS object.
				if i := slices.Index(obj.keys, key); i >= 0 {
					obj.vals[i] = val
					continue
				}
				obj.keys = append(obj.keys, key)
				obj.vals = append(obj.vals, val)
			}
			if _, err := dec.Token(); err != nil { // '}'
				return nil, err
			}
			return obj, nil
		case '[':
			var arr []any
			for dec.More() {
				val, err := decodeJSONValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, val)
			}
			if _, err := dec.Token(); err != nil { // ']'
				return nil, err
			}
			return arr, nil
		}
		return nil, fmt.Errorf("unexpected %q", t)
	default:
		return t, nil
	}
}
