package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// Webhook delivery reuses the fetcher's safe clients: DNS-resolved private-IP
// blocking, a 5-redirect cap and scheme allowlist. The private variant mirrors
// the system "allow private fetch" setting for LAN webhook receivers.
var (
	webhookSecureClient = newSafeHTTPClient(false, false)
	webhookPrivateClient = newSafeHTTPClient(false, true)
	webhookAllowPrivate  atomic.Bool
)

// SetWebhookAllowPrivateNetwork toggles whether webhook delivery may target
// private/LAN addresses; mirrors the system "allow private fetch" setting.
func SetWebhookAllowPrivateNetwork(allow bool) { webhookAllowPrivate.Store(allow) }

// SendWebhookNotification posts a JSON alert payload to a configured HTTP/HTTPS webhook.
// Supports generic webhooks, Discord/Slack-compatible `content`/`text` fields, and
// Bark (`body`)/Telegram gateways.
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
		"body":      message, // Bark native field
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

	client := webhookSecureClient
	if webhookAllowPrivate.Load() {
		client = webhookPrivateClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Bounded read so a hostile endpoint cannot stall the connection.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}
