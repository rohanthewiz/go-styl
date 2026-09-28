package styl

import (
	"sort"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/css"
	"github.com/rohanthewiz/go-styl/internal/eval"
	"github.com/rohanthewiz/go-styl/internal/parser"
)

// Used is the set of names present in a rendered document, driving Prune.
// Collect one with UsedFromHTML, or build it directly when the renderer
// already knows which names it emitted.
//
// A nil slice means "unknown" — selectors are never pruned on that axis —
// while a non-nil empty slice means "known empty". UsedFromHTML always
// returns non-nil slices. Leave Tags nil to keep every type selector (the
// safe choice when the HTML fragment is not the whole page).
type Used struct {
	Classes []string
	IDs     []string
	Tags    []string // lower-case element names
}

// UsedFromHTML scans rendered HTML and collects the tag names, classes, and
// element IDs it uses, for feeding Prune. It is a lightweight scanner, not a
// full HTML parser: markup-like text inside <script> bodies may contribute
// extra names, which only ever keeps more CSS than strictly needed.
func UsedFromHTML(html string) Used {
	classes, ids, tags := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for i := 0; i < len(html); {
		if html[i] != '<' {
			i++
			continue
		}
		if strings.HasPrefix(html[i:], "<!--") {
			if end := strings.Index(html[i+4:], "-->"); end >= 0 {
				i += 4 + end + 3
			} else {
				i = len(html)
			}
			continue
		}
		if i+1 < len(html) && (html[i+1] == '/' || html[i+1] == '!' || html[i+1] == '?') {
			i = skipTo(html, i, '>')
			continue
		}
		name, j := readToken(html, i+1)
		if name == "" {
			i++
			continue
		}
		tags[strings.ToLower(name)] = true
		i = scanAttrs(html, j, classes, ids)
	}
	return Used{Classes: sortedNames(classes), IDs: sortedNames(ids), Tags: sortedNames(tags)}
}

// Prune compiles Stylus source like Compile, then keeps only the rules whose
// selectors can match a document restricted to the used names — the critical
// CSS for that document. A selector survives when every class, ID, and tag it
// requires is present in Used (names inside functional pseudo-class arguments
// such as ":not(.x)" are never required); selectors that require nothing
// ("@font-face", ":root", "*", attribute-only) always survive. @keyframes
// blocks are kept only while a surviving animation/animation-name declaration
// references them, and at-rules left empty are dropped.
//
// Compiles cost microseconds, so pruning per response and caching by used-set
// is practical: render the page, then
//
//	css, err := styl.Prune(src, styl.UsedFromHTML(page), opts)
func Prune(src string, used Used, opts Options) (string, error) {
	sb, err := opts.sandbox(len(src))
	if err != nil {
		return "", compileErr(err, opts.Filename)
	}
	sheet, err := parser.Parse(src)
	if err != nil {
		return "", compileErr(err, opts.Filename)
	}
	out, err := eval.EvaluatePruned(sheet, eval.Options{
		Pretty:           opts.Pretty,
		MergeDuplicates:  opts.MergeDuplicates,
		Filename:         opts.Filename,
		BaseDir:          opts.baseDir(),
		IncludePaths:     opts.IncludePaths,
		FS:               opts.FS,
		Globals:          opts.Globals,
		CustomProperties: opts.CustomProperties,
		Warn:             opts.Warn,
		Sandbox:          sb,
	}, css.UsedNames{
		Classes: nameSet(used.Classes),
		IDs:     nameSet(used.IDs),
		Tags:    nameSet(used.Tags),
	})
	if err != nil {
		return "", compileErr(err, opts.Filename)
	}
	return out, nil
}

// PruneFile prunes the Stylus file at path (from Options.FS when set) like
// Prune.
func PruneFile(path string, used Used, opts Options) (string, error) {
	data, err := readSource(path, opts)
	if err != nil {
		return "", err
	}
	if opts.Filename == "" {
		opts.Filename = path
	}
	return Prune(string(data), used, opts)
}

// nameSet converts a name list to a set, preserving the nil = "unknown"
// distinction.
func nameSet(names []string) map[string]bool {
	if names == nil {
		return nil
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}

func sortedNames(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// readToken reads a tag or attribute name: letters, digits, '-', '_', ':',
// and any non-ASCII rune.
func readToken(s string, i int) (string, int) {
	start := i
	for i < len(s) && isTokenByte(s[i]) {
		i++
	}
	return s[start:i], i
}

func isTokenByte(b byte) bool {
	return b == '-' || b == '_' || b == ':' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9') || b >= 0x80
}

// scanAttrs walks the attributes of one open tag, recording class and id
// values, and returns the index just past the closing '>'.
func scanAttrs(html string, i int, classes, ids map[string]bool) int {
	for i < len(html) {
		for i < len(html) && (html[i] == ' ' || html[i] == '\t' || html[i] == '\n' || html[i] == '\r' || html[i] == '/') {
			i++
		}
		if i >= len(html) {
			return i
		}
		if html[i] == '>' {
			return i + 1
		}
		name, j := readToken(html, i)
		if name == "" {
			i++
			continue
		}
		i = j
		val := ""
		if i < len(html) && html[i] == '=' {
			i++
			val, i = readAttrValue(html, i)
		}
		switch strings.ToLower(name) {
		case "class":
			for _, c := range strings.Fields(val) {
				classes[c] = true
			}
		case "id":
			if v := strings.TrimSpace(val); v != "" {
				ids[v] = true
			}
		}
	}
	return i
}

// readAttrValue reads a quoted or unquoted attribute value starting at i.
func readAttrValue(s string, i int) (string, int) {
	if i < len(s) && (s[i] == '"' || s[i] == '\'') {
		quote := s[i]
		i++
		start := i
		for i < len(s) && s[i] != quote {
			i++
		}
		end := i
		if i < len(s) {
			i++ // consume the closing quote
		}
		return s[start:end], i
	}
	start := i
	for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' && s[i] != '>' && s[i] != '/' {
		i++
	}
	return s[start:i], i
}

// skipTo returns the index just past the next occurrence of b at or after i.
func skipTo(s string, i int, b byte) int {
	if j := strings.IndexByte(s[i:], b); j >= 0 {
		return i + j + 1
	}
	return len(s)
}
