package css

import "strings"

// Scope renames, in place, every local class name and @keyframes name in a
// resolved node tree, for component-scoped stylesheets (CSS Modules
// semantics). rename maps a local name to its scoped form; the returned map
// records each name that was actually renamed (local -> scoped).
//
// What gets renamed:
//
//   - ".name" tokens in rule selectors — the rule's own selectors, @extend
//     grafts, and merged duplicates. Names inside functional pseudo-classes
//     (":not(.x)", ":is(.a, .b)") are local too.
//   - The name of every @keyframes block (vendor-prefixed forms included),
//     and identifier or quoted-string tokens equal to one of those names in
//     animation / animation-name values. Animation references to keyframes
//     not defined in this tree are left alone — they are someone else's.
//
// What does not: element IDs, type selectors, attribute-selector contents,
// quoted strings, escaped characters, and anything wrapped in
// ":global(...)". The wrapper is the opt-out: it is removed and its contents
// are emitted verbatim, so ".card :global(.is-open)" becomes
// ".card_x .is-open".
//
// The rewrite is textual over the flattened selectors, which is safe because
// @extend matching (applyExtends) and duplicate merging have already run on
// the local names by the time this is called.
func Scope(nodes []Node, rename func(string) string) map[string]string {
	s := &scoper{rename: rename, renamed: map[string]string{}, keyframes: map[string]bool{}}
	// Two passes: animation declarations may precede (or live in a different
	// @media block than) the @keyframes they reference, so the set of local
	// keyframes names must be complete before any value is rewritten.
	s.collectKeyframes(nodes)
	s.walk(nodes)
	return s.renamed
}

type scoper struct {
	rename    func(string) string
	renamed   map[string]string // local -> scoped, for every name rewritten
	keyframes map[string]bool   // local @keyframes names defined in the tree
}

// name returns the scoped form of a local name, memoizing it in renamed so
// the caller gets the complete mapping back.
func (s *scoper) name(local string) string {
	if scoped, ok := s.renamed[local]; ok {
		return scoped
	}
	scoped := s.rename(local)
	s.renamed[local] = scoped
	return scoped
}

func (s *scoper) collectKeyframes(nodes []Node) {
	for _, n := range nodes {
		if atr, ok := n.(*AtRule); ok {
			if name, isKF := keyframesName(atr.Header); isKF {
				s.keyframes[name] = true
				continue
			}
			s.collectKeyframes(atr.Nodes)
		}
	}
}

func (s *scoper) walk(nodes []Node) {
	for _, n := range nodes {
		switch node := n.(type) {
		case *Rule:
			s.rule(node)
		case *AtRule:
			if name, ok := keyframesName(node.Header); ok {
				// Rebuild the header rather than string-replace inside it: the
				// name may have been quoted, and the at-word itself (e.g.
				// "@-webkit-keyframes") must not be touched. Step selectors
				// (from/to/percentages) in the body carry no names.
				atWord, _, _ := strings.Cut(node.Header, " ")
				node.Header = atWord + " " + s.name(name)
				continue
			}
			s.walk(node.Nodes)
		}
	}
}

func (s *scoper) rule(rule *Rule) {
	rule.Selector = s.selector(rule.Selector)
	for i, sel := range rule.Selectors {
		rule.Selectors[i] = s.selector(sel)
	}
	for i, sel := range rule.Extenders {
		rule.Extenders[i] = s.selector(sel)
	}
	for _, st := range rule.Statements {
		if p := stripVendor(strings.ToLower(st.Property)); p == "animation" || p == "animation-name" {
			st.Value = s.animationValue(st.Value)
		}
	}
	for _, dup := range rule.Duplicates {
		s.rule(dup)
	}
}

// globalPrefix opens the CSS Modules opt-out wrapper.
const globalPrefix = ":global("

// selector rewrites the class names in one selector string. The scan mirrors
// nameCollector.selector (escapes, quotes, and attribute blocks are copied
// through untouched) so that exactly the names CollectNames reports as
// classes are the ones renamed.
func (s *scoper) selector(sel string) string {
	// Fast path: no class token and no :global wrapper means nothing to do
	// (e.g. "@font-face", "html body", "#main").
	if !strings.ContainsAny(sel, ".:") {
		return sel
	}
	runes := []rune(sel)
	var b strings.Builder
	b.Grow(len(sel) + 16)
	for i := 0; i < len(runes); i++ {
		switch r := runes[i]; r {
		case '\\':
			b.WriteRune(r)
			if i+1 < len(runes) {
				i++
				b.WriteRune(runes[i])
			}
		case '"', '\'':
			end := skipQuoted(runes, i)
			b.WriteString(string(runes[i:min(end+1, len(runes))]))
			i = end
		case '[':
			end := skipAttr(runes, i)
			b.WriteString(string(runes[i:min(end+1, len(runes))]))
			i = end
		case ':':
			if hasRunePrefix(runes[i:], globalPrefix) {
				open := i + len(globalPrefix) - 1 // index of '('
				end := closeParen(runes, open)
				// Emit the wrapped selector verbatim, minus ":global(" and ")".
				b.WriteString(string(runes[open+1 : min(end, len(runes))]))
				i = end
				continue
			}
			b.WriteRune(r)
		case '.':
			b.WriteRune(r)
			name, end := readIdent(runes, i+1)
			if name == "" {
				continue // e.g. a stray '.' before a digit: not a class token
			}
			b.WriteString(s.name(name))
			i = end - 1
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// animationValue rewrites the tokens of an animation / animation-name value
// that name a local @keyframes block, keeping every other token (durations,
// easing keywords, external keyframes names) as-is. Quoted names keep their
// quotes.
func (s *scoper) animationValue(v string) string {
	if len(s.keyframes) == 0 {
		return v
	}
	runes := []rune(v)
	var b strings.Builder
	for i := 0; i < len(runes); i++ {
		switch r := runes[i]; {
		case r == '"' || r == '\'':
			end := min(skipQuoted(runes, i), len(runes))
			if end > i+1 {
				if name := unescapeQuoted(runes[i+1 : end]); s.keyframes[name] {
					b.WriteRune(r)
					b.WriteString(s.name(name))
					if end < len(runes) {
						b.WriteRune(runes[end])
					}
					i = end
					continue
				}
			}
			b.WriteString(string(runes[i:min(end+1, len(runes))]))
			i = end
		case identStart(r):
			name, end := readIdent(runes, i)
			if s.keyframes[name] {
				b.WriteString(s.name(name))
			} else {
				b.WriteString(name)
			}
			i = end - 1
		case r >= '0' && r <= '9':
			// Consume a whole numeric token ("1.5s", "2e3ms") so its unit is
			// never mistaken for a keyframes name ("1s" with keyframes "s").
			end := i
			for end < len(runes) && (identCont(runes[end]) || runes[end] == '.' || runes[end] == '%') {
				end++
			}
			b.WriteString(string(runes[i:end]))
			i = end - 1
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// hasRunePrefix reports whether runes starts with prefix (ASCII).
func hasRunePrefix(runes []rune, prefix string) bool {
	if len(runes) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if runes[i] != rune(prefix[i]) {
			return false
		}
	}
	return true
}

// closeParen returns the index of the ')' matching the '(' at runes[open],
// honoring nesting ("(:is(.a))"), quotes, and escapes, or len(runes) when
// unbalanced.
func closeParen(runes []rune, open int) int {
	depth := 0
	for i := open; i < len(runes); i++ {
		switch runes[i] {
		case '\\':
			i++
		case '"', '\'':
			i = skipQuoted(runes, i)
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(runes)
}
