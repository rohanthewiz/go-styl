package lsp

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/parser"
)

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// --- document symbols ---

// symbols is the document outline: selectors and at-rules as containers,
// mixins/functions and variables as leaves. Each symbol's range runs from
// its line to the last line of its block (see lastLine).
func (s *Server) symbols(d *document) []DocumentSymbol {
	an := d.index()
	if an == nil || an.sheet == nil {
		return []DocumentSymbol{}
	}
	out := outline(an.sheet.Statements, d.lines)
	if out == nil {
		out = []DocumentSymbol{}
	}
	return out
}

func outline(stmts []ast.Stmt, lines []string) []DocumentSymbol {
	var out []DocumentSymbol
	sym := func(name, detail string, kind, line int, body []ast.Stmt, children []DocumentSymbol) DocumentSymbol {
		i := max(line-1, 0)
		end := max(line, lastLine(body)) - 1
		r := lineRange(lines, i)
		r.End = lineRange(lines, end).End
		return DocumentSymbol{
			Name: name, Detail: detail, Kind: kind,
			Range: r, SelectionRange: lineRange(lines, i),
			Children: children,
		}
	}
	for _, st := range stmts {
		switch n := st.(type) {
		case *ast.RuleSet:
			out = append(out, sym(strings.Join(n.Selectors, ", "), "", symClass, n.Line, n.Body, outline(n.Body, lines)))
		case *ast.AtRule:
			if n.Body != nil {
				out = append(out, sym("@"+n.Name+" "+n.Params, "", symNamespace, n.Line, n.Body, outline(n.Body, lines)))
			}
		case *ast.FuncDef:
			params := make([]string, len(n.Params))
			for i, p := range n.Params {
				params[i] = p.Name
				if p.Rest {
					params[i] += "..."
				}
			}
			out = append(out, sym(n.Name, "("+strings.Join(params, ", ")+")", symFunction, n.Line, n.Body, nil))
		case *ast.Assignment:
			out = append(out, sym(n.Name, "", symVariable, n.Line, nil, nil))
		case *ast.If:
			// Conditionals aren't outline entries, but what they define is.
			for _, b := range n.Branches {
				out = append(out, outline(b.Body, lines)...)
			}
			out = append(out, outline(n.Else, lines)...)
		case *ast.For:
			out = append(out, outline(n.Body, lines)...)
		}
	}
	return out
}

// --- colors ---

// hexColor matches a 3/4/6/8-digit hex color not glued to a longer word.
var hexColor = regexp.MustCompile(`#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3,4})\b`)

// documentColors finds hex color literals for the editor's swatches.
//
// Only lines where a statement with a value starts are scanned
// (declarations, assignments, calls, returns), so an ID selector that
// happens to be valid hex (`#bad`, `#fade`) isn't mistaken for a color;
// comments and strings are masked out first. A match directly followed by
// `{` is a brace-syntax selector and skipped for the same reason.
func documentColors(d *document) []ColorInformation {
	out := []ColorInformation{}
	an := d.index()
	if an == nil || an.sheet == nil {
		return out
	}
	valueLines := map[int]bool{}
	var mark func(stmts []ast.Stmt)
	mark = func(stmts []ast.Stmt) {
		for _, st := range stmts {
			switch st.(type) {
			case *ast.Declaration, *ast.Assignment, *ast.MemberAssign, *ast.MixinCall, *ast.Return, *ast.ExprStmt:
				l, _ := ast.Pos(st)
				valueLines[l-1] = true
			}
			for _, b := range bodies(st) {
				mark(b)
			}
		}
	}
	mark(an.sheet.Statements)

	clean := strings.Split(maskStrings(parser.StripComments(d.text)), "\n")
	for i, text := range clean {
		if !valueLines[i] || i >= len(d.lines) {
			continue
		}
		for _, loc := range hexColor.FindAllStringIndex(text, -1) {
			if strings.HasPrefix(strings.TrimLeft(text[loc[1]:], " \t"), "{") {
				continue
			}
			c, ok := parseHex(text[loc[0]+1 : loc[1]])
			if !ok {
				continue
			}
			line := d.lines[i]
			// Byte offsets → rune offsets → UTF-16 columns. Masking kept
			// every byte in place, so offsets into clean are offsets into
			// the real line.
			rs := len([]rune(line[:min(loc[0], len(line))]))
			re := len([]rune(line[:min(loc[1], len(line))]))
			out = append(out, ColorInformation{
				Range: Range{
					Start: Position{Line: i, Character: utf16Col(line, rs)},
					End:   Position{Line: i, Character: utf16Col(line, re)},
				},
				Color: c,
			})
		}
	}
	return out
}

// maskStrings replaces the contents of quoted strings with spaces, keeping
// every byte offset (non-ASCII bytes become spaces one for one, which is
// fine: the result is only scanned, never shown).
func maskStrings(s string) string {
	b := []byte(s)
	var quote byte
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case quote != 0:
			if c == '\n' {
				quote = 0
			} else if c == '\\' && i+1 < len(b) && b[i+1] != '\n' {
				b[i], b[i+1] = ' ', ' '
				i++
			} else if c == quote {
				quote = 0
			} else {
				b[i] = ' '
			}
		case c == '"' || c == '\'':
			quote = c
		}
	}
	return string(b)
}

// parseHex decodes 3/4/6/8 hex digits (no '#') into an LSP color.
func parseHex(h string) (Color, bool) {
	if len(h) == 3 || len(h) == 4 {
		var sb strings.Builder
		for _, c := range h {
			sb.WriteRune(c)
			sb.WriteRune(c)
		}
		h = sb.String()
	}
	if len(h) == 6 {
		h += "ff"
	}
	if len(h) != 8 {
		return Color{}, false
	}
	var ch [4]float64
	for i := range ch {
		v, err := strconv.ParseUint(h[i*2:i*2+2], 16, 8)
		if err != nil {
			return Color{}, false
		}
		ch[i] = float64(v) / 255
	}
	return Color{Red: ch[0], Green: ch[1], Blue: ch[2], Alpha: ch[3]}, true
}

// colorPresentations offers the edited color back as text: hex first (what
// the picker writes into the file), then rgba().
func colorPresentations(c Color) []ColorPresentation {
	to255 := func(f float64) int { return int(math.Round(math.Max(0, math.Min(1, f)) * 255)) }
	r, g, b, a := to255(c.Red), to255(c.Green), to255(c.Blue), to255(c.Alpha)
	hex := fmt.Sprintf("#%02x%02x%02x", r, g, b)
	if a < 255 {
		hex += fmt.Sprintf("%02x", a)
	}
	alpha := strconv.FormatFloat(math.Round(c.Alpha*100)/100, 'f', -1, 64)
	return []ColorPresentation{
		{Label: hex},
		{Label: fmt.Sprintf("rgba(%d, %d, %d, %s)", r, g, b, alpha)},
	}
}
