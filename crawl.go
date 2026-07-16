package main

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const maxBody = 512 << 10

const thinSite = 600

var (
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

var metaKeys = map[string]bool{
	"description": true, "og:description": true, "og:title": true,
	"og:site_name": true, "twitter:description": true, "twitter:title": true,
	"keywords": true, "application-name": true,
}

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

	return strings.TrimSpace(metaText(string(raw)) + "\n" + htmlText(string(raw)))
}

func htmlText(raw string) string {
	s := reScript.ReplaceAllString(raw, " ")
	s = reTag.ReplaceAllString(s, " ")
	s = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&quot;", `"`, "&#39;", "'", "&lt;", "<", "&gt;", ">").Replace(s)
	return strings.TrimSpace(reSpace.ReplaceAllString(s, " "))
}

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

func fetchSite(ctx context.Context, site string) string {
	base := normalizeURL(site)
	if base == "" {
		return ""
	}
	home := fetchPage(ctx, base)
	if home == "" {
		return ""
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
		out = out[:24000]
	}
	return out
}
