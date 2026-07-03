package css

import (
	"sort"
	"strings"
)

// Names holds the identifiers found in a resolved stylesheet: class names,
// element IDs, and @keyframes animation names. Each list is sorted and
// duplicate-free.
type Names struct {
	Classes   []string
	IDs       []string
	Keyframes []string
}

// CollectNames walks a resolved node tree and gathers every class name, ID,
// and keyframes name that would appear in the rendered CSS. Rules that render
// no output (empty bodies, unextended placeholders) are skipped so the result
// mirrors the final stylesheet.
func CollectNames(nodes []Node) Names {
	c := &nameCollector{classes: map[string]bool{}, ids: map[string]bool{}, keyframes: map[string]bool{}}
	c.walk(nodes)
	return Names{
		Classes:   sortedKeys(c.classes),
		IDs:       sortedKeys(c.ids),
		Keyframes: sortedKeys(c.keyframes),
	}
}

type nameCollector struct {
	classes   map[string]bool
	ids       map[string]bool
	keyframes map[string]bool
}

func (c *nameCollector) walk(nodes []Node) {
	for _, n := range nodes {
		switch node := n.(type) {
		case *Rule:
			c.rule(node)
		case *AtRule:
			if name, ok := keyframesName(node.Header); ok {
				if node.hasOutput() {
					c.keyframes[name] = true
				}
				continue // step selectors (from/to/percentages) carry no names
			}
			c.walk(node.Nodes)
		}
	}
}

// rule scans the selectors a rule would actually render: its own (unless it is
// a placeholder), selectors grafted on via @extend, and selectors of merged
// duplicate rules.
func (c *nameCollector) rule(rule *Rule) {
	if !rule.hasOutput() {
		return
	}
	if !rule.Placeholder {
		for _, sel := range rule.Selectors {
			c.selector(sel)
		}
	}
	for _, sel := range rule.Extenders {
		c.selector(sel)
	}
	for _, dup := range rule.Duplicates {
		for _, sel := range dup.Selectors {
			c.selector(sel)
		}
	}
}

// selector scans one selector string for ".class" and "#id" tokens. Quoted
// strings and attribute blocks ("[href$=\".png\"]") are skipped so their
// contents are not mistaken for names; escaped characters never start a name.
func (c *nameCollector) selector(sel string) {
	runes := []rune(sel)
	for i := 0; i < len(runes); i++ {
		switch r := runes[i]; r {
		case '\\':
			i++ // skip the escaped character
		case '"', '\'':
			i = skipQuoted(runes, i)
		case '[':
			i = skipAttr(runes, i)
		case '.', '#':
			name, end := readIdent(runes, i+1)
			if name != "" {
				if r == '.' {
					c.classes[name] = true
				} else {
					c.ids[name] = true
				}
			}
			i = end - 1
		}
	}
}

// skipQuoted returns the index of the closing quote matching runes[i].
func skipQuoted(runes []rune, i int) int {
	quote := runes[i]
	for i++; i < len(runes); i++ {
		switch runes[i] {
		case '\\':
			i++
		case quote:
			return i
		}
	}
	return len(runes)
}

// skipAttr returns the index of the ']' closing the attribute block opened at
// runes[i], honoring quoted values inside it.
func skipAttr(runes []rune, i int) int {
	for i++; i < len(runes); i++ {
		switch runes[i] {
		case '"', '\'':
			i = skipQuoted(runes, i)
		case ']':
			return i
		}
	}
	return len(runes)
}

// readIdent reads a CSS identifier starting at runes[i], returning it and the
// index just past its end. It returns "" when the position cannot start an
// identifier (e.g. a digit).
func readIdent(runes []rune, i int) (string, int) {
	start := i
	if i >= len(runes) || !identStart(runes[i]) {
		return "", i
	}
	for i < len(runes) && identCont(runes[i]) {
		i++
	}
	return string(runes[start:i]), i
}

func identStart(r rune) bool {
	return r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r >= 0x80
}

func identCont(r rune) bool {
	return identStart(r) || (r >= '0' && r <= '9')
}

// keyframesName extracts the animation name from a "@keyframes name" (or
// vendor-prefixed) at-rule header.
func keyframesName(header string) (string, bool) {
	atWord, rest, ok := strings.Cut(header, " ")
	if !ok || !strings.HasPrefix(atWord, "@") || !strings.HasSuffix(atWord, "keyframes") {
		return "", false
	}
	name := strings.Trim(strings.TrimSpace(rest), `"'`)
	if name == "" {
		return "", false
	}
	return name, true
}

func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
