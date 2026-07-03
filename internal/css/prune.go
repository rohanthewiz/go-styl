package css

import "strings"

// UsedNames is the set of names known to be present in a document, used by
// Prune to decide which selectors can still match. A nil map means "unknown"
// — selectors are never pruned on that axis; a non-nil empty map means "known
// empty".
type UsedNames struct {
	Classes map[string]bool
	IDs     map[string]bool
	Tags    map[string]bool
}

// Prune returns a copy of the resolved node tree keeping only the rules whose
// selectors can match a document containing the given used names. A selector
// survives when every class, ID, and type name it requires is present in the
// corresponding set (names inside functional pseudo-class arguments such as
// ":not(.x)" or ":is(.a, .b)" are never required — the selector may match
// without them). Selectors that require nothing — "@font-face", ":root",
// "*", "[data-x]" — always survive.
//
// Rules whose every selector is pruned are dropped, at-rules left empty are
// dropped, and @keyframes blocks are dropped unless a surviving declaration
// references their name from an animation or animation-name property
// (vendor-prefixed forms included). Non-rule nodes (@import, @charset lines)
// pass through. The input nodes are not modified.
func Prune(nodes []Node, used UsedNames, pretty bool) []Node {
	kept := pruneNodes(nodes, used, pretty)
	return pruneKeyframes(kept, animationRefs(kept))
}

// pruneNodes applies selector pruning to one nesting level, recursing into
// at-rule bodies. @keyframes blocks are kept untouched for the second pass.
func pruneNodes(nodes []Node, used UsedNames, pretty bool) []Node {
	out := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		switch node := n.(type) {
		case *Rule:
			if r := pruneRule(node, used, pretty); r != nil {
				out = append(out, r)
			}
		case *AtRule:
			if _, ok := keyframesName(node.Header); ok {
				out = append(out, node)
				continue
			}
			inner := pruneNodes(node.Nodes, used, pretty)
			if len(inner) > 0 {
				out = append(out, &AtRule{Header: node.Header, Nodes: inner, Pos: node.Pos})
			}
		default:
			out = append(out, n)
		}
	}
	return out
}

// pruneRule returns a copy of rule with unmatchable selectors removed from its
// own selector list, its @extend grafts, and its merged duplicates, or nil
// when nothing would render.
func pruneRule(rule *Rule, used UsedNames, pretty bool) *Rule {
	if len(rule.Statements) == 0 {
		return nil
	}
	out := *rule
	if !rule.Placeholder {
		out.Selectors = keepSelectors(rule.Selectors, used)
		out.Selector = joinGroup(rule.Selector, rule.Selectors, out.Selectors, pretty)
	}
	out.Extenders = keepSelectors(rule.Extenders, used)
	out.Duplicates = nil
	for _, dup := range rule.Duplicates {
		if d := pruneRule(dup, used, pretty); d != nil {
			out.Duplicates = append(out.Duplicates, d)
		}
	}
	if !out.hasOutput() {
		return nil
	}
	return &out
}

// joinGroup rebuilds a rule's pre-joined selector string after pruning; when
// every selector survived the original string is reused verbatim.
func joinGroup(orig string, before, after []string, pretty bool) string {
	if len(after) == len(before) {
		return orig
	}
	sep := ","
	if pretty {
		sep = ", "
	}
	return strings.Join(after, sep)
}

// keepSelectors filters a selector list down to those that can still match.
func keepSelectors(sels []string, used UsedNames) []string {
	if len(sels) == 0 {
		return nil
	}
	out := make([]string, 0, len(sels))
	for _, sel := range sels {
		if selectorMatchable(sel, used) {
			out = append(out, sel)
		}
	}
	return out
}

// selectorMatchable reports whether a selector could match a document limited
// to the used names. A stray comma-joined group counts as matchable when any
// of its parts is, so over-grouped input errs toward keeping.
func selectorMatchable(sel string, used UsedNames) bool {
	for _, part := range splitTopLevel(sel) {
		if partMatchable(part, used) {
			return true
		}
	}
	return false
}

func partMatchable(sel string, used UsedNames) bool {
	ok := true
	scanSelector(sel, func(kind byte, name string) {
		switch kind {
		case '.':
			if used.Classes != nil && !used.Classes[name] {
				ok = false
			}
		case '#':
			if used.IDs != nil && !used.IDs[name] {
				ok = false
			}
		case 't':
			if used.Tags != nil && !used.Tags[strings.ToLower(name)] {
				ok = false
			}
		}
	})
	return ok
}

// scanSelector walks one selector and reports each name it requires to match:
// kind '.' for classes, '#' for IDs, and 't' for type (tag) selectors. Quoted
// strings, attribute blocks, and the parenthesised arguments of functional
// pseudo-classes are skipped; escaped characters never start a name.
func scanSelector(sel string, report func(kind byte, name string)) {
	runes := []rune(sel)
	expectType := true // a type selector can appear here (start / after combinator)
	for i := 0; i < len(runes); i++ {
		switch r := runes[i]; {
		case r == '\\':
			i++
			expectType = false
		case r == '"' || r == '\'':
			i = skipQuoted(runes, i)
			expectType = false
		case r == '[':
			i = skipAttr(runes, i)
			expectType = false
		case r == '.' || r == '#':
			name, end := readIdent(runes, i+1)
			if name != "" {
				report(byte(r), name)
			}
			i = end - 1
			expectType = false
		case r == ':':
			for i+1 < len(runes) && runes[i+1] == ':' {
				i++
			}
			_, end := readIdent(runes, i+1)
			i = end - 1
			if end < len(runes) && runes[end] == '(' {
				i = skipParens(runes, end)
			}
			expectType = false
		case r == ' ' || r == '\t' || r == '>' || r == '+' || r == '~':
			expectType = true
		case expectType && identStart(r):
			name, end := readIdent(runes, i)
			if name != "" && name != "-" {
				report('t', name)
			}
			i = end - 1
			expectType = false
		default:
			expectType = false
		}
	}
}

// skipParens returns the index of the ')' matching the '(' at runes[i],
// honoring nesting, quotes, and attribute blocks.
func skipParens(runes []rune, i int) int {
	depth := 0
	for ; i < len(runes); i++ {
		switch runes[i] {
		case '"', '\'':
			i = skipQuoted(runes, i)
		case '[':
			i = skipAttr(runes, i)
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

// splitTopLevel splits a selector string on commas outside quotes, brackets,
// and parentheses. Resolved selectors are normally comma-free; this guards
// pruning decisions on any that are not.
func splitTopLevel(sel string) []string {
	if !strings.Contains(sel, ",") {
		return []string{sel}
	}
	runes := []rune(sel)
	var parts []string
	start := 0
	for i := 0; i < len(runes); i++ {
		switch runes[i] {
		case '\\':
			i++
		case '"', '\'':
			i = skipQuoted(runes, i)
		case '[':
			i = skipAttr(runes, i)
		case '(':
			i = skipParens(runes, i)
		case ',':
			parts = append(parts, string(runes[start:i]))
			start = i + 1
		}
	}
	return append(parts, string(runes[start:]))
}

// animationRefs collects every identifier and quoted-string token appearing in
// the value of an animation or animation-name declaration (vendor prefixes
// stripped) anywhere in the kept tree. Matching keyframes names against this
// superset only ever errs toward keeping a block.
func animationRefs(nodes []Node) map[string]bool {
	refs := map[string]bool{}
	var walk func([]Node)
	walk = func(nodes []Node) {
		for _, n := range nodes {
			switch node := n.(type) {
			case *Rule:
				collectAnimationRefs(node, refs)
			case *AtRule:
				if _, ok := keyframesName(node.Header); !ok {
					walk(node.Nodes)
				}
			}
		}
	}
	walk(nodes)
	return refs
}

func collectAnimationRefs(rule *Rule, refs map[string]bool) {
	for _, st := range rule.Statements {
		if p := stripVendor(strings.ToLower(st.Property)); p == "animation" || p == "animation-name" {
			valueTokens(st.Value, refs)
		}
	}
	for _, dup := range rule.Duplicates {
		collectAnimationRefs(dup, refs)
	}
}

// stripVendor removes a "-vendor-" prefix ("-webkit-animation" -> "animation").
func stripVendor(p string) string {
	if strings.HasPrefix(p, "-") {
		if i := strings.Index(p[1:], "-"); i >= 0 {
			return p[i+2:]
		}
	}
	return p
}

// valueTokens adds each identifier and quoted string in a declaration value to
// the set.
func valueTokens(v string, refs map[string]bool) {
	runes := []rune(v)
	for i := 0; i < len(runes); i++ {
		switch r := runes[i]; {
		case r == '"' || r == '\'':
			end := skipQuoted(runes, i)
			if end > i+1 && end <= len(runes) {
				refs[unescapeQuoted(runes[i+1:min(end, len(runes))])] = true
			}
			i = end
		case identStart(r):
			name, end := readIdent(runes, i)
			refs[name] = true
			i = end - 1
		}
	}
}

func unescapeQuoted(runes []rune) string {
	var b strings.Builder
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) {
			i++
		}
		b.WriteRune(runes[i])
	}
	return b.String()
}

// pruneKeyframes drops @keyframes blocks whose name is not referenced by any
// kept animation declaration, then drops at-rules that became empty.
func pruneKeyframes(nodes []Node, refs map[string]bool) []Node {
	out := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		atr, ok := n.(*AtRule)
		if !ok {
			out = append(out, n)
			continue
		}
		if name, isKF := keyframesName(atr.Header); isKF {
			if refs[name] {
				out = append(out, atr)
			}
			continue
		}
		inner := pruneKeyframes(atr.Nodes, refs)
		if len(inner) > 0 {
			out = append(out, &AtRule{Header: atr.Header, Nodes: inner, Pos: atr.Pos})
		}
	}
	return out
}
