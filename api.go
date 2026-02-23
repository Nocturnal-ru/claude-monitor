package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type UsageBucket struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    string  `json:"resets_at"`
}

type UsageResponse struct {
	FiveHour       UsageBucket  `json:"five_hour"`
	SevenDay       UsageBucket  `json:"seven_day"`
	SevenDayOpus   *UsageBucket `json:"seven_day_opus"`
	SevenDaySonnet *UsageBucket `json:"seven_day_sonnet"`
	ExtraUsage     *struct {
		IsEnabled    bool     `json:"is_enabled"`
		MonthlyLimit *float64 `json:"monthly_limit"`
		UsedCredits  *float64 `json:"used_credits"`
		Utilization  *float64 `json:"utilization"`
	} `json:"extra_usage"`
}

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        1,
		MaxIdleConnsPerHost: 1,
		IdleConnTimeout:     90 * time.Second,
	},
}

var retryDelays = []time.Duration{10 * time.Second, 30 * time.Second, 60 * time.Second}

func isRetryable(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "HTTP 403") ||
		strings.Contains(msg, "HTTP 5") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "EOF")
}

func fetchUsage(cfg *Config) (*UsageResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= len(retryDelays); attempt++ {
		if attempt > 0 {
			delay := retryDelays[attempt-1]
			log.Printf("Retry %d/%d after %v (error: %v)", attempt, len(retryDelays), delay, lastErr)
			time.Sleep(delay)
		}
		usage, err := doFetch(cfg)
		if err == nil {
			return usage, nil
		}
		lastErr = err
		if !isRetryable(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("all %d attempts failed: %w", len(retryDelays)+1, lastErr)
}

func doFetch(cfg *Config) (*UsageResponse, error) {
	url := fmt.Sprintf("https://claude.ai/api/organizations/%s/usage", cfg.OrgID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	cookieStr := fmt.Sprintf("sessionKey=%s", cfg.SessionKey)
	if cfg.CfClearance != "" {
		cookieStr += fmt.Sprintf("; cf_clearance=%s", cfg.CfClearance)
	}

	req.Header.Set("Cookie", cookieStr)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:135.0) Gecko/20100101 Firefox/135.0")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Referer", "https://claude.ai/")
	req.Header.Set("Origin", "https://claude.ai")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-GPC", "1")
	req.Header.Set("DNT", "1")
	req.Header.Set("TE", "trailers")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != 200 {
		bodyStr := string(body)
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "..."
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, bodyStr)
	}

	var usage UsageResponse
	if err := json.Unmarshal(body, &usage); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	return &usage, nil
}
