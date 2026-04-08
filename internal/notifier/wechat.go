package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/pstrr/kronos/internal/config"
)

// WechatNotifier sends notifications to a WeChat push service (e.g. api.ossec.cn).
type WechatNotifier struct {
	url    string
	token  string
	client *http.Client
}

// NewWechatNotifier creates a WechatNotifier from config.
func NewWechatNotifier(cfg config.WechatNotifierConfig) *WechatNotifier {
	return &WechatNotifier{
		url:    cfg.URL,
		token:  cfg.Token,
		client: &http.Client{},
	}
}

// Name returns the channel name.
func (w *WechatNotifier) Name() string { return "wechat" }

// Send posts a form-urlencoded message to the WeChat push API.
func (w *WechatNotifier) Send(ctx context.Context, n *Notification) error {
	topic := fmt.Sprintf("Kronos: %s", n.TaskName)
	message := buildWechatMessage(n)

	form := url.Values{
		"token":   {w.token},
		"topic":   {topic},
		"message": {message},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create wechat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("wechat request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// API returns {"status": 108200, "message": "发送成功!"}
	var result struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse wechat response: %w", err)
	}
	if !strings.Contains(result.Message, "成功") {
		return fmt.Errorf("wechat send failed: %s", string(body))
	}

	return nil
}

// buildWechatMessage formats the notification content for WeChat.
func buildWechatMessage(n *Notification) string {
	// For remind tasks, the Output IS the reminder message — send it directly.
	if n.Output != "" && n.Error == "" {
		return n.Output
	}

	// For other tasks, build a structured message.
	status := "SUCCESS"
	if n.Status != "success" {
		status = "FAIL"
	}

	msg := fmt.Sprintf("[%s] %s\n%s", status, n.TaskName, n.Timestamp.Format("2006-01-02 15:04:05"))
	if n.Output != "" {
		msg += fmt.Sprintf("\n\n%s", truncate(n.Output, 500))
	}
	if n.Error != "" {
		msg += fmt.Sprintf("\n\nError: %s", truncate(n.Error, 300))
	}
	return msg
}
