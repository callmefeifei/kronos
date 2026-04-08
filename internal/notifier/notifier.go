package notifier

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/pstrr/kronos/internal/config"
	"github.com/pstrr/kronos/internal/model"
	"github.com/pstrr/kronos/internal/store"
)

// Notification holds the data to be sent through a notifier channel.
type Notification struct {
	TaskName  string
	Status    string // success, fail
	Output    string
	Error     string
	TaskID    int64
	TaskRunID int64
	Timestamp time.Time
}

// Notifier is the interface that notification channels must implement.
type Notifier interface {
	// Send delivers a notification. It returns an error if the send fails.
	Send(ctx context.Context, n *Notification) error
	// Name returns the channel name (e.g. "feishu", "webhook").
	Name() string
}

// Manager holds configured notifiers and dispatches notifications by channel name.
type Manager struct {
	notifiers map[string]Notifier
	store     *store.NotificationStore
}

// NewManager creates a Manager from the notifier config and notification store.
func NewManager(cfg config.NotifierConfig, ns *store.NotificationStore) *Manager {
	m := &Manager{
		notifiers: make(map[string]Notifier),
		store:     ns,
	}

	if cfg.Feishu.Enabled && cfg.Feishu.WebhookURL != "" {
		m.notifiers["feishu"] = NewFeishuNotifier(cfg.Feishu)
	}
	if cfg.Webhook.Enabled && cfg.Webhook.URL != "" {
		m.notifiers["webhook"] = NewWebhookNotifier(cfg.Webhook)
	}

	return m
}

// Send dispatches a notification to all configured channels.
// Each send is non-blocking: it fires in a goroutine.
// Errors are logged and stored but do not affect the caller.
func (m *Manager) Send(n *Notification) {
	for _, notifier := range m.notifiers {
		go m.sendAndRecord(notifier, n)
	}
}

// SendTo dispatches a notification to a specific channel by name.
// The send is non-blocking.
func (m *Manager) SendTo(channel string, n *Notification) {
	notifier, ok := m.notifiers[channel]
	if !ok {
		slog.Warn("notifier channel not found", "channel", channel)
		return
	}
	go m.sendAndRecord(notifier, n)
}

// sendAndRecord sends the notification via the given notifier and records
// the result to the notification store.
func (m *Manager) sendAndRecord(notifier Notifier, n *Notification) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build payload for storage
	payload, _ := json.Marshal(n)

	record := &model.Notification{
		TaskID:    n.TaskID,
		TaskRunID: n.TaskRunID,
		Channel:   notifier.Name(),
		Payload:   string(payload),
	}

	err := notifier.Send(ctx, n)
	if err != nil {
		record.Status = "failed"
		record.Response = err.Error()
		slog.Error("notification send failed",
			"channel", notifier.Name(),
			"task_id", n.TaskID,
			"error", err,
		)
	} else {
		record.Status = "sent"
		slog.Info("notification sent",
			"channel", notifier.Name(),
			"task_id", n.TaskID,
		)
	}

	if m.store != nil {
		if storeErr := m.store.Create(record); storeErr != nil {
			slog.Error("failed to store notification record",
				"channel", notifier.Name(),
				"error", storeErr,
			)
		}
	}
}

// HasChannels returns true if any notifier channels are configured.
func (m *Manager) HasChannels() bool {
	return len(m.notifiers) > 0
}

// truncate cuts s to at most maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
