package eval

import "strings"

// combineSelectors produces the fully-qualified selectors for a nested ruleset by
// combining each parent selector with each of the ruleset's own selectors
// (cartesian product). At the top level (no parents) the selectors pass through.
func combineSelectors(parents, selfs []string, pretty bool) []string {
	if len(parents) == 0 {
		out := make([]string, len(selfs))
		copy(out, selfs)
		return out
	}
	out := make([]string, 0, len(parents)*len(selfs))
	for _, p := range parents {
		for _, s := range selfs {
			out = append(out, combine(p, s, pretty))
		}
	}
	return out
}

// combine joins a single parent selector with a single child selector following
// Stylus/scarlet nesting rules:
//
//	&     -> attaches directly to the parent     (a + &:hover  => a:hover)
//	:     -> attaches directly (pseudo-class)     (a + :hover   => a:hover)
//	> + ~ -> combinator, spaced in pretty mode     (ul + > li    => ul > li)
//	other -> descendant combinator                 (a + .active  => a .active)
func combine(parent, child string, pretty bool) string {
	if child == "" {
		return parent
	}
	switch child[0] {
	case '&':
		return parent + child[1:]
	case ':':
		return parent + child
	case '>', '+', '~':
		if pretty {
			return parent + " " + child
		}
		return parent + child[:1] + strings.TrimSpace(child[1:])
	default:
		return parent + " " + child
	}
}

// joinSelectors renders a selector group as a single comma-separated string.
func joinSelectors(sels []string, pretty bool) string {
	sep := ","
	if pretty {
		sep = ", "
	}
	return strings.Join(sels, sep)
}
