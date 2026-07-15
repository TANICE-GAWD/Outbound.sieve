package main

import (
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
	  <meta property="og:image" content="https:
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

func TestNormalizeURL(t *testing.T) {
	cases := map[string]string{
		"acme.com":          "https:
		"https:
		"http:
		"  acme.com  ":      "https:
		"":                  "",
	}
	for in, want := range cases {
		if got := normalizeURL(in); got != want {
			t.Errorf("normalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}
