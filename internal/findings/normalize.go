package findings

// Deduplicate removes findings with duplicate IDs, keeping the one with the
// highest confidence (confirmed > likely > needs_review).
func Deduplicate(in []Finding) []Finding {
	seen := make(map[string]Finding)
	order := []string{}

	rank := map[string]int{
		"confirmed":    2,
		"likely":       1,
		"needs_review": 0,
	}

	for _, f := range in {
		if existing, ok := seen[f.ID]; ok {
			if rank[f.Confidence] > rank[existing.Confidence] {
				seen[f.ID] = f
			}
		} else {
			seen[f.ID] = f
			order = append(order, f.ID)
		}
	}

	out := make([]Finding, 0, len(order))
	for _, id := range order {
		out = append(out, seen[id])
	}
	return out
}

// Correlate enriches findings from one tool with evidence from another when
// they share the same location.
func Correlate(primary, secondary []Finding) []Finding {
	byLoc := make(map[string]*Finding)
	for i := range secondary {
		byLoc[secondary[i].Location] = &secondary[i]
	}

	result := make([]Finding, len(primary))
	copy(result, primary)

	for i := range result {
		if extra, ok := byLoc[result[i].Location]; ok && extra.Evidence != "" {
			if result[i].Evidence == "" {
				result[i].Evidence = extra.Evidence
			}
		}
	}
	return result
}
