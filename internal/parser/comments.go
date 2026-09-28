package parser

import (
	"strings"

	"github.com/rohanthewiz/go-styl/internal/ast"
)

// stripComments removes `//` line comments and `/* ... */` block comments from
// the source while preserving line structure (newlines and leading indentation),
// so that line numbers and indentation are unaffected.
//
// A `//` only starts a comment at the beginning of content or when preceded by
// whitespace; this lets unquoted URLs like http://example.com survive. String
// literals are respected so delimiters inside them are not treated as comments.
func stripComments(src string) string {
	var b strings.Builder
	runes := []rune(src)
	n := len(runes)

	inBlock := false
	var strQuote rune // 0 when not inside a string

	for i := 0; i < n; i++ {
		c := runes[i]

		if inBlock {
			if c == '*' && i+1 < n && runes[i+1] == '/' {
				inBlock = false
				i++
			} else if c == '\n' {
				b.WriteRune(c)
			}
			continue
		}

		if strQuote != 0 {
			b.WriteRune(c)
			if c == '\\' && i+1 < n {
				b.WriteRune(runes[i+1])
				i++
			} else if c == strQuote {
				strQuote = 0
			}
			continue
		}

		switch {
		case c == '"' || c == '\'':
			strQuote = c
			b.WriteRune(c)
		case c == '/' && i+1 < n && runes[i+1] == '*':
			inBlock = true
			i++
		case c == '/' && i+1 < n && runes[i+1] == '/' && (i == 0 || isSpace(runes[i-1]) || runes[i-1] == '\n'):
			// Skip to end of line.
			for i < n && runes[i] != '\n' {
				i++
			}
			if i < n {
				b.WriteRune('\n')
			}
		default:
			b.WriteRune(c)
		}
	}

	return b.String()
}

func isSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\r' }

// StripComments is stripComments for tooling (the language server scans
// code for color literals without matching inside comments).
func StripComments(src string) string { return stripComments(src) }

// srcComment is one comment found by scanComments, positioned in the
// original source.
type srcComment struct {
	text    string // body without the delimiters (`//`, `/*`, `*/`)
	block   bool   // `/* … */` (true) or `// …` (false)
	line    int    // 1-based line the comment starts on
	endLine int    // 1-based line the comment ends on (== line for `//`)
	col     int    // 1-based rune column of the opening delimiter
	indent  int    // indentation width of the start line (tabs expanded)
	inline  bool   // shares a line with code (set by newCommentAttacher)
	gap     bool   // the line after the comment's last line is blank
}

// scanComments returns every comment stripComments would remove, in source
// order. It applies exactly the same rules (a `//` only after whitespace or
// at a line start, string literals skipped, an unterminated `/*` running to
// the end), so the comments it reports and the text stripComments keeps
// partition the source. It is a separate walk rather than a mode of
// stripComments so the compile path, which never wants comments, pays
// nothing for them.
func scanComments(src string) []srcComment {
	runes := []rune(src)
	n := len(runes)
	var out []srcComment

	lineNo := 1
	lineStart := 0 // rune index where the current line begins
	var strQuote rune

	// indentOf measures the leading whitespace of the line starting at s.
	indentOf := func(s int) int {
		w := 0
		for i := s; i < n; i++ {
			switch runes[i] {
			case ' ':
				w++
			case '\t':
				w += tabWidth - (w % tabWidth)
			default:
				return w
			}
		}
		return w
	}

	for i := 0; i < n; i++ {
		c := runes[i]

		if strQuote != 0 {
			if c == '\\' && i+1 < n {
				// An escaped newline still starts a new source line.
				if runes[i+1] == '\n' {
					lineNo++
					lineStart = i + 2
				}
				i++
			} else if c == strQuote {
				strQuote = 0
			} else if c == '\n' {
				lineNo++
				lineStart = i + 1
			}
			continue
		}

		switch {
		case c == '\n':
			lineNo++
			lineStart = i + 1
		case c == '"' || c == '\'':
			strQuote = c
		case c == '/' && i+1 < n && runes[i+1] == '*':
			sc := srcComment{block: true, line: lineNo, col: i - lineStart + 1, indent: indentOf(lineStart)}
			j := i + 2
			for j < n && !(runes[j] == '*' && j+1 < n && runes[j+1] == '/') {
				if runes[j] == '\n' {
					lineNo++
					lineStart = j + 1
				}
				j++
			}
			sc.text = string(runes[i+2 : j])
			sc.endLine = lineNo
			sc.gap = blankLineAfter(runes, j)
			out = append(out, sc)
			i = j + 1 // past "*/" (or past the end when unterminated)
		case c == '/' && i+1 < n && runes[i+1] == '/' && (i == 0 || isSpace(runes[i-1]) || runes[i-1] == '\n'):
			j := i + 2
			for j < n && runes[j] != '\n' {
				j++
			}
			out = append(out, srcComment{
				text: strings.TrimRight(string(runes[i+2:j]), " \t\r"), line: lineNo, endLine: lineNo,
				col: i - lineStart + 1, indent: indentOf(lineStart), gap: blankLineAfter(runes, j),
			})
			i = j - 1 // the '\n' is handled by the next iteration
		}
	}
	return out
}

// blankLineAfter reports whether the line after the one holding runes[pos]
// exists and is empty or whitespace only.
func blankLineAfter(runes []rune, pos int) bool {
	i := pos
	for i < len(runes) && runes[i] != '\n' {
		i++
	}
	if i >= len(runes) {
		return false // last line: nothing follows
	}
	for i++; i < len(runes) && runes[i] != '\n'; i++ {
		if !isSpace(runes[i]) {
			return false
		}
	}
	return true
}

// commentAttacher hands the comments from scanComments to the lines of the
// indentation tree as buildTree creates them. Each comment has a key line.
// An inline comment (code on its start line, or after its `*/`) is keyed to
// that code line and leads it: `color red // why` puts the comment above
// `color: red`. An own-line comment is keyed to its start line and waits
// for the next code line, whose indentation decides where it goes:
//
//	.a
//	  // x          x leads `color red` (it is no deeper than that line)
//	  color red
//	  // y          y is deeper than `.b`, so it closes .a's block: it
//	.b              trails .a's last child, `color red`
//
// Keys never decrease through the source, so one cursor walks the list.
// Every comment is placed somewhere: whatever is left at the end trails
// the last line of the block it is indented into, or the root.
type commentAttacher struct {
	comments []srcComment
	keys     []int  // key line of comments[i]
	inline   []bool // comments[i] shares its key line with code
	next     int
}

// newCommentAttacher classifies comments against cleaned, the comment-free
// source buildTree splits into lines.
func newCommentAttacher(comments []srcComment, cleaned string) *commentAttacher {
	a := &commentAttacher{comments: comments}
	if len(comments) == 0 {
		return a
	}
	lines := strings.Split(cleaned, "\n")
	hasCode := func(n int) bool {
		return n >= 1 && n <= len(lines) && strings.TrimSpace(lines[n-1]) != ""
	}
	for i, c := range comments {
		switch {
		case hasCode(c.line):
			a.keys, a.inline = append(a.keys, c.line), append(a.inline, true)
		case hasCode(c.endLine):
			a.keys, a.inline = append(a.keys, c.endLine), append(a.inline, true)
		default:
			a.keys, a.inline = append(a.keys, c.line), append(a.inline, false)
		}
		comments[i].inline = a.inline[i]
	}
	return a
}

// place attaches every comment keyed at or before ln's line, before ln is
// linked into the tree. stack is ln's would-be ancestry before popping
// (stack[0] is the synthetic root).
func (a *commentAttacher) place(ln *line, stack []*line) {
	for ; a.next < len(a.comments) && a.keys[a.next] <= ln.lineNo; a.next++ {
		c := a.comments[a.next]
		if (a.inline[a.next] && a.keys[a.next] == ln.lineNo) || c.indent <= ln.indent {
			ln.lead = append(ln.lead, c)
			continue
		}
		if !trailInto(c, stack) {
			ln.lead = append(ln.lead, c)
		}
	}
}

// continued attaches comments keyed up to lineNo, a line folded into cont
// by a trailing comma, to the continued statement.
func (a *commentAttacher) continued(cont *line, lineNo int) {
	for ; a.next < len(a.comments) && a.keys[a.next] <= lineNo; a.next++ {
		cont.lead = append(cont.lead, a.comments[a.next])
	}
}

// finish places the comments after the last code line.
func (a *commentAttacher) finish(root *line, stack []*line) {
	for ; a.next < len(a.comments); a.next++ {
		c := a.comments[a.next]
		if !trailInto(c, stack) {
			root.trail = append(root.trail, c)
		}
	}
}

// trailInto appends c to the trail of the last child of the deepest open
// block c is indented into (a line shallower than c that already has
// children; a leaf can't adopt a comment, which would make it a block). It
// reports false when there is no such block (nothing parsed yet).
func trailInto(c srcComment, stack []*line) bool {
	for k := len(stack) - 1; k >= 0; k-- {
		e := stack[k]
		if e.indent < c.indent && len(e.children) > 0 {
			last := e.children[len(e.children)-1]
			last.trail = append(last.trail, c)
			return true
		}
	}
	return false
}

// commentStmts converts attached comments into statements.
func commentStmts(cs []srcComment) []ast.Stmt {
	if len(cs) == 0 {
		return nil
	}
	out := make([]ast.Stmt, len(cs))
	for i, c := range cs {
		out[i] = &ast.Comment{Text: c.text, Block: c.block, Line: c.line, Col: c.col, EndLine: c.endLine, Inline: c.inline, BlankAfter: c.gap}
	}
	return out
}
