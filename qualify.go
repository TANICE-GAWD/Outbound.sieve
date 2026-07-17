package main

import (
	"fmt"
	"strconv"
	"strings"
)

// parseEmployeeRange turns an ICP size string into a [min,max] headcount band.
// Handles "1-50", "51-200 employees", "under 500", "500+", "11 to 50".
// Missing bound => 0 (min) or a large sentinel (max). Unparseable => 0,maxInt.
func parseEmployeeRange(s string) (min, max int) {
	const big = 1 << 30
	nums := extractInts(s)
	low := strings.ToLower(s)

	switch {
	case len(nums) >= 2:
		return nums[0], nums[1]
	case len(nums) == 1 && (strings.Contains(low, "under") || strings.Contains(low, "up to") || strings.Contains(low, "<")):
		return 0, nums[0]
	case len(nums) == 1 && (strings.Contains(low, "+") || strings.Contains(low, "over") || strings.Contains(low, "more")):
		return nums[0], big
	case len(nums) == 1:
		return 0, nums[0] // a lone number reads as a ceiling ("50 employees")
	default:
		return 0, big
	}
}

func extractInts(s string) []int {
	var out []int
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		if n, err := strconv.Atoi(cur.String()); err == nil {
			out = append(out, n)
		}
		cur.Reset()
	}
	for _, r := range s {
		if r >= '0' && r <= '9' {
			cur.WriteRune(r)
		} else if r != ',' { // treat "1,200" as 1200
			flush()
		}
	}
	flush()
	return out
}

// qualify is the hard ICP gate. Every decision returns a human-readable reason
// so the output is auditable — the whole point for a buyer who says Clay's
// filtering is a black box. Unknown headcount is kept and flagged, never dropped.
func qualify(cand candidate, siteText string, icp ICP) (pass bool, reason string) {
	min, max := parseEmployeeRange(icp.EmployeeRange)

	// Size gate.
	sizeNote := "size unknown"
	if cand.Employees >= 0 {
		if cand.Employees > max {
			return false, fmt.Sprintf("dropped — %s over %s cap", commaInt(cand.Employees), commaInt(max))
		}
		if cand.Employees < min {
			return false, fmt.Sprintf("dropped — %s under %s floor", commaInt(cand.Employees), commaInt(min))
		}
		sizeNote = fmt.Sprintf("%s emp within %s–%s", commaInt(cand.Employees), commaInt(min), commaInt(max))
	}

	// Fit gate: must hit at least one industry or keyword from the ICP.
	matched := matches(siteText, icp.Industries, icp.Keywords)
	if len(matched) == 0 {
		return false, fmt.Sprintf("dropped — %s, no ICP industry/keyword match", sizeNote)
	}
	return true, fmt.Sprintf("kept — %s, matched %s", sizeNote, quoteList(matched, 3))
}

// matches returns the ICP terms actually present in the site text.
func matches(siteText string, groups ...[]string) []string {
	hay := strings.ToLower(siteText)
	var hits []string
	seen := map[string]bool{}
	for _, g := range groups {
		for _, term := range g {
			term = strings.ToLower(strings.TrimSpace(term))
			if term == "" || seen[term] {
				continue
			}
			if strings.Contains(hay, term) {
				seen[term] = true
				hits = append(hits, term)
			}
		}
	}
	return hits
}

func quoteList(xs []string, n int) string {
	if len(xs) > n {
		xs = xs[:n]
	}
	q := make([]string, len(xs))
	for i, x := range xs {
		q[i] = `"` + x + `"`
	}
	return strings.Join(q, ", ")
}

func commaInt(n int) string {
	if n >= 1<<30 {
		return "∞"
	}
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	pre := len(s) % 3
	if pre > 0 {
		b.WriteString(s[:pre])
	}
	for i := pre; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
