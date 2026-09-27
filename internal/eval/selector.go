package eval

import "strings"

// combineSelectors produces the fully-qualified selectors for a nested ruleset by
// combining each parent selector with each of the ruleset's own selectors
// (cartesian product). At the top level (no parents) the selectors pass through,
// except that a parent reference there resolves to nothing, as in Stylus
// (`& .a` → `.a`).
func combineSelectors(parents, selfs []string, pretty bool) []string {
	if len(parents) == 0 {
		out := make([]string, 0, len(selfs))
		for _, s := range selfs {
			if strings.Contains(s, "&") {
				// Stylus drops a selector that is only `&`; keep it when that
				// would leave the group empty, so the rule still prints.
				if r := strings.TrimSpace(strings.ReplaceAll(s, "&", "")); r != "" {
					out = append(out, r)
				}
				continue
			}
			out = append(out, s)
		}
		if len(out) == 0 {
			out = append(out, selfs...)
		}
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
//	&     -> every & is replaced by the parent    (a + &:hover  => a:hover,
//	         and nothing is prepended               a + .b &    => .b a)
//	:     -> attaches directly (pseudo-class)     (a + :hover   => a:hover)
//	> + ~ -> combinator, spaced in pretty mode     (ul + > li    => ul > li)
//	other -> descendant combinator                 (a + .active  => a .active)
//
// A parent reference can sit anywhere in the child: `.checkbox &`, `& + &`,
// `html.ie &.y`, `:not(&)`. Stylus substitutes each occurrence textually, even
// inside an attribute string (`[data-a="&"]` → `[data-a=".x"]`). This matches
// it exactly rather than skipping strings, so output stays byte-compatible.
func combine(parent, child string, pretty bool) string {
	if child == "" {
		return parent
	}
	if strings.Contains(child, "&") {
		return strings.ReplaceAll(child, "&", parent)
	}
	switch child[0] {
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
