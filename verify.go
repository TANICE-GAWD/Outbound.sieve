package main

import (
	"context"
	"net"
	"strings"
	"time"
)

// mailDeliverability answers one question at the domain level: could an address
// at this domain receive mail at all? It is a pre-send filter, not per-mailbox
// proof — we never claim a specific inbox exists.
//
//	deliverable — has a mail server, corporate domain
//	risky       — has a mail server but is a free/throwaway domain
//	invalid     — no mail server at all; anything sent here bounces
func mailDeliverability(ctx context.Context, domain string) (status, reason string) {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" {
		return "invalid", "no domain"
	}
	// Short DNS deadline so one slow lookup can't stall the batch.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var r net.Resolver
	mx, err := r.LookupMX(ctx, domain)
	hasMX := err == nil && len(mx) > 0
	if !hasMX {
		// Some domains accept mail on their A record with no MX. Rare, but
		// checking it keeps us from wrongly failing a live domain.
		if ips, aerr := r.LookupHost(ctx, domain); aerr == nil && len(ips) > 0 {
			hasMX = true
		}
	}
	return classifyDomain(domain, hasMX)
}

// classifyDomain is the network-free decision so it can be unit-tested without DNS.
func classifyDomain(domain string, hasMX bool) (status, reason string) {
	if !hasMX {
		return "invalid", "no mail server — bounces"
	}
	if disposableDomains[domain] {
		return "risky", "disposable domain"
	}
	if freeProviders[domain] {
		return "risky", "free mailbox, not a company domain"
	}
	return "deliverable", "mail server present"
}

// ponytail: static sets, swap for a maintained list if it ever matters.
var freeProviders = map[string]bool{
	"gmail.com": true, "googlemail.com": true, "yahoo.com": true,
	"outlook.com": true, "hotmail.com": true, "live.com": true,
	"aol.com": true, "icloud.com": true, "proton.me": true,
	"protonmail.com": true, "gmx.com": true, "mail.com": true,
	"yandex.com": true, "zoho.com": true,
}

var disposableDomains = map[string]bool{
	"mailinator.com": true, "guerrillamail.com": true, "10minutemail.com": true,
	"temp-mail.org": true, "throwawaymail.com": true, "yopmail.com": true,
	"getnada.com": true, "trashmail.com": true, "sharklasers.com": true,
	"tempmail.com": true, "dispostable.com": true, "maildrop.cc": true,
}
