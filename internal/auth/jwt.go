package auth

import (
	"time"

	"pie/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	TokenUse string `json:"token_use"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret        string
	expire        time.Duration
	refreshExpire time.Duration
}

func NewJWTManager(cfg config.JWTConfig) *JWTManager {
	return &JWTManager{
		secret:        cfg.Secret,
		expire:        time.Duration(cfg.ExpireHours) * time.Hour,
		refreshExpire: time.Duration(cfg.RefreshExpireHours) * time.Hour,
	}
}

func (m *JWTManager) GenerateAccessToken(userID int64, username, role string) (string, error) {
	return m.signToken(userID, username, role, "access", m.expire)
}

func (m *JWTManager) GenerateRefreshToken(userID int64, username, role string) (string, error) {
	return m.signToken(userID, username, role, "refresh", m.refreshExpire)
}

func (m *JWTManager) signToken(userID int64, username, role, tokenUse string, expire time.Duration) (string, error) {
	now := time.Now()

	claims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		TokenUse: tokenUse,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expire)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "pie",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.secret))
}

func (m *JWTManager) VerifyToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrInvalidType
		}
		return []byte(m.secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}

func (m *JWTManager) VerifyAccessToken(tokenStr string) (*Claims, error) {
	claims, err := m.VerifyToken(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.TokenUse != "access" {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}

func (m *JWTManager) VerifyRefreshToken(tokenStr string) (*Claims, error) {
	claims, err := m.VerifyToken(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.TokenUse != "refresh" {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return claims, nil
}
