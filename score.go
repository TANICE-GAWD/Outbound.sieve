package main

import "strings"

const (
	weightKeywords   = 55
	weightIndustry   = 30
	weightPainPoints = 15

	// A single company's site never mentions *every* ICP term, so scoring on the
	// fraction of all terms punishes clear fits. These saturation points say "this
	// many hits already proves fit": reach them and the category pays out in full.
	fullKeywords   = 3 // three on-ICP keywords is a strong signal
	fullIndustry   = 1 // you're either in an ICP industry or you aren't
	fullPainPoints = 1 // any pain-point match is meaningful (and rare)
)

func scoreICP(siteText string, icp ICP) int {
	if strings.TrimSpace(siteText) == "" {
		return 0
	}
	hay := strings.ToLower(siteText)

	score := 0.0
	score += weightKeywords * saturate(hitCount(hay, icp.Keywords), fullKeywords)
	score += weightIndustry * saturate(hitCount(hay, icp.Industries), fullIndustry)
	score += weightPainPoints * saturate(hitCount(hay, icp.PainPoints), fullPainPoints)

	return clamp(int(score+0.5), 0, 100)
}

// hitCount is how many distinct needles appear in hay.
func hitCount(hay string, needles []string) int {
	n := 0
	for _, s := range needles {
		s = strings.ToLower(strings.TrimSpace(s))
		if s != "" && strings.Contains(hay, s) {
			n++
		}
	}
	return n
}

// saturate caps credit: full hits (or more) => 1.0, fewer => proportional.
func saturate(hits, full int) float64 {
	if full <= 0 || hits >= full {
		return 1
	}
	return float64(hits) / float64(full)
}

func clamp(v, lo, hi int) int {
	return max(lo, min(v, hi))
}
