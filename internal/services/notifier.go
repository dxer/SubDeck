package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var webhookHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
}

// SendWebhookNotification posts a JSON alert payload to a configured HTTP/HTTPS webhook.
// Supports generic webhooks, Discord/Slack-compatible `content`/`text` fields, and Bark/Telegram gateways.
func SendWebhookNotification(webhookURL, event, title, message string) error {
	raw := strings.TrimSpace(webhookURL)
	if raw == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	u, err := url.ParseRequestURI(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("invalid webhook URL: only http/https URLs are allowed")
	}

	payload := map[string]interface{}{
		"event":     event,
		"title":     title,
		"message":   message,
		"text":      fmt.Sprintf("[%s] %s", title, message),
		"content":   fmt.Sprintf("**%s**\n%s", title, message),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "SubDeck-Notifier/1.0")

	resp, err := webhookHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}
