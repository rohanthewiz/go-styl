package css

import (
	"hash/fnv"
	"sort"
)

// statementsHash returns a hash of a rule's declaration body, order-independent,
// used to detect rules with identical declarations. Statements are sorted by
// property first so that declaration order does not affect the hash.
func statementsHash(rule *Rule) uint64 {
	sort.Slice(rule.Statements, func(i, j int) bool {
		return rule.Statements[i].Property < rule.Statements[j].Property
	})

	h := fnv.New64a()
	for _, st := range rule.Statements {
		_, _ = h.Write([]byte(st.Property))
		_, _ = h.Write([]byte(st.Value))
	}
	return h.Sum64()
}

// MergeDuplicates folds rules that share an identical declaration body into a
// single rule with a comma-separated selector group. This is the optional
// compression pass inherited from scarlet; standard Stylus output leaves it off.
//
//	a { color: blue }
//	p { color: blue }  =>  a, p { color: blue }
func MergeDuplicates(rules []*Rule) []*Rule {
	result := make([]*Rule, 0, len(rules))
	seen := map[uint64]*Rule{}

	for _, rule := range rules {
		h := statementsHash(rule)
		if existing, ok := seen[h]; ok {
			existing.Duplicates = append(existing.Duplicates, rule)
		} else {
			result = append(result, rule)
			seen[h] = rule
		}
	}

	return result
}
