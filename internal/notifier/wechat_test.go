package notifier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pstrr/kronos/internal/config"
)

func TestWechatNotifierUsesPerTaskToken(t *testing.T) {
	var gotToken, gotTopic, gotMessage string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		gotToken = r.FormValue("token")
		gotTopic = r.FormValue("topic")
		gotMessage = r.FormValue("message")
		_, _ = w.Write([]byte(`{"status":108200,"message":"发送成功!"}`))
	}))
	defer server.Close()

	n := NewWechatNotifier(config.WechatNotifierConfig{
		URL:   server.URL,
		Token: "global-token",
	})

	err := n.Send(context.Background(), &Notification{
		TaskName:  "Sleep reminder",
		Status:    "success",
		Output:    "早点睡觉",
		Timestamp: time.Now(),
		Meta: map[string]string{
			"wechat_token": "user-token",
			"wechat_topic": "user-topic",
		},
	})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if gotToken != "user-token" {
		t.Fatalf("token = %q, want user-token", gotToken)
	}
	if gotTopic != "user-topic" {
		t.Fatalf("topic = %q, want user-topic", gotTopic)
	}
	if gotMessage != "早点睡觉" {
		t.Fatalf("message = %q, want reminder output", gotMessage)
	}
}
