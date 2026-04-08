package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pstrr/kronos/internal/auth"
	"github.com/pstrr/kronos/internal/config"
	"github.com/pstrr/kronos/internal/store"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	userStore *store.UserStore
	authCfg   config.AuthConfig
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(userStore *store.UserStore, authCfg config.AuthConfig) *AuthHandler {
	return &AuthHandler{
		userStore: userStore,
		authCfg:   authCfg,
	}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Login validates username/password and returns access + refresh tokens.
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid request: username and password are required")
		return
	}

	user, err := h.userStore.GetByUsername(req.Username)
	if err != nil {
		Error(c, http.StatusUnauthorized, 401, "invalid username or password")
		return
	}

	if user.Status != "active" {
		Error(c, http.StatusUnauthorized, 401, "user account is disabled")
		return
	}

	if !user.CheckPassword(req.Password) {
		Error(c, http.StatusUnauthorized, 401, "invalid username or password")
		return
	}

	accessToken, err := auth.GenerateAccessToken(user, h.authCfg)
	if err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to generate access token")
		return
	}

	refreshToken, err := auth.GenerateRefreshToken(user, h.authCfg)
	if err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to generate refresh token")
		return
	}

	Success(c, tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Refresh validates a refresh token and returns new access + refresh tokens.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid request: refresh_token is required")
		return
	}

	claims, err := auth.ParseToken(req.RefreshToken, h.authCfg.JWTSecret)
	if err != nil {
		Error(c, http.StatusUnauthorized, 401, "invalid or expired refresh token")
		return
	}

	if claims.Type != auth.TokenTypeRefresh {
		Error(c, http.StatusUnauthorized, 401, "invalid token type: expected refresh token")
		return
	}

	user, err := h.userStore.GetByID(claims.UserID)
	if err != nil {
		Error(c, http.StatusUnauthorized, 401, "user not found")
		return
	}

	if user.Status != "active" {
		Error(c, http.StatusUnauthorized, 401, "user account is disabled")
		return
	}

	accessToken, err := auth.GenerateAccessToken(user, h.authCfg)
	if err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to generate access token")
		return
	}

	refreshToken, err := auth.GenerateRefreshToken(user, h.authCfg)
	if err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to generate refresh token")
		return
	}

	Success(c, tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}
