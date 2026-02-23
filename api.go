package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
}

func fetchUsage(cfg *Config) (*UsageResponse, error) {
	url := fmt.Sprintf("https://claude.ai/api/organizations/%s/usage", cfg.OrgID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Собираем cookie-строку как в браузере
	cookieStr := fmt.Sprintf("sessionKey=%s", cfg.SessionKey)
	if cfg.CfClearance != "" {
		cookieStr += fmt.Sprintf("; cf_clearance=%s", cfg.CfClearance)
	}

	req.Header.Set("Cookie", cookieStr)
	// User-Agent должен совпадать с браузером, из которого взяты cookies
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:135.0) Gecko/20100101 Firefox/135.0")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Referer", "https://claude.ai/")
	req.Header.Set("Origin", "https://claude.ai")

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
