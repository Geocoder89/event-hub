package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID    string `json:"sub"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	TokenType string `json:"typ"`
	JTI       string `json:"jti"`
	jwt.RegisteredClaims
}

type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewManager(secret string, accessTTL time.Duration, refreshTTL time.Duration) *Manager {
	return &Manager{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (m *Manager) GenerateAccessToken(userID, email, role string) (string, error) {
	now := time.Now().UTC()

	claims := Claims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		TokenType: "access",
		JTI:       uuid.NewString(),
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *Manager) GenerateRefreshToken(userID, email, role string) (raw string, jti string, expiresAt time.Time, err error) {
	now := time.Now().UTC()
	jti = uuid.NewString()
	expiresAt = now.Add(m.refreshTTL)

	claims := Claims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		TokenType: "refresh",
		JTI:       jti,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	raw, err = token.SignedString(m.secret)

	return
}

func (m *Manager) ParseAndValidate(tokenStr string) (claims *Claims, err error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		// Enforce HS256

		_, ok := t.Method.(*jwt.SigningMethodHMAC)

		if !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})

	if err != nil {
		return
	}
	claims, ok := token.Claims.(*Claims)

	if !ok || !token.Valid {
		err = errors.New("invalid token")
		return
	}
	return
}

func (m *Manager) VerifyAccessToken(tokenStr string) (*Claims, error) {
	claims, err := m.ParseAndValidate(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != "access" {
		return nil, errors.New("invalid token type")
	}
	return claims, nil
}

func (m *Manager) VerifyRefreshToken(tokenStr string) (*Claims, error) {
	claims, err := m.ParseAndValidate(tokenStr)

	if err != nil {
		return nil, err
	}

	if claims.TokenType != "refresh" {
		return nil, errors.New("invalid token type")
	}

	if claims.JTI == "" {
		return nil, errors.New("missing jti")
	}

	return claims, nil
}

// Deterministic HMAC hash (server-side pepper = JWT secret bytes).
// Store this in DB (never store raw refresh token).
func (m *Manager) HashRefreshToken(raw string) string {
	h := hmac.New(sha256.New, m.secret)
	h.Write([]byte(raw))
	return hex.EncodeToString(h.Sum(nil))
}
