package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)


func enrichEmployees(ctx context.Context, domain, key string) (int, bool) {
	if key == "" || domain == "" {
		return 0, false
	}
	u := fmt.Sprintf("https://companyenrichment.abstractapi.com/v2/?api_key=%s&domain=%s",
		url.QueryEscape(key), url.QueryEscape(domain))
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return 0, false
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, false
	}
	var out struct {
		EmployeeCount *int `json:"employee_count"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil || out.EmployeeCount == nil {
		return 0, false
	}
	return *out.EmployeeCount, true
}

// domainOf reduces a website field to the bare host Abstract expects
// ("https://www.acme.com/about" -> "acme.com").
func domainOf(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "www.")
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return s
}
