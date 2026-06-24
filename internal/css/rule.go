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
}

// RawNode is a verbatim line emitted as-is, used for passthrough at-rules such as
// `@import "foo.css";`.
type RawNode struct {
	Text string
}

// Render writes the raw text followed by a newline in pretty mode.
func (r *RawNode) Render(out *strings.Builder, pretty bool) {
	out.WriteString(r.Text)
	if pretty {
		out.WriteByte('\n')
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
// that bare nesting containers don't emit empty `{}` blocks. Unextended
// placeholder rules render nothing.
func (rule *Rule) Render(out *strings.Builder, pretty bool) {
	if len(rule.Statements) == 0 {
		return
	}

	selectors := rule.renderSelectors(pretty)
	if selectors == "" {
		return
	}
	out.WriteString(selectors)

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

// Render renders a block at-rule and its nested nodes. An at-rule whose nested
// nodes produce no output (e.g. all-empty rules) is skipped entirely.
func (a *AtRule) Render(out *strings.Builder, pretty bool) {
	var inner strings.Builder
	for _, n := range a.Nodes {
		n.Render(&inner, pretty)
	}
	body := inner.String()
	if strings.TrimSpace(body) == "" {
		return
	}

	out.WriteString(a.Header)
	if pretty {
		out.WriteByte(' ')
	}
	out.WriteByte('{')
	if pretty {
		out.WriteByte('\n')
		// Indent the already-rendered nested block by one level.
		body = indentBlock(body)
	}
	out.WriteString(body)
	out.WriteByte('}')
	if pretty {
		out.WriteByte('\n')
	}
}

// indentBlock prefixes each non-empty line of an already-rendered block with a tab
// so nested rules sit one level inside their at-rule in pretty output.
func indentBlock(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	var b strings.Builder
	for _, ln := range lines {
		if ln != "" {
			b.WriteByte('\t')
			b.WriteString(ln)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
