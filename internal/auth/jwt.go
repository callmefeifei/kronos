package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pstrr/kronos/internal/config"
	"github.com/pstrr/kronos/internal/model"
)

// Token types distinguish access tokens from refresh tokens.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// Claims is the custom JWT claims struct for Kronos.
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Type     string `json:"type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

var (
	ErrTokenExpired  = errors.New("token has expired")
	ErrTokenInvalid  = errors.New("token is invalid")
	ErrTokenMalformed = errors.New("token is malformed")
)

// GenerateAccessToken creates a signed JWT access token for the given user.
func GenerateAccessToken(user *model.User, authCfg config.AuthConfig) (string, error) {
	return generateToken(user, authCfg.JWTSecret, authCfg.AccessTokenTTL, TokenTypeAccess)
}

// GenerateRefreshToken creates a signed JWT refresh token for the given user.
func GenerateRefreshToken(user *model.User, authCfg config.AuthConfig) (string, error) {
	return generateToken(user, authCfg.JWTSecret, authCfg.RefreshTokenTTL, TokenTypeRefresh)
}

func generateToken(user *model.User, secret string, ttl time.Duration, tokenType string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		Type:     tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "kronos",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseToken validates and parses a JWT token string, returning the claims.
func ParseToken(tokenString string, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrTokenInvalid
		}
		return []byte(secret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, ErrTokenMalformed
		}
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}
