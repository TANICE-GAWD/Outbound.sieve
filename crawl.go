package main

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ponytail: stdlib fetch + tag strip. Swap in Firecrawl when a JS-heavy site
// returns empty text (check: fetchSite returns < ~200 chars for a real site).

const maxBody = 512 << 10

// thinSite is the char count below which a page is effectively client-rendered
// and we're running on meta tags alone. Everything downstream gets weaker, so
// the pipeline says so out loud rather than quietly guessing.
const thinSite = 600

var (
	// Go's regexp is RE2: no backreferences, so each tag is spelled out.
	reScript = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>` +
		`|<style\b[^>]*>.*?</style>` +
		`|<noscript\b[^>]*>.*?</noscript>` +
		`|<svg\b[^>]*>.*?</svg>` +
		`|<!--.*?-->`)
	reTag   = regexp.MustCompile(`(?s)<[^>]+>`)
	reSpace = regexp.MustCompile(`\s+`)

	reTitle  = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reMeta   = regexp.MustCompile(`(?is)<meta\b[^>]*>`)
	reAttr   = regexp.MustCompile(`(?is)\b(name|property)\s*=\s*["']([^"']+)["']`)
	reCont   = regexp.MustCompile(`(?is)\bcontent\s*=\s*["']([^"']*)["']`)
	reLDJSON = regexp.MustCompile(`(?is)<script[^>]*type\s*=\s*["']application/ld\+json["'][^>]*>(.*?)</script>`)
)

// metaKeys are the head tags worth reading. Client-rendered sites (Lovable,
// Framer, most SPA builders) ship an empty body — the meta description is
// often the only real copy in the HTML, and it's usually the positioning line.
var metaKeys = map[string]bool{
	"description": true, "og:description": true, "og:title": true,
	"og:site_name": true, "twitter:description": true, "twitter:title": true,
	"keywords": true, "application-name": true,
}

// metaText pulls title, meta descriptions and JSON-LD out of raw HTML,
// deduped and in a stable order.
func metaText(raw string) string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(htmlText(s))
		if s == "" || seen[strings.ToLower(s)] {
			return
		}
		seen[strings.ToLower(s)] = true
		out = append(out, s)
	}

	if m := reTitle.FindStringSubmatch(raw); m != nil {
		add(m[1])
	}
	for _, tag := range reMeta.FindAllString(raw, -1) {
		key := reAttr.FindStringSubmatch(tag)
		val := reCont.FindStringSubmatch(tag)
		if key == nil || val == nil || !metaKeys[strings.ToLower(key[2])] {
			continue
		}
		add(val[1])
	}
	for _, m := range reLDJSON.FindAllStringSubmatch(raw, -1) {
		add(m[1])
	}
	return strings.Join(out, ". ")
}

var httpClient = &http.Client{Timeout: 12 * time.Second}

// fetchPage returns visible text from one URL, or "" if it can't be read.
func fetchPage(ctx context.Context, url string) string {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; outbound-sieve/1.0)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "html") && !strings.Contains(ct, "text") {
		return ""
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return ""
	}
	// Head first: on a client-rendered page it's all there is.
	return strings.TrimSpace(metaText(string(raw)) + "\n" + htmlText(string(raw)))
}

func htmlText(raw string) string {
	s := reScript.ReplaceAllString(raw, " ")
	s = reTag.ReplaceAllString(s, " ")
	s = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&quot;", `"`, "&#39;", "'", "&lt;", "<", "&gt;", ">").Replace(s)
	return strings.TrimSpace(reSpace.ReplaceAllString(s, " "))
}

// normalizeURL makes a bare domain usable ("acme.com" -> "https://acme.com").
func normalizeURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "https://" + s
	}
	return strings.TrimSuffix(s, "/")
}

// fetchSite reads the homepage plus a couple of high-signal subpages.
// Empty return means the site is dead or unreadable — callers drop it.
func fetchSite(ctx context.Context, site string) string {
	base := normalizeURL(site)
	if base == "" {
		return ""
	}
	home := fetchPage(ctx, base)
	if home == "" {
		return "" // dead domain: nothing downstream should trust it
	}
	var b strings.Builder
	b.WriteString(home)
	for _, p := range []string{"/about", "/product", "/pricing"} {
		if b.Len() > 24000 {
			break
		}
		if t := fetchPage(ctx, base+p); t != "" {
			b.WriteString("\n\n--- " + p + " ---\n")
			b.WriteString(t)
		}
	}
	out := b.String()
	if len(out) > 24000 {
		out = out[:24000] // Groq context is generous but prompts are cheaper short
	}
	return out
}
