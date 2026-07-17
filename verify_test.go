package main

import (
	"context"
	"testing"
)

func TestClassifyDomain(t *testing.T) {
	cases := []struct {
		domain string
		hasMX  bool
		want   string
	}{
		{"acme-capital.com", true, "deliverable"},
		{"gmail.com", true, "risky"},
		{"mailinator.com", true, "risky"},
		{"deadbrand.example", false, "invalid"},
		{"acme-capital.com", false, "invalid"}, // corporate but no mail server still bounces
	}
	for _, c := range cases {
		if got, _ := classifyDomain(c.domain, c.hasMX); got != c.want {
			t.Errorf("classifyDomain(%q, hasMX=%v) = %q, want %q", c.domain, c.hasMX, got, c.want)
		}
	}
}

func TestMailDeliverability_Live(t *testing.T) {
	if testing.Short() {
		t.Skip("-short: skipping live DNS check")
	}
	if got, _ := mailDeliverability(context.Background(), "google.com"); got == "invalid" {
		t.Errorf("google.com should have a mail server, got %q", got)
	}
	// A domain that resolves to nothing must come back invalid, not error-swallowed as deliverable.
	if got, _ := mailDeliverability(context.Background(), "no-such-domain-xyz-9271.example"); got != "invalid" {
		t.Errorf("nonexistent domain = %q, want invalid", got)
	}
}
