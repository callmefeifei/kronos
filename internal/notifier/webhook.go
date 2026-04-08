package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pstrr/kronos/internal/config"
)

// webhookPayload is the JSON body sent to webhook endpoints.
type webhookPayload struct {
	TaskName  string `json:"task_name"`
	Status    string `json:"status"`
	Output    string `json:"output"`
	Error     string `json:"error"`
	Timestamp string `json:"timestamp"`
}

// WebhookNotifier sends notifications to a generic webhook URL.
type WebhookNotifier struct {
	url     string
	headers map[string]string
	client  *http.Client
}

// NewWebhookNotifier creates a WebhookNotifier from config.
func NewWebhookNotifier(cfg config.WebhookNotifierConfig) *WebhookNotifier {
	return &WebhookNotifier{
		url:     cfg.URL,
		headers: cfg.Headers,
		client:  &http.Client{},
	}
}

// Name returns the channel name.
func (w *WebhookNotifier) Name() string { return "webhook" }

// Send posts a JSON payload to the configured webhook URL.
func (w *WebhookNotifier) Send(ctx context.Context, n *Notification) error {
	payload := webhookPayload{
		TaskName:  n.TaskName,
		Status:    n.Status,
		Output:    n.Output,
		Error:     n.Error,
		Timestamp: n.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Apply custom headers
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
