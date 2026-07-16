package main

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestScoreICP(t *testing.T) {
	icp := ICP{
		Keywords:   []string{"outbound", "cold email", "pipeline"},
		Industries: []string{"saas", "agency"},
		PainPoints: []string{"low reply rates", "manual prospecting"},
	}

	full := "We are a SaaS agency fixing outbound. Cold email pipeline broken? " +
		"Low reply rates and manual prospecting are killing you."
	if got := scoreICP(full, icp); got != 100 {
		t.Errorf("full match = %d, want 100", got)
	}

	if got := scoreICP("", icp); got != 0 {
		t.Errorf("empty site = %d, want 0", got)
	}
	if got := scoreICP("we sell artisanal cheese to restaurants", icp); got != 0 {
		t.Errorf("unrelated site = %d, want 0", got)
	}

	got := scoreICP("OUTBOUND tooling for SaaS teams", icp)
	if got <= 0 || got >= 100 {
		t.Errorf("partial match = %d, want strictly between 0 and 100", got)
	}

	if got := scoreICP("anything at all", ICP{}); got != 0 {
		t.Errorf("empty ICP = %d, want 0", got)
	}

	kwOnly := scoreICP("outbound cold email pipeline", icp)
	ppOnly := scoreICP("low reply rates manual prospecting", icp)
	if kwOnly <= ppOnly {
		t.Errorf("keywords (%d) should outweigh pain points (%d)", kwOnly, ppOnly)
	}
}

func TestHTMLText(t *testing.T) {
	in := `<html><head><style>body{color:red}</style><script>var x=1;</script></head>
	       <body><h1>Acme</h1><p>We do&nbsp;things &amp; stuff</p></body></html>`
	got := htmlText(in)
	want := "Acme We do things & stuff"
	if got != want {
		t.Errorf("htmlText = %q, want %q", got, want)
	}
}

func TestMetaText_ClientRenderedShell(t *testing.T) {
	raw := `<html><head>
	  <title>LoopGTM | Co-Build Your GTM Engine in 90 Days</title>
	  <meta name="description" content="LoopGTM helps recruiting &amp; staffing firms co-build their GTM engine." />
	  <meta property="og:description" content="LoopGTM helps recruiting &amp; staffing firms co-build their GTM engine." />
	  <meta property="og:image" content="https://example.com/x.png" />
	  <script type="application/ld+json">{"@type":"Organization"}</script>
	</head><body><div id="root"></div></body></html>`

	got := metaText(raw)
	if !strings.Contains(got, "recruiting & staffing") {
		t.Errorf("metaText lost the ICP line: %q", got)
	}

	if n := strings.Count(got, "co-build their GTM engine"); n != 1 {
		t.Errorf("duplicate description kept %d times: %q", n, got)
	}

	if strings.Contains(got, "x.png") {
		t.Errorf("metaText leaked og:image: %q", got)
	}

	if len(got) < 60 {
		t.Errorf("metaText too thin to be useful: %q", got)
	}
}

func TestParseAccountsCSV(t *testing.T) {
	raw := "Company,Website,# Employees\n" +
		"Acme Staffing,acme.com,\"1,200\"\n" +
		"Boutique Recruiters,boutique.io,51-200\n" +
		",noname.com,40\n" + 
		"NoSize Co,nosize.com,\n" 
	cands, err := parseAccountsCSV(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 3 {
		t.Fatalf("got %d candidates, want 3: %+v", len(cands), cands)
	}
	if cands[0].Employees != 1200 {
		t.Errorf(`"1,200" parsed as %d, want 1200`, cands[0].Employees)
	}
	if cands[1].Employees != 51 {
		t.Errorf(`"51-200" parsed as %d, want 51`, cands[1].Employees)
	}
	if cands[2].Employees != -1 {
		t.Errorf("empty size parsed as %d, want -1", cands[2].Employees)
	}

	
	kept, dropped := filterBySize(cands, 500)
	if dropped != 1 || len(kept) != 2 {
		t.Errorf("filterBySize(500) kept %d dropped %d, want kept 2 dropped 1", len(kept), dropped)
	}

	
	if c, _ := parseAccountsCSV("Foo,Bar\n1,2"); c != nil {
		t.Errorf("CSV without name/website columns should return nil, got %+v", c)
	}
}


func TestEnrichAndFilter_Live(t *testing.T) {
	key := os.Getenv("ABSTRACT_API_KEY")
	if key == "" || testing.Short() {
		t.Skip("no ABSTRACT_API_KEY (or -short): skipping live enrichment check")
	}
	cands := []candidate{
		{Name: "Aerotek", Website: "aerotek.com", Employees: -1},
		{Name: "The Bowdoin Group", Website: "https://www.bowdoingroup.com/about", Employees: -1},
	}
	for i := range cands {
		if n, ok := enrichEmployees(context.Background(), domainOf(cands[i].Website), key); ok {
			cands[i].Employees = n
		}
	}
	if cands[0].Employees < 1000 {
		t.Errorf("Aerotek should enrich to a large headcount, got %d", cands[0].Employees)
	}
	kept, dropped := filterBySize(cands, 1000)
	if dropped != 1 || len(kept) != 1 || kept[0].Name != "The Bowdoin Group" {
		t.Errorf("cap 1000 should drop Aerotek and keep Bowdoin; kept=%v dropped=%d", names(kept), dropped)
	}
}

func names(cs []candidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

func TestDomainOf(t *testing.T) {
	cases := map[string]string{
		"https://www.acme.com/about": "acme.com",
		"http://acme.com":            "acme.com",
		"ACME.com":                   "acme.com",
		"acme.com/pricing?ref=x":     "acme.com",
		"www.sub.acme.co.uk":         "sub.acme.co.uk",
	}
	for in, want := range cases {
		if got := domainOf(in); got != want {
			t.Errorf("domainOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeURL(t *testing.T) {
	cases := map[string]string{
		"acme.com":          "https://acme.com",
		"https://acme.com/": "https://acme.com",
		"http://acme.com":   "http://acme.com",
		"  acme.com  ":      "https://acme.com",
		"":                  "",
	}
	for in, want := range cases {
		if got := normalizeURL(in); got != want {
			t.Errorf("normalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}
