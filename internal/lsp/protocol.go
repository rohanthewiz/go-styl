package lsp

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// The subset of LSP 3.17 types the server speaks. Field names follow the
// specification so the JSON tags are the spec's names.

type Position struct {
	Line      int `json:"line"`      // 0-based
	Character int `json:"character"` // 0-based, in UTF-16 code units
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type TextDocumentItem struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
	Text    string `json:"text"`
}

type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type DidOpenParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type DidChangeParams struct {
	TextDocument struct {
		URI     string `json:"uri"`
		Version int    `json:"version"`
	} `json:"textDocument"`
	// With full sync (the only kind the server advertises) each change
	// carries the whole document in Text.
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
}

type DidCloseParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type DocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// Diagnostic severities.
const (
	SeverityError   = 1
	SeverityWarning = 2
	SeverityHint    = 4
)

// tagUnnecessary (DiagnosticTag) asks the client to fade the range, as for
// unused code.
const tagUnnecessary = 1

type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Source   string `json:"source"`
	Message  string `json:"message"`
	Tags     []int  `json:"tags,omitempty"`
}

type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Version     int          `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type MarkupContent struct {
	Kind  string `json:"kind"` // "markdown"
	Value string `json:"value"`
}

type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// Completion item kinds (CompletionItemKind).
const (
	kindFunction = 3
	kindProperty = 10
	kindVariable = 6
	kindKeyword  = 14
	kindColor    = 16
)

type CompletionItem struct {
	Label  string `json:"label"`
	Kind   int    `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// Symbol kinds (SymbolKind).
const (
	symNamespace = 3
	symClass     = 5
	symFunction  = 12
	symVariable  = 13
	symProperty  = 7
)

type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

type ParameterInformation struct {
	Label string `json:"label"` // a substring of the signature's label
}

type SignatureInformation struct {
	Label      string                 `json:"label"`
	Parameters []ParameterInformation `json:"parameters"`
}

type SignatureHelp struct {
	Signatures      []SignatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature"`
	ActiveParameter int                    `json:"activeParameter"`
}

type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes"`
}

type ReferenceParams struct {
	TextDocumentPositionParams
	Context struct {
		IncludeDeclaration bool `json:"includeDeclaration"`
	} `json:"context"`
}

type RenameParams struct {
	TextDocumentPositionParams
	NewName string `json:"newName"`
}

type Color struct {
	Red   float64 `json:"red"`
	Green float64 `json:"green"`
	Blue  float64 `json:"blue"`
	Alpha float64 `json:"alpha"`
}

type ColorInformation struct {
	Range Range `json:"range"`
	Color Color `json:"color"`
}

type ColorPresentationParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Color        Color                  `json:"color"`
	Range        Range                  `json:"range"`
}

type ColorPresentation struct {
	Label string `json:"label"`
}

// --- URIs ---

// uriToPath converts a file:// URI to an OS path. Non-file URIs (untitled
// buffers) return "", and the server then analyzes them without a location:
// no relative imports, no filename in diagnostics.
func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return ""
	}
	p := u.Path
	if runtime.GOOS == "windows" {
		// file:///C:/x → /C:/x; drop the slash before the drive letter.
		p = strings.TrimPrefix(p, "/")
	}
	return filepath.FromSlash(p)
}

// pathToURI converts an absolute OS path to a file:// URI.
func pathToURI(p string) string {
	p = filepath.ToSlash(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // Windows drive paths
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

// --- Columns ---
//
// LSP counts columns in UTF-16 code units; the server works in rune
// indexes into a line. These convert between the two for one line's text.

// utf16Col returns the UTF-16 column of rune index r in line.
func utf16Col(line string, r int) int {
	col := 0
	for i, c := range []rune(line) {
		if i >= r {
			break
		}
		col += len(utf16.Encode([]rune{c}))
	}
	return col
}

// runeCol returns the rune index at UTF-16 column u in line (clamped to the
// line's length).
func runeCol(line string, u int) int {
	col, n := 0, 0
	for _, c := range line {
		if col >= u {
			break
		}
		if c >= 0x10000 {
			col += 2
		} else {
			col++
		}
		n++
	}
	return min(n, utf8.RuneCountInString(line))
}
