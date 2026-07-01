package eval

// suggest returns the candidate closest to name by edit distance, for
// "did you mean" hints. It returns "" when nothing is close enough (distance
// above 1 for short names, 2 otherwise). Ties break lexicographically so
// suggestions are deterministic.
func suggest(name string, candidates []string) string {
	maxDist := 2
	if len(name) <= 4 {
		maxDist = 1
	}
	best, bestDist := "", maxDist+1
	for _, c := range candidates {
		if c == name {
			continue
		}
		d := editDistance(name, c)
		if d < bestDist || (d == bestDist && c < best) {
			best, bestDist = c, d
		}
	}
	if bestDist > maxDist {
		return ""
	}
	return best
}

// editDistance is the Levenshtein distance between a and b.
func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, min(cur[j-1]+1, prev[j-1]+cost))
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
