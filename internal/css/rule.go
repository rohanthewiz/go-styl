// Package css is the output backend of the compiler. It holds the resolved CSS
// rule tree (selectors already flattened by the evaluator) and renders it to a
// CSS string, optionally merging duplicate rule bodies for extra compression.
package css

import "strings"

// Node is anything that can render itself into the output stream. hasOutput
// reports whether it would emit anything (used to skip empty rules/at-rules and to
// place blank-line separators).
type Node interface {
	Render(p *Printer)
	hasOutput() bool
}

// Statement is a single resolved CSS declaration, e.g. {Property:"color", Value:"#000"}.
type Statement struct {
	Property  string
	Value     string
	Important bool
	Pos       Pos
}

// Rule is a resolved CSS rule: a fully-qualified selector plus its declarations.
// Duplicates holds other rules that share an identical declaration body; they are
// rendered as a comma-separated selector group (see dedupe.go).
//
// Selectors carries the rule's selectors as a slice (the same data as Selector,
// pre-join) so @extend can match against individual selectors. Placeholder marks
// a `$name` rule whose own selector is never emitted — only the Extenders grafted
// onto it via @extend. Extenders are selectors added by @extend.
type Rule struct {
	Selector    string
	Selectors   []string
	Statements  []*Statement
	Duplicates  []*Rule
	Placeholder bool
	Extenders   []string
	Pos         Pos
}

// RawNode is a verbatim line emitted as-is, used for passthrough at-rules such as
// `@import "foo.css";`.
type RawNode struct {
	Text string
}

func (r *RawNode) hasOutput() bool { return strings.TrimSpace(r.Text) != "" }

// Render writes the raw text followed by a newline in pretty mode.
func (r *RawNode) Render(p *Printer) {
	p.write(r.Text)
	if p.pretty {
		p.write("\n")
	}
}

// renderSelectors returns the comma-joined selector group for a rule, accounting
// for placeholders (own selector suppressed) and @extend additions. It returns ""
// when nothing should render (an unextended placeholder).
func (rule *Rule) renderSelectors(pretty bool) string {
	sels := make([]string, 0, 1+len(rule.Duplicates)+len(rule.Extenders))
	if !rule.Placeholder {
		sels = append(sels, rule.Selector)
	}
	for _, dup := range rule.Duplicates {
		sels = append(sels, dup.Selector)
	}
	sels = append(sels, rule.Extenders...)
	if len(sels) == 0 {
		return ""
	}
	sep := ","
	if pretty {
		sep = ", "
	}
	return strings.Join(sels, sep)
}

// AtRule is a block at-rule such as @media, @supports, or @keyframes whose body is
// a set of nested nodes (e.g. "@media (min-width: 900px) { ... }").
type AtRule struct {
	Header string // e.g. "@media all and (min-width: 900px)"
	Nodes  []Node // nested rules (and possibly nested at-rules)
	Pos    Pos
}

// Stylesheet is the top-level ordered collection of output nodes.
type Stylesheet struct {
	Nodes []Node
}

// Render renders the whole stylesheet, trimming a trailing blank line in pretty mode.
func (s *Stylesheet) Render(pretty bool) string {
	return RenderSheet(s.Nodes, pretty, nil)
}

// hasOutput reports whether the rule emits anything (it is skipped when it has no
// declarations or is an unextended placeholder).
func (rule *Rule) hasOutput() bool {
	return len(rule.Statements) > 0 && rule.renderSelectors(false) != ""
}

// Render renders a single CSS rule at the printer's current depth.
func (rule *Rule) Render(p *Printer) {
	if !rule.hasOutput() {
		return
	}

	p.tabs(p.depth)
	p.mark(rule.Pos)
	p.write(rule.renderSelectors(p.pretty))
	if p.pretty {
		p.write(" ")
	}
	p.write("{")
	if p.pretty {
		p.write("\n")
	}

	for i, st := range rule.Statements {
		p.tabs(p.depth + 1)
		p.mark(st.Pos)
		p.write(st.Property)
		p.write(":")
		if p.pretty {
			p.write(" ")
		}
		p.write(st.Value)
		if st.Important {
			if p.pretty {
				p.write(" !important")
			} else {
				p.write("!important")
			}
		}
		// Compressed output omits the final semicolon.
		if p.pretty || i != len(rule.Statements)-1 {
			p.write(";")
		}
		if p.pretty {
			p.write("\n")
		}
	}

	p.tabs(p.depth)
	p.write("}")
	if p.pretty {
		p.write("\n")
	}
}

// hasOutput reports whether any nested node would render output.
func (a *AtRule) hasOutput() bool {
	for _, n := range a.Nodes {
		if n.hasOutput() {
			return true
		}
	}
	return false
}

// Render renders a block at-rule and its nested nodes (indented one level deeper).
// An at-rule whose body produces no output is skipped entirely.
func (a *AtRule) Render(p *Printer) {
	if !a.hasOutput() {
		return
	}

	p.tabs(p.depth)
	p.mark(a.Pos)
	p.write(a.Header)
	if p.pretty {
		p.write(" ")
	}
	p.write("{")
	if p.pretty {
		p.write("\n")
	}

	p.depth++
	renderNodes(p, a.Nodes)
	p.depth--

	p.tabs(p.depth)
	p.write("}")
	if p.pretty {
		p.write("\n")
	}
}
