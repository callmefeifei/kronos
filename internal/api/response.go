package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response is the unified API response format.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// PagedData wraps paginated results.
type PagedData struct {
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

// Success sends a 200 response with code 0.
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

// Error sends an error response with the given HTTP status and application code.
func Error(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

// PagedSuccess sends a 200 response with paginated data.
func PagedSuccess(c *gin.Context, items interface{}, total int64, page, size int) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data: PagedData{
			Items: items,
			Total: total,
			Page:  page,
			Size:  size,
		},
	})
}
