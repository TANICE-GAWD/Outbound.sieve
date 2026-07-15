package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)










func pushToClay(ctx context.Context, webhook, authToken string, rows []map[string]any) (int, error) {
	if webhook == "" {
		return 0, nil 
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
		time.Sleep(120 * time.Millisecond) 
	}
	return sent, nil
}
