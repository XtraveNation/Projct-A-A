package srv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Config is hot-reloadable. Persisted to disk so admin edits survive restarts.
type Config struct {
	BrandName        string   `json:"brand_name"`
	Tagline          string   `json:"tagline"`
	PublicURL        string   `json:"public_url"`
	AdminEmails      []string `json:"admin_emails"`
	OpenAIKey        string   `json:"openai_key"`
	OpenAIModel      string   `json:"openai_model"`
	OpenAIBaseURL    string   `json:"openai_base_url"`
	FreeMonthlyQuota int      `json:"free_monthly_quota"`
	ProPriceUSD      int      `json:"pro_price_usd"`
	ProCheckoutURL   string   `json:"pro_checkout_url"`
	LifetimeURL      string   `json:"lifetime_url"`
	RedeemSecret     string   `json:"redeem_secret"` // simple license keys for early sales
	AnalyticsTag     string   `json:"analytics_tag"` // injected raw HTML in <head>
	MarketingPrompt  string   `json:"marketing_prompt"`
	HostingNotes     string   `json:"hosting_notes"`
}

var configPath = func() string {
	if p := os.Getenv("JOBPILOT_CONFIG"); p != "" {
		return p
	}
	return "jobpilot.config.json"
}

func DefaultConfig() *Config {
	return &Config{
		BrandName:        envOr("BRAND_NAME", "JobPilot AI"),
		Tagline:          envOr("TAGLINE", "Land more interviews. AI tailors your resume to every job in 30 seconds."),
		PublicURL:        envOr("PUBLIC_URL", ""),
		AdminEmails:      splitCSV(envOr("ADMIN_EMAILS", "jhs_soaringstar@proton.me")),
		OpenAIKey:        os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:      envOr("OPENAI_MODEL", "gpt-4o-mini"),
		OpenAIBaseURL:    envOr("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		FreeMonthlyQuota: 3,
		ProPriceUSD:      19,
		ProCheckoutURL:   envOr("PRO_CHECKOUT_URL", ""),
		LifetimeURL:      envOr("LIFETIME_URL", ""),
		RedeemSecret:     envOr("REDEEM_SECRET", "jobpilot-launch"),
		AnalyticsTag:     "",
		MarketingPrompt:  "You are a punchy growth marketer. Keep copy short, benefit-led, no jargon.",
		HostingNotes:     "",
	}
}

func LoadConfig() *Config {
	c := DefaultConfig()
	data, err := os.ReadFile(configPath())
	if err != nil {
		return c
	}
	var disk Config
	if err := json.Unmarshal(data, &disk); err != nil {
		return c
	}
	// Merge disk over defaults.
	if disk.BrandName != "" {
		c.BrandName = disk.BrandName
	}
	if disk.Tagline != "" {
		c.Tagline = disk.Tagline
	}
	if disk.PublicURL != "" {
		c.PublicURL = disk.PublicURL
	}
	if len(disk.AdminEmails) > 0 {
		c.AdminEmails = disk.AdminEmails
	}
	if disk.OpenAIKey != "" {
		c.OpenAIKey = disk.OpenAIKey
	}
	if disk.OpenAIModel != "" {
		c.OpenAIModel = disk.OpenAIModel
	}
	if disk.OpenAIBaseURL != "" {
		c.OpenAIBaseURL = disk.OpenAIBaseURL
	}
	if disk.FreeMonthlyQuota > 0 {
		c.FreeMonthlyQuota = disk.FreeMonthlyQuota
	}
	if disk.ProPriceUSD > 0 {
		c.ProPriceUSD = disk.ProPriceUSD
	}
	if disk.ProCheckoutURL != "" {
		c.ProCheckoutURL = disk.ProCheckoutURL
	}
	if disk.LifetimeURL != "" {
		c.LifetimeURL = disk.LifetimeURL
	}
	if disk.RedeemSecret != "" {
		c.RedeemSecret = disk.RedeemSecret
	}
	c.AnalyticsTag = disk.AnalyticsTag
	if disk.MarketingPrompt != "" {
		c.MarketingPrompt = disk.MarketingPrompt
	}
	c.HostingNotes = disk.HostingNotes
	return c
}

func SaveConfig(c *Config) error {
	p := configPath()
	if dir := filepath.Dir(p); dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
