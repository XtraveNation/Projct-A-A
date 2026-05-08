package srv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type AIClient struct {
	cfg    *Config
	client *http.Client
}

func NewAIClient(c *Config) *AIClient {
	return &AIClient{cfg: c, client: &http.Client{Timeout: 90 * time.Second}}
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatReq struct {
	Model       string    `json:"model"`
	Messages    []chatMsg `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type chatResp struct {
	Choices []struct {
		Message chatMsg `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (a *AIClient) Complete(ctx context.Context, system, user string) (string, error) {
	if a.cfg.OpenAIKey == "" {
		return fallback(system, user), nil
	}
	reqBody, _ := json.Marshal(chatReq{
		Model:       a.cfg.OpenAIModel,
		Temperature: 0.5,
		Messages: []chatMsg{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	url := strings.TrimRight(a.cfg.OpenAIBaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.OpenAIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var parsed chatResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("ai parse: %w (%s)", err, string(body))
	}
	if parsed.Error != nil {
		return "", errors.New(parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("no choices")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

// fallback runs when no API key is set, so the demo still works.
func fallback(system, user string) string {
	return "(Demo mode — add OPENAI_API_KEY in /admin/config to enable real AI output.)\n\nPrompt summary:\n" + truncate(user, 800) + "\n\nThis is placeholder content describing the structure of the response you'd see in production."
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
