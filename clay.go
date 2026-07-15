package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// pushToClay POSTs one row per request to a Clay webhook source.
//
// ponytail: sequential, one row per POST. Clay's webhook takes a single JSON
// object per request; batching isn't offered. Go concurrent if 200+ rows ever
// makes this the slow step (it won't on the free tier's 200-row table cap).
//
// Known ceiling: a webhook source accepts 50,000 submissions lifetime, and the
// count survives row deletion. A long-lived demo template eventually needs a
// fresh webhook URL.
func pushToClay(ctx context.Context, webhook, authToken string, rows []map[string]any) (int, error) {
	if webhook == "" {
		return 0, nil // no webhook configured: export still works
	}
	sent := 0
	for i, row := range rows {
		body, err := json.Marshal(row)
		if err != nil {
			return sent, err
		}
		req, err := http.NewRequestWithContext(ctx, "POST", webhook, bytes.NewReader(body))
		if err != nil {
			return sent, err
		}
		req.Header.Set("Content-Type", "application/json")
		if authToken != "" {
			req.Header.Set("x-clay-webhook-auth", authToken)
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return sent, fmt.Errorf("clay row %d: %w", i+1, err)
		}
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			return sent, fmt.Errorf("clay row %d: HTTP %d", i+1, resp.StatusCode)
		}
		sent++
		time.Sleep(120 * time.Millisecond) // be polite to the free tier
	}
	return sent, nil
}
