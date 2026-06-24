package css

import "strings"

// Pos is a 1-based source position. A zero Line means "unknown" (no mapping).
type Pos struct{ Line, Col int }

// Printer accumulates rendered CSS while tracking the current output position, so
// it can emit source-map segments as it goes. When sm is nil it renders only.
type Printer struct {
	sb     strings.Builder
	pretty bool
	depth  int        // current indentation level (pretty mode)
	line   int        // 0-based current output line
	col    int        // 0-based current output column
	sm     *SourceMap // nil unless a source map is being collected
}

func newPrinter(pretty bool, sm *SourceMap) *Printer {
	return &Printer{pretty: pretty, sm: sm}
}

// write appends s and advances the tracked output position.
func (p *Printer) write(s string) {
	p.sb.WriteString(s)
	for _, r := range s {
		if r == '\n' {
			p.line++
			p.col = 0
		} else {
			p.col++
		}
	}
}

// tabs writes depth indentation in pretty mode.
func (p *Printer) tabs(depth int) {
	if p.pretty && depth > 0 {
		p.write(strings.Repeat("\t", depth))
	}
}

// mark records a source-map segment from the current output position back to the
// given source position (if mapping is enabled and the position is known).
func (p *Printer) mark(pos Pos) {
	if p.sm != nil && pos.Line > 0 {
		p.sm.add(p.line, p.col, pos.Line-1, pos.Col-1)
	}
}

// renderNodes renders a list of block nodes, inserting a blank line between
// consecutive non-empty blocks in pretty mode.
func renderNodes(p *Printer, nodes []Node) {
	first := true
	for _, n := range nodes {
		if !n.hasOutput() {
			continue
		}
		if p.pretty && !first {
			p.write("\n")
		}
		n.Render(p)
		first = false
	}
}

// RenderSheet renders a stylesheet, optionally collecting source-map segments into
// sm. It returns the CSS with any trailing blank line trimmed.
func RenderSheet(nodes []Node, pretty bool, sm *SourceMap) string {
	p := newPrinter(pretty, sm)
	renderNodes(p, nodes)
	return strings.TrimRight(p.sb.String(), "\n")
}
