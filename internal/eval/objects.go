package eval

import (
	"fmt"

	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/token"
	"github.com/rohanthewiz/go-styl/internal/value"
)

// Objects (Stylus hashes) — evaluation of literals, member reads, member
// assignment, `in`, and `for key, val in obj`. The value type and its
// by-reference semantics are described on value.Hash.

// evalObject builds a new object from a literal. Each evaluation makes a
// fresh Hash, so a literal inside a function body gives every call its own
// object.
func (ev *evaluator) evalObject(o *ast.Object, scope *Scope) (value.Value, error) {
	h := value.NewHash()
	for _, p := range o.Pairs {
		v, err := ev.evalExpr(p.Value, scope)
		if err != nil {
			return nil, err
		}
		h.Set(p.Key, v)
	}
	return h, nil
}

// evalMember reads obj.key. A missing key is null, as in Stylus. Reading a
// key of anything but an object is an error: in go-styl `a.b` only exists as
// member access (it used to be a lex error), so there is no CSS reading to
// fall back to.
func (ev *evaluator) evalMember(m *ast.Member, scope *Scope) (value.Value, error) {
	x, err := ev.evalExpr(m.X, scope)
	if err != nil {
		return nil, err
	}
	h, ok := value.Deref(x).(*value.Hash)
	if !ok {
		return nil, fmt.Errorf("cannot read .%s of %s %q (not an object)", m.Name, x.TypeName(), x.CSS(true))
	}
	if v, found := h.Get(m.Name); found {
		return v, nil
	}
	return value.Null{}, nil
}

// evalMemberAssign stores into an object slot: the target's container is
// evaluated to the Hash it names and the key is set in place, so every
// variable sharing that object sees the change.
//
//	theme.colors.bg = #fff
//	└──── container ┘ └ key
//
// Subscript assignment into a list (`r[1] = x`) is not supported; lists are
// immutable values in go-styl.
func (ev *evaluator) evalMemberAssign(s *ast.MemberAssign, scope *Scope) error {
	var containerExpr ast.Expr
	var key string
	switch t := s.Target.(type) {
	case *ast.Member:
		containerExpr, key = t.X, t.Name
	case *ast.Index:
		k, err := ev.evalExpr(t.Index, scope)
		if err != nil {
			return err
		}
		containerExpr, key = t.X, value.KeyString(k)
	default:
		return fmt.Errorf("invalid assignment target")
	}
	cv, err := ev.evalExpr(containerExpr, scope)
	if err != nil {
		return err
	}
	h, ok := value.Deref(cv).(*value.Hash)
	if !ok {
		return fmt.Errorf("cannot set key %q on %s %q: only object keys can be assigned", key, cv.TypeName(), cv.CSS(true))
	}
	if s.Op == token.ASSIGNQ && h.Has(key) {
		return nil // ?= only stores when the key is absent
	}
	v, err := ev.evalExpr(s.Value, scope)
	if err != nil {
		return err
	}
	h.Set(key, v)
	return nil
}

// evalForHash runs `for key in obj` / `for key, val in obj`: the first
// variable gets each key (as a quoted string, as keys() returns them) and the
// optional second one its value, in insertion order. (Over a list the second
// variable is the index instead.)
func (ev *evaluator) evalForHash(s *ast.For, h *value.Hash, ctx *execCtx) error {
	// Iterate a snapshot of the keys, so a body that adds keys doesn't
	// extend the loop.
	keys := append([]string(nil), h.Keys()...)
	for _, k := range keys {
		ctx.scope.Set(s.Value, &value.Str{Val: k, Quote: '\''})
		if s.Index != "" {
			v, _ := h.Get(k)
			ctx.scope.Set(s.Index, v)
		}
		if err := ev.execBlock(s.Body, ctx); err != nil {
			return err
		}
		if ctx.returned {
			break
		}
	}
	return nil
}

// inAsText handles an ast.Binary.InText `in`: when the right operand is a
// bare word — an identifier that names no variable — the expression is the
// CSS text `L in R` (a space list), as in `to right in oklch`. ok is false
// when the test should run normally.
func (ev *evaluator) inAsText(b *ast.Binary, scope *Scope) (value.Value, bool, error) {
	id, isIdent := b.R.(*ast.Ident)
	if !isIdent {
		return nil, false, nil
	}
	switch id.Name {
	case "true", "false", "null":
		return nil, false, nil
	}
	if _, isVar := scope.Get(id.Name); isVar {
		return nil, false, nil
	}
	r, err := ev.evalExpr(b.R, scope)
	if err != nil {
		return nil, true, err
	}
	l, err := ev.evalExpr(b.L, scope)
	if err != nil {
		return nil, true, err
	}
	return &value.List{Items: []value.Value{l, &value.Ident{Name: "in"}, r}}, true, nil
}

// contains implements `needle in haystack`: key presence for an object,
// membership for a list, and equality for a single value (a lone value is a
// one-item list). Items compare like `==`, by CSS form.
func contains(haystack, needle value.Value) bool {
	haystack, needle = value.Deref(haystack), value.Deref(needle)
	switch h := haystack.(type) {
	case *value.Hash:
		return h.Has(value.KeyString(needle))
	case value.Null:
		return false
	}
	want := needle.CSS(true)
	for _, it := range iterItems(haystack) {
		if value.Deref(it).CSS(true) == want {
			return true
		}
	}
	return false
}
