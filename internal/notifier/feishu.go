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

// FeishuNotifier sends notifications to a Feishu (Lark) webhook as interactive cards.
type FeishuNotifier struct {
	webhookURL string
	client     *http.Client
}

// NewFeishuNotifier creates a FeishuNotifier from config.
func NewFeishuNotifier(cfg config.FeishuNotifierConfig) *FeishuNotifier {
	return &FeishuNotifier{
		webhookURL: cfg.WebhookURL,
		client:     &http.Client{},
	}
}

// Name returns the channel name.
func (f *FeishuNotifier) Name() string { return "feishu" }

// Send posts an interactive card message to the Feishu webhook.
func (f *FeishuNotifier) Send(ctx context.Context, n *Notification) error {
	card := buildFeishuCard(n)

	body, err := json.Marshal(card)
	if err != nil {
		return fmt.Errorf("marshal feishu card: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create feishu request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("feishu request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("feishu returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// buildFeishuCard constructs a Feishu interactive card message.
func buildFeishuCard(n *Notification) map[string]any {
	// Status color: green for success, red for fail
	color := "green"
	statusText := "SUCCESS"
	if n.Status != "success" {
		color = "red"
		statusText = "FAIL"
	}

	title := fmt.Sprintf("Kronos Task Notification - %s", statusText)
	outputSummary := truncate(n.Output, 500)

	// Build card elements
	elements := []map[string]any{
		{
			"tag": "div",
			"fields": []map[string]any{
				{
					"is_short": true,
					"text": map[string]any{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**Task:** %s", n.TaskName),
					},
				},
				{
					"is_short": true,
					"text": map[string]any{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**Status:** %s", statusText),
					},
				},
			},
		},
		{
			"tag": "div",
			"fields": []map[string]any{
				{
					"is_short": false,
					"text": map[string]any{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**Timestamp:** %s", n.Timestamp.Format("2006-01-02 15:04:05")),
					},
				},
			},
		},
		{
			"tag": "hr",
		},
		{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": fmt.Sprintf("**Output:**\n%s", outputSummary),
			},
		},
	}

	// Add error field if present
	if n.Error != "" {
		elements = append(elements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": fmt.Sprintf("**Error:**\n%s", truncate(n.Error, 500)),
			},
		})
	}

	return map[string]any{
		"msg_type": "interactive",
		"card": map[string]any{
			"header": map[string]any{
				"title": map[string]any{
					"tag":     "plain_text",
					"content": title,
				},
				"template": color,
			},
			"elements": elements,
		},
	}
}
