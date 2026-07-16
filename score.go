package main

import "strings"

// scoreICP rates a crawled target site against the ICP, 0-100.
//
// Deterministic on purpose: scoring is the one thing a GTM lead will poke at,
// and "the model said 87" is not an answer. Weights are the tuning knob.
//
// ponytail: keyword overlap, not embeddings. Add semantic matching when
// synonyms visibly cost real accounts (e.g. "logistics" missing "freight").
const (
	weightKeywords   = 55
	weightIndustry   = 30
	weightPainPoints = 15
)

func scoreICP(siteText string, icp ICP) int {
	if strings.TrimSpace(siteText) == "" {
		return 0 // no evidence, no score
	}
	hay := strings.ToLower(siteText)

	score := 0.0
	score += float64(weightKeywords) * hitRatio(hay, icp.Keywords)
	score += float64(weightIndustry) * hitRatio(hay, icp.Industries)
	score += float64(weightPainPoints) * hitRatio(hay, icp.PainPoints)

	return clamp(int(score+0.5), 0, 100)
}

// hitRatio is the fraction of needles present in hay. Empty needles score 0 so
// a sparse ICP can't hand out free points.
func hitRatio(hay string, needles []string) float64 {
	total, hits := 0, 0
	for _, n := range needles {
		n = strings.ToLower(strings.TrimSpace(n))
		if n == "" {
			continue
		}
		total++
		if strings.Contains(hay, n) {
			hits++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

func clamp(v, lo, hi int) int {
	return max(lo, min(v, hi))
}
