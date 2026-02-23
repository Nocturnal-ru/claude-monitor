package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	SessionKey  string `json:"session_key"`
	OrgID       string `json:"org_id"`
	CfClearance string `json:"cf_clearance"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	cfg.SessionKey = strings.TrimSpace(cfg.SessionKey)
	cfg.OrgID = strings.TrimSpace(cfg.OrgID)
	cfg.CfClearance = strings.TrimSpace(cfg.CfClearance)

	if cfg.SessionKey == "" || strings.HasPrefix(cfg.SessionKey, "PASTE") {
		return nil, fmt.Errorf("session_key not configured")
	}
	if cfg.OrgID == "" || strings.HasPrefix(cfg.OrgID, "PASTE") {
		return nil, fmt.Errorf("org_id not configured")
	}

	return &cfg, nil
}

// saveFirefoxConfig writes (or updates) config.json with cookies from Firefox.
// If cfClearance is empty, preserves the existing cf_clearance value.
func saveFirefoxConfig(path, sessionKey, orgID, cfClearance string) error {
	// Preserve existing cf_clearance if the new one is empty
	if cfClearance == "" {
		var existing Config
		if data, err := os.ReadFile(path); err == nil {
			json.Unmarshal(data, &existing) //nolint — best-effort
		}
		cfClearance = existing.CfClearance
	}

	cfg := Config{
		SessionKey:  sessionKey,
		OrgID:       orgID,
		CfClearance: cfClearance,
	}

	// Ensure the directory exists
	if err := os.MkdirAll(strings.TrimSuffix(path, "config.json"), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func createTemplateConfig(path string) error {
	cfg := Config{
		SessionKey:  "PASTE_sessionKey_HERE",
		OrgID:       "PASTE_lastActiveOrg_HERE",
		CfClearance: "PASTE_cf_clearance_HERE",
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	dir := strings.TrimSuffix(path, "config.json")
	readme := `=== Claude Monitor - Setup ===

To get the values for config.json:

1. Open https://claude.ai in Firefox and log in

2. Press F12 (DevTools) -> tab "Storage" -> Cookies -> https://claude.ai

3. Find and copy these 3 cookies:
   - sessionKey      (starts with sk-ant-sid01-...)
   - lastActiveOrg   (UUID format)
   - cf_clearance     (Cloudflare token)

4. Paste all three values into config.json

Note: cf_clearance refreshes frequently (hours/days).
sessionKey refreshes roughly once a month.
If the app stops showing data - update the values.
`
	os.WriteFile(dir+"README-config.txt", []byte(readme), 0644)

	return os.WriteFile(path, data, 0644)
}
