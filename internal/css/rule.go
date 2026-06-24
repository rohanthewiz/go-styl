// Package css is the output backend of the compiler. It holds the resolved CSS
// rule tree (selectors already flattened by the evaluator) and renders it to a
// CSS string, optionally merging duplicate rule bodies for extra compression.
package css

import "strings"

// Node is anything that can render itself into the output stream.
type Node interface {
	Render(out *strings.Builder, pretty bool)
}

// Statement is a single resolved CSS declaration, e.g. {Property:"color", Value:"#000"}.
type Statement struct {
	Property string
	Value    string
}

// Rule is a resolved CSS rule: a fully-qualified selector plus its declarations.
// Duplicates holds other rules that share an identical declaration body; they are
// rendered as a comma-separated selector group (see dedupe.go).
type Rule struct {
	Selector   string
	Statements []*Statement
	Duplicates []*Rule
}

// AtRule is a block at-rule such as @media or @supports whose body is a set of
// nested rules (e.g. "@media (min-width: 900px) { ... }").
type AtRule struct {
	Header string // e.g. "@media all and (min-width: 900px)"
	Rules  []*Rule
}

// Stylesheet is the top-level ordered collection of output nodes.
type Stylesheet struct {
	Nodes []Node
}

// Render renders the whole stylesheet, trimming a trailing blank line in pretty mode.
func (s *Stylesheet) Render(pretty bool) string {
	out := strings.Builder{}
	for _, n := range s.Nodes {
		n.Render(&out, pretty)
	}
	return strings.TrimRight(out.String(), "\n")
}

// Render renders a single CSS rule. Rules with no declarations are skipped so
// that bare nesting containers don't emit empty `{}` blocks.
func (rule *Rule) Render(out *strings.Builder, pretty bool) {
	if len(rule.Statements) == 0 {
		return
	}

	out.WriteString(rule.Selector)

	for _, dup := range rule.Duplicates {
		out.WriteByte(',')
		if pretty {
			out.WriteByte(' ')
		}
		out.WriteString(dup.Selector)
	}

	if pretty {
		out.WriteByte(' ')
	}
	out.WriteByte('{')
	if pretty {
		out.WriteByte('\n')
	}

	for i, st := range rule.Statements {
		if pretty {
			out.WriteByte('\t')
		}
		out.WriteString(st.Property)
		out.WriteByte(':')
		if pretty {
			out.WriteByte(' ')
		}
		out.WriteString(st.Value)

		// Compressed output omits the final semicolon.
		if pretty || i != len(rule.Statements)-1 {
			out.WriteByte(';')
		}
		if pretty {
			out.WriteByte('\n')
		}
	}

	out.WriteByte('}')
	if pretty {
		out.WriteString("\n\n")
	}
}

// Render renders a block at-rule and its nested rules.
func (a *AtRule) Render(out *strings.Builder, pretty bool) {
	out.WriteString(a.Header)
	if pretty {
		out.WriteByte(' ')
	}
	out.WriteByte('{')
	if pretty {
		out.WriteByte('\n')
	}
	for _, r := range a.Rules {
		r.Render(out, pretty)
	}
	out.WriteByte('}')
	if pretty {
		out.WriteByte('\n')
	}
}
