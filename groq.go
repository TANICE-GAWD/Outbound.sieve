package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const groqModel = "llama-3.3-70b-versatile"

const groqURL = "https://api.groq.com/openai/v1/chat/completions"

var groqClient = &http.Client{Timeout: 90 * time.Second}

type groqMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqReq struct {
	Model          string          `json:"model"`
	Messages       []groqMsg       `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *map[string]any `json:"response_format,omitempty"`
}

type groqResp struct {
	Choices []struct {
		Message groqMsg `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func parseRetryAfter(h string) time.Duration {
	secs, err := strconv.ParseFloat(strings.TrimSpace(h), 64)
	if err != nil || secs <= 0 {
		return 5 * time.Second
	}
	if secs > 60 {
		secs = 60
	}
	return time.Duration(secs*float64(time.Second)) + 250*time.Millisecond
}

func groqJSON(ctx context.Context, system, user string, out any) error {
	key := os.Getenv("GROQ_API_KEY")
	if key == "" {
		return fmt.Errorf("GROQ_API_KEY not set")
	}
	format := map[string]any{"type": "json_object"}
	body, err := json.Marshal(groqReq{
		Model:          groqModel,
		Messages:       []groqMsg{{"system", system}, {"user", user}},
		Temperature:    0.4,
		ResponseFormat: &format,
	})
	if err != nil {
		return err
	}

	var lastErr error
	wait := time.Duration(0)
	for attempt := range 5 {
		if attempt > 0 {
			if wait <= 0 {
				wait = time.Duration(attempt) * 2 * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			wait = 0
		}
		req, err := http.NewRequestWithContext(ctx, "POST", groqURL, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Content-Type", "application/json")

		resp, err := groqClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		var gr groqResp
		err = json.NewDecoder(resp.Body).Decode(&gr)
		retryAfter := resp.Header.Get("retry-after")
		status := resp.StatusCode
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if gr.Error != nil {
			lastErr = fmt.Errorf("groq: %s", gr.Error.Message)
			if status == 429 {
				wait = parseRetryAfter(retryAfter)
				continue
			}
			if status >= 500 {
				continue
			}
			return lastErr
		}
		if len(gr.Choices) == 0 {
			lastErr = fmt.Errorf("groq: empty response")
			continue
		}
		content := strings.TrimSpace(gr.Choices[0].Message.Content)
		if err := json.Unmarshal([]byte(content), out); err != nil {
			lastErr = fmt.Errorf("groq: bad JSON: %w", err)
			continue
		}
		return nil
	}
	return lastErr
}
