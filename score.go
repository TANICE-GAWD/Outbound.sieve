package main

import "strings"








const (
	weightKeywords   = 55
	weightIndustry   = 30
	weightPainPoints = 15
)

func scoreICP(siteText string, icp ICP) int {
	if strings.TrimSpace(siteText) == "" {
		return 0 
	}
	hay := strings.ToLower(siteText)

	score := 0.0
	score += float64(weightKeywords) * hitRatio(hay, icp.Keywords)
	score += float64(weightIndustry) * hitRatio(hay, icp.Industries)
	score += float64(weightPainPoints) * hitRatio(hay, icp.PainPoints)

	return clamp(int(score+0.5), 0, 100)
}



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
