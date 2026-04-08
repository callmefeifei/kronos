package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pstrr/kronos/internal/config"
	"github.com/pstrr/kronos/internal/store"
)

// Context keys for storing user info in gin.Context.
const (
	ContextKeyUserID   = "user_id"
	ContextKeyUsername = "username"
	ContextKeyRole     = "role"
)

// Response is the unified API response format.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func abortWithError(c *gin.Context, httpStatus int, code int, message string) {
	c.AbortWithStatusJSON(httpStatus, Response{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

// AuthRequired returns a Gin middleware that validates JWT access tokens.
// It extracts the token from the Authorization header, validates it,
// checks that the user is still active, and sets user info in the context.
func AuthRequired(authCfg config.AuthConfig, userStore *store.UserStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			abortWithError(c, http.StatusUnauthorized, 401, "authorization header is required")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			abortWithError(c, http.StatusUnauthorized, 401, "authorization header must be: Bearer <token>")
			return
		}

		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" {
			abortWithError(c, http.StatusUnauthorized, 401, "token is required")
			return
		}

		// Parse and validate token
		claims, err := ParseToken(tokenString, authCfg.JWTSecret)
		if err != nil {
			msg := "invalid token"
			if errors.Is(err, ErrTokenExpired) {
				msg = "token has expired"
			} else if errors.Is(err, ErrTokenMalformed) {
				msg = "malformed token"
			}
			abortWithError(c, http.StatusUnauthorized, 401, msg)
			return
		}

		// Only accept access tokens (not refresh tokens)
		if claims.Type != TokenTypeAccess {
			abortWithError(c, http.StatusUnauthorized, 401, "invalid token type")
			return
		}

		// CLI admin tokens use user_id=0; skip DB lookup and grant admin access.
		if claims.UserID == 0 && claims.Role == "admin" {
			c.Set(ContextKeyUserID, int64(0))
			c.Set(ContextKeyUsername, claims.Username)
			c.Set(ContextKeyRole, "admin")
			c.Next()
			return
		}

		// Check that user still exists and is active
		user, err := userStore.GetByID(claims.UserID)
		if err != nil {
			slog.Warn("auth: user not found", "user_id", claims.UserID, "error", err)
			abortWithError(c, http.StatusUnauthorized, 401, "user not found")
			return
		}

		if user.Status != "active" {
			abortWithError(c, http.StatusUnauthorized, 401, "user account is disabled")
			return
		}

		// Set user info in context
		c.Set(ContextKeyUserID, user.ID)
		c.Set(ContextKeyUsername, user.Username)
		c.Set(ContextKeyRole, user.Role)

		c.Next()
	}
}

// AdminRequired returns a Gin middleware that requires the user to have the "admin" role.
// It must be used after AuthRequired in the middleware chain.
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get(ContextKeyRole)
		if !exists {
			abortWithError(c, http.StatusUnauthorized, 401, "authentication required")
			return
		}

		if role.(string) != "admin" {
			abortWithError(c, http.StatusForbidden, 403, "admin access required")
			return
		}

		c.Next()
	}
}

// CurrentUser holds the authenticated user's info extracted from gin.Context.
type CurrentUser struct {
	ID       int64
	Username string
	Role     string
}

// GetCurrentUser extracts the authenticated user's info from gin.Context.
// Returns nil if no user is set (i.e., the request is not authenticated).
func GetCurrentUser(c *gin.Context) *CurrentUser {
	userID, exists := c.Get(ContextKeyUserID)
	if !exists {
		return nil
	}

	username, _ := c.Get(ContextKeyUsername)
	role, _ := c.Get(ContextKeyRole)

	return &CurrentUser{
		ID:       userID.(int64),
		Username: username.(string),
		Role:     role.(string),
	}
}
