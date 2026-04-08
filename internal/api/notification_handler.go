package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pstrr/kronos/internal/store"
)

// NotificationHandler handles notification endpoints.
type NotificationHandler struct {
	notificationStore *store.NotificationStore
}

// NewNotificationHandler creates a new NotificationHandler.
func NewNotificationHandler(notificationStore *store.NotificationStore) *NotificationHandler {
	return &NotificationHandler{notificationStore: notificationStore}
}

// List returns a paginated list of notifications.
func (h *NotificationHandler) List(c *gin.Context) {
	page, size := parsePagination(c)

	items, total, err := h.notificationStore.List(page, size)
	if err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to list notifications")
		return
	}

	PagedSuccess(c, items, total, page, size)
}
