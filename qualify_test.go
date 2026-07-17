package main

import "testing"

func TestParseEmployeeRange(t *testing.T) {
	big := 1 << 30
	cases := []struct {
		in       string
		min, max int
	}{
		{"1-50", 1, 50},
		{"51-200 employees", 51, 200},
		{"11 to 50", 11, 50},
		{"under 500", 0, 500},
		{"up to 100", 0, 100},
		{"500+", 500, big},
		{"over 1000", 1000, big},
		{"50 employees", 0, 50},
		{"1,200-5,000", 1200, 5000},
		{"", 0, big},
		{"small teams", 0, big},
	}
	for _, c := range cases {
		min, max := parseEmployeeRange(c.in)
		if min != c.min || max != c.max {
			t.Errorf("parseEmployeeRange(%q) = %d,%d want %d,%d", c.in, min, max, c.min, c.max)
		}
	}
}

func TestQualify(t *testing.T) {
	icp := ICP{
		EmployeeRange: "1-50",
		Industries:    []string{"asset management", "hedge fund"},
		Keywords:      []string{"research", "portfolio"},
	}
	site := "We are a boutique asset management firm focused on equity research."

	// In-range + fit => kept.
	if pass, r := qualify(candidate{Employees: 32}, site, icp); !pass {
		t.Errorf("in-range fit should pass, got %q", r)
	}

	// Over cap => dropped, regardless of fit.
	if pass, r := qualify(candidate{Employees: 4200}, site, icp); pass {
		t.Errorf("over-cap should fail, got pass with %q", r)
	}

	// Unknown headcount is kept (flagged), not dropped, when fit matches.
	if pass, r := qualify(candidate{Employees: -1}, site, icp); !pass {
		t.Errorf("unknown size with fit should pass, got %q", r)
	}

	// No ICP match => dropped even if size is fine.
	if pass, _ := qualify(candidate{Employees: 20}, "we sell artisanal cheese", icp); pass {
		t.Error("no industry/keyword match should fail")
	}
}

func TestCommaInt(t *testing.T) {
	cases := map[int]string{5: "5", 50: "50", 500: "500", 1200: "1,200", 4200: "4,200", 1000000: "1,000,000"}
	for n, want := range cases {
		if got := commaInt(n); got != want {
			t.Errorf("commaInt(%d) = %q, want %q", n, got, want)
		}
	}
}
