package eval

import (
	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/value"
)

// Closure is a function/mixin definition paired with the scope it was defined in
// (so it can resolve free variables lexically when invoked). File records the
// source file of the definition, for positioning errors raised in the body.
type Closure struct {
	Def   *ast.FuncDef
	Scope *Scope
	File  string
}

// Scope is a lexical scope in the chain. It holds variables and function/mixin
// definitions. Lookups walk outward to the parent; bindings land in the current
// scope.
type Scope struct {
	vars   map[string]value.Value
	funcs  map[string]*Closure
	parent *Scope
}

// NewScope creates a root scope.
func NewScope() *Scope {
	return &Scope{vars: map[string]value.Value{}, funcs: map[string]*Closure{}}
}

// Child creates a nested scope.
func (s *Scope) Child() *Scope {
	return &Scope{vars: map[string]value.Value{}, funcs: map[string]*Closure{}, parent: s}
}

// Get resolves a variable by walking outward through enclosing scopes.
func (s *Scope) Get(name string) (value.Value, bool) {
	for sc := s; sc != nil; sc = sc.parent {
		if v, ok := sc.vars[name]; ok {
			return v, true
		}
	}
	return nil, false
}

// Set binds a variable in the current scope.
func (s *Scope) Set(name string, v value.Value) {
	s.vars[name] = v
}

// Has reports whether a variable is defined anywhere in the chain.
func (s *Scope) Has(name string) bool {
	_, ok := s.Get(name)
	return ok
}

// GetFunc resolves a function/mixin by walking outward through enclosing scopes.
func (s *Scope) GetFunc(name string) (*Closure, bool) {
	for sc := s; sc != nil; sc = sc.parent {
		if f, ok := sc.funcs[name]; ok {
			return f, true
		}
	}
	return nil, false
}

// SetFunc binds a function/mixin in the current scope.
func (s *Scope) SetFunc(name string, c *Closure) {
	s.funcs[name] = c
}

// FuncNames returns every function/mixin name visible from this scope.
func (s *Scope) FuncNames() []string {
	var out []string
	for sc := s; sc != nil; sc = sc.parent {
		for name := range sc.funcs {
			out = append(out, name)
		}
	}
	return out
}
