// Package value defines the runtime values produced by evaluating expressions,
// together with their CSS rendering and arithmetic.
package value

import (
	"fmt"
	"strconv"
	"strings"
)

// Value is an evaluated Stylus value.
type Value interface {
	// CSS returns the value's CSS output form. pretty=false applies minification
	// niceties such as stripping a leading zero from "0.5".
	CSS(pretty bool) string
	// String returns a debug representation (used in error messages).
	String() string
	// TypeName returns the Stylus type name (number, color, string, ...).
	TypeName() string
}

// Number is a numeric value with an optional unit (e.g. 10px, 1.5, 50%).
type Number struct {
	Num  float64
	Unit string
}

// Color is an RGBA color. A is in the range [0,1].
type Color struct {
	R, G, B uint8
	A       float64
}

// Str is a string value. Quote is the original quote rune, or 0 if unquoted.
type Str struct {
	Val   string
	Quote rune
}

// Ident is a bare keyword such as `blue`, `solid`, or `flex`.
type Ident struct{ Name string }

// Bool is a boolean value.
type Bool struct{ Val bool }

// Null is the absence of a value.
type Null struct{}

// List is a space- or comma-separated sequence of values.
type List struct {
	Items []Value
	Comma bool
}

// SlashList is a literal slash join — a property-value `/` whose operands are
// evaluated but not divided, e.g. the 14px/1.5 in `font: 14px/1.5 Arial`.
type SlashList struct{ L, R Value }

func (s *SlashList) TypeName() string { return "literal" }
func (s *SlashList) String() string   { return s.CSS(true) }
func (s *SlashList) CSS(pretty bool) string {
	return s.L.CSS(pretty) + "/" + s.R.CSS(pretty)
}

// --- Number ---

func (n *Number) TypeName() string { return "unit" }
func (n *Number) String() string   { return n.CSS(true) }
func (n *Number) CSS(pretty bool) string {
	s := strconv.FormatFloat(n.Num, 'f', -1, 64)
	if !pretty {
		switch {
		case strings.HasPrefix(s, "0."):
			s = s[1:]
		case strings.HasPrefix(s, "-0."):
			s = "-" + s[2:]
		}
	}
	return s + n.Unit
}

// --- Color ---

func (c *Color) TypeName() string { return "color" }
func (c *Color) String() string   { return c.CSS(true) }
func (c *Color) CSS(pretty bool) string {
	if c.A >= 1 {
		// Use #rgb shorthand when each channel is a doubled nibble.
		if c.R>>4 == c.R&0xf && c.G>>4 == c.G&0xf && c.B>>4 == c.B&0xf {
			return fmt.Sprintf("#%x%x%x", c.R&0xf, c.G&0xf, c.B&0xf)
		}
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	alpha := (&Number{Num: c.A}).CSS(pretty)
	return fmt.Sprintf("rgba(%d,%d,%d,%s)", c.R, c.G, c.B, alpha)
}

// --- Str ---

func (s *Str) TypeName() string { return "string" }
func (s *Str) String() string   { return s.Val }
func (s *Str) CSS(pretty bool) string {
	if s.Quote == 0 {
		return s.Val
	}
	return string(s.Quote) + s.Val + string(s.Quote)
}

// --- Ident ---

func (i *Ident) TypeName() string       { return "ident" }
func (i *Ident) String() string         { return i.Name }
func (i *Ident) CSS(pretty bool) string { return i.Name }

// --- Bool ---

func (b *Bool) TypeName() string       { return "boolean" }
func (b *Bool) String() string         { return strconv.FormatBool(b.Val) }
func (b *Bool) CSS(pretty bool) string { return strconv.FormatBool(b.Val) }

// --- Null ---

func (Null) TypeName() string       { return "null" }
func (Null) String() string         { return "null" }
func (Null) CSS(pretty bool) string { return "" }

// --- List ---

func (l *List) TypeName() string { return "list" }
func (l *List) String() string   { return l.CSS(true) }
func (l *List) CSS(pretty bool) string {
	sep := " "
	if l.Comma {
		if pretty {
			sep = ", "
		} else {
			sep = ","
		}
	}
	parts := make([]string, len(l.Items))
	for i, it := range l.Items {
		parts[i] = it.CSS(pretty)
	}
	return strings.Join(parts, sep)
}
