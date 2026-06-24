// Package ast defines the abstract syntax tree produced by the parser.
//
// The tree has two layers: block-level statements (Stmt) that form the
// indentation structure, and expressions (Expr) that appear on the right-hand
// side of declarations, assignments, and call arguments.
package ast

import "github.com/rohanthewiz/go-styl/internal/token"

// Stmt is a block-level statement node.
type Stmt interface{ stmtNode() }

// Expr is an expression node.
type Expr interface{ exprNode() }

// --- Statements ---

// Stylesheet is the root node: an ordered list of top-level statements.
type Stylesheet struct {
	Statements []Stmt
}

// RuleSet is a selector (or comma-separated group) with a nested body.
type RuleSet struct {
	Selectors []string // raw selector strings, e.g. ["a", "p"] or ["&:hover"]
	Body      []Stmt
}

// Declaration is a CSS property assignment, e.g. `color blue` or `width base * 2`.
type Declaration struct {
	Property string
	Value    Expr
}

// Assignment binds a variable, e.g. `base = 10px` or `x ?= 1`.
type Assignment struct {
	Name  string
	Op    token.Kind // token.ASSIGN or token.ASSIGNQ
	Value Expr
}

// Param is a function/mixin parameter, optionally with a default value or
// marked as a rest parameter (which collects the remaining arguments as a list).
type Param struct {
	Name    string
	Default Expr // nil if no default
	Rest    bool
}

// FuncDef defines a function or mixin. The two share definition syntax; whether
// a callee behaves as a value-returning function or a declaration-emitting mixin
// is decided at the call site by how it is used.
type FuncDef struct {
	Name   string
	Params []Param
	Body   []Stmt
}

// MixinCall invokes a function/mixin in statement position (emitting its body),
// e.g. `clearfix()` or `+button(blue)`.
type MixinCall struct {
	Name string
	Args []Expr
}

// CondBranch is one `if`/`else if` arm: a condition and the body to run if true.
type CondBranch struct {
	Cond Expr
	Body []Stmt
}

// If is an if/else-if/else chain. Else is the final else body (nil if absent).
type If struct {
	Branches []CondBranch
	Else     []Stmt
}

// For is a `for val in expr` or `for key, val in expr` loop.
type For struct {
	Index    string // optional index/key variable name ("" if absent)
	Value    string // value variable name
	Iterable Expr
	Body     []Stmt
}

// Return yields a value from a function body.
type Return struct {
	Value Expr
}

// Extend records an `@extend` (or `@extends`) request: the current rule's
// selectors should be grafted onto every rule matching Target. Target is a raw
// selector string, e.g. ".message" or a "$placeholder" name.
type Extend struct {
	Target string
}

// Import brings in another stylesheet. When Literal is true the import is left as
// a verbatim `@import` in the output (CSS imports, url(), absolute URLs);
// otherwise the referenced .styl file is parsed and inlined, sharing scope.
type Import struct {
	Path    string // the import argument with quotes stripped
	Literal bool   // true => passthrough @import; false => inline a .styl file
}

// AtRule is a block or leaf at-rule (@media, @keyframes, @font-face, @supports,
// @charset, …). Name is the keyword without '@' (e.g. "media"); Params is the raw
// text after it (e.g. "(min-width: 768px)"). Body is nil for leaf at-rules, which
// render as a verbatim passthrough line.
type AtRule struct {
	Name   string
	Params string
	Body   []Stmt
}

func (*Stylesheet) stmtNode()  {}
func (*RuleSet) stmtNode()     {}
func (*Declaration) stmtNode() {}
func (*Assignment) stmtNode()  {}
func (*FuncDef) stmtNode()     {}
func (*MixinCall) stmtNode()   {}
func (*If) stmtNode()          {}
func (*For) stmtNode()         {}
func (*Return) stmtNode()      {}
func (*Extend) stmtNode()      {}
func (*Import) stmtNode()      {}
func (*AtRule) stmtNode()      {}

// --- Expressions ---

// NumberLit is a numeric literal with an optional unit, e.g. "10px" or "1.5".
type NumberLit struct{ Text string }

// ColorLit is a hex color literal including the leading '#'.
type ColorLit struct{ Text string }

// StringLit is a quoted string literal (quotes stripped).
type StringLit struct {
	Value string
	Quote rune
}

// Ident is a bare word: either a variable reference or a CSS keyword (blue, solid).
type Ident struct{ Name string }

// Unary is a prefix operation, e.g. -x or !cond.
type Unary struct {
	Op token.Kind
	X  Expr
}

// Binary is an infix operation, e.g. a * b or a == b.
type Binary struct {
	Op   token.Kind
	L, R Expr
}

// Call is a function/mixin invocation, e.g. rgba(0, 0, 0, 0.5).
type Call struct {
	Name string
	Args []Expr
}

// List is a space- or comma-separated sequence of expressions,
// e.g. `1px solid black` (space) or `a, b` (comma).
type List struct {
	Items []Expr
	Comma bool // true for comma-separated, false for space-separated
}

func (*NumberLit) exprNode() {}
func (*ColorLit) exprNode()  {}
func (*StringLit) exprNode() {}
func (*Ident) exprNode()     {}
func (*Unary) exprNode()     {}
func (*Binary) exprNode()    {}
func (*Call) exprNode()      {}
func (*List) exprNode()      {}
