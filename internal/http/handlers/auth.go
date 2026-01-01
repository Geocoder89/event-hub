package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/geocoder89/eventhub/internal/auth"
	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/domain/user"
	"github.com/geocoder89/eventhub/internal/repo/postgres"
	"github.com/geocoder89/eventhub/internal/security"
	"github.com/gin-gonic/gin"
)

type UserReader interface {
	GetByEmail(ctx context.Context, email string) (user.User, error)
}

type UserWriter interface {
	Create(ctx context.Context, email, passwordHash, name, role string) (user.User, error)
}

type postgresTx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type RefreshTokenStore interface {
	BeginTx(ctx context.Context) (postgresTx, error)
	Create(ctx context.Context, tx postgresTx, row postgres.RefreshTokenRow) error
	GetForUpdate(ctx context.Context, tx postgresTx, id string) (postgres.RefreshTokenRow, error)
	Revoke(ctx context.Context, tx postgresTx, id string, replacedBy *string) error
	RevokeAllForUser(ctx context.Context, tx postgresTx, userID string) error
}

type AuthHandler struct {
	users        UserReader
	userWriter   UserWriter
	jwt          *auth.Manager
	refreshStore *postgres.RefreshTokensRepo
	cfg          config.Config
}

func NewAuthHandler(users UserReader, userWriter UserWriter, jwtManager *auth.Manager, refreshStore *postgres.RefreshTokensRepo, cfg config.Config) *AuthHandler {
	return &AuthHandler{
		users:        users,
		userWriter:   userWriter,
		jwt:          jwtManager,
		refreshStore: refreshStore,
		cfg:          cfg,
	}
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}
type SignUpRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name" binding:"required"`
}

func (h *AuthHandler) SignUp(ctx *gin.Context) {
	var req SignUpRequest

	if !BindJSON(ctx, &req) {
		return
	}

	cctx, cancel := config.WithTimeout(3 * time.Second)

	defer cancel()

	hash, err := security.HashPassword(req.Password)

	if err != nil {
		RespondInternal(ctx, "Could not create user")
		return
	}

	// default role for new users

	role := "user"

	u, err := h.userWriter.Create(cctx, req.Email, hash, req.Name, role)

	if err != nil {
		if err == postgres.ErrEmailAlreadyUsed {
			RespondBadRequest(ctx, "email_taken", "Email is already in use.")
			return
		}

		RespondInternal(ctx, "Could not create user")
		return
	}

	accessToken, err := h.jwt.GenerateAccessToken(u.ID, u.Email, u.Role)

	if err != nil {
		RespondInternal(ctx, "Could not generate access token")
		return
	}

	rawRefreshToken, jti, expiresAt, err := h.jwt.GenerateRefreshToken(u.ID, u.Email, u.Role)

	if err != nil {
		RespondInternal(ctx, "Could not generate refresh token")
		return
	}

	err = h.storeRefreshToken(cctx, u.ID, jti, rawRefreshToken, expiresAt)

	if err != nil {
		RespondInternal(ctx, "Could not create session")
		return
	}

	h.setRefreshCookie(ctx, rawRefreshToken, expiresAt)

	ctx.JSON(http.StatusCreated, gin.H{
		"accessToken": accessToken,
	})
}

func (h *AuthHandler) Login(ctx *gin.Context) {
	var req LoginRequest

	if !BindJSON(ctx, &req) {
		return
	}
	// short timeout for DB lookup
	cctx, cancel := config.WithTimeout(2 * time.Second)
	defer cancel()

	foundUser, err := h.users.GetByEmail(cctx, req.Email)
	if err != nil {
		RespondUnAuthorized(ctx, "invalid_credentials", "Email or password is incorrect.")
		return
	}

	err = security.CheckPassword(foundUser.PasswordHash, req.Password)

	if err != nil {
		RespondUnAuthorized(ctx, "invalid_credentials", "Email or password is incorrect.")
		return
	}

	accessToken, err := h.jwt.GenerateAccessToken(foundUser.ID, foundUser.Email, foundUser.Role)

	if err != nil {
		RespondInternal(ctx, "Could not generate access token")
		return
	}

	rawRefreshToken, jti, expiresAt, err := h.jwt.GenerateRefreshToken(foundUser.ID, foundUser.Email, foundUser.Role)

	if err != nil {
		RespondInternal(ctx, "Could not generate refresh token")
		return
	}

	if err := h.storeRefreshToken(cctx, foundUser.ID, jti, rawRefreshToken, expiresAt); err != nil {
		RespondInternal(ctx, "Could not create session")
		return
	}

	h.setRefreshCookie(ctx, rawRefreshToken, expiresAt)

	ctx.JSON(http.StatusOK, gin.H{
		"accessToken": accessToken,
	})
}

// Refresh Token functions

func (h *AuthHandler) Refresh(ctx *gin.Context) {
	raw, err := ctx.Cookie(h.refreshCookieName())

	if err != nil || raw == "" {
		RespondUnAuthorized(ctx, "no_refresh", "Missing refresh token")
		return
	}

	claims, err := h.jwt.VerifyRefreshToken(raw)

	if err != nil {
		RespondUnAuthorized(ctx, "invalid_refresh", "Invalid refresh token")
		return
	}

	// rotation with a tx with row lock

	cctx, cancel := config.WithTimeout(3 * time.Second)

	defer cancel()

	tx, err := h.refreshStore.BeginTx(cctx)

	if err != nil {
		RespondInternal(ctx, "Could not refresh session")
		return
	}

	defer func() { _ = tx.Rollback(cctx) }()

	row, err := h.refreshStore.GetForUpdate(cctx, tx, claims.JTI)

	if err != nil {
		RespondUnAuthorized(ctx, "invalid_refresh", "Invalid refresh token")
		return
	}

	//  check if it is revoked/expired

	if row.RevokedAt != nil {
		RespondUnAuthorized(ctx, "invalid_refresh", "Invalid refresh token")
		return
	}

	if time.Now().UTC().After(row.ExpiresAt) {
		RespondUnAuthorized(ctx, "expired_refresh", "Refresh token expired.")
		return
	}

	// verify hash matches the presented token (prevents token substitution)

	if row.TokenHash != h.jwt.HashRefreshToken(raw) {
		RespondUnAuthorized(ctx, "invalid_refresh", "Invalid refresh token.")
		return
	}

	// if these checks pass issue a new refresh token

	newRaw, newJTI, newExpiresAt, err := h.jwt.GenerateRefreshToken(row.UserID, claims.Email, claims.Role)
	if err != nil {
		RespondInternal(ctx, "Could not refresh session")
		return
	}

	// revoke old, insert new

	err = h.refreshStore.Revoke(cctx, tx, row.ID, &newJTI)

	if err != nil {
		RespondInternal(ctx, "Could not refresh session")
		return
	}

	newRow := postgres.RefreshTokenRow{
		ID:        newJTI,
		UserID:    row.UserID,
		TokenHash: h.jwt.HashRefreshToken(newRaw),
		ExpiresAt: newExpiresAt,
		CreatedAt: time.Now().UTC(),
	}

	err = h.refreshStore.Create(cctx, tx, newRow)

	if err != nil {
		fmt.Println(err, "this is the store creation error")
		RespondInternal(ctx, "Could not refresh session")
		return
	}

	err = tx.Commit(cctx)

	if err != nil {
		fmt.Println(err, "this is the commit error")
		RespondInternal(ctx, "Could not refresh session")
		return
	}

	// Generate a new access token
	accessToken, err := h.jwt.GenerateAccessToken(row.UserID, claims.Email, claims.Role)
	if err != nil {
		RespondInternal(ctx, "Could not generate access token")
		return
	}

	h.setRefreshCookie(ctx, newRaw, newExpiresAt)

	ctx.JSON(http.StatusOK, gin.H{
		"accessToken": accessToken,
	})
}

// Logout function

func (h *AuthHandler) Logout(ctx *gin.Context) {
	raw, err := ctx.Cookie(h.refreshCookieName())

	if err != nil || raw == "" {
		// still clear cookie to be safe
		h.clearRefreshCookie(ctx)
		ctx.Status(http.StatusNoContent)
		return
	}

	// verify the token and then clear
	claims, err := h.jwt.VerifyRefreshToken(raw)
	if err != nil {
		h.clearRefreshCookie(ctx)
		ctx.Status(http.StatusNoContent)
		return
	}

	cctx, cancel := config.WithTimeout(3 * time.Second)
	defer cancel()

	tx, err := h.refreshStore.BeginTx(cctx)
	if err != nil {
		h.clearRefreshCookie(ctx)
		ctx.Status(http.StatusNoContent)
		return
	}
	defer func() { _ = tx.Rollback(cctx) }()

	// revoke that one token (idempotent)
	_ = h.refreshStore.Revoke(cctx, tx, claims.JTI, nil)
	_ = tx.Commit(cctx)

	h.clearRefreshCookie(ctx)
	ctx.Status(http.StatusNoContent)
}

// Helper functions

func (h *AuthHandler) storeRefreshToken(ctx context.Context, userID, jti, raw string, expiresAt time.Time) error {
	tx, err := h.refreshStore.BeginTx(ctx)

	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	row := postgres.RefreshTokenRow{
		ID:        jti,
		UserID:    userID,
		TokenHash: h.jwt.HashRefreshToken(raw),
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}

	err = h.refreshStore.Create(ctx, tx, row)

	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (h *AuthHandler) refreshCookieName() string {
	return "refresh_token"
}

func (h *AuthHandler) setRefreshCookie(ctx *gin.Context, raw string, expiresAt time.Time) {
	secure := h.cfg.Env == "prod"

	maxAge := int(time.Until(expiresAt).Seconds())

	ctx.SetSameSite(http.SameSiteStrictMode)

	ctx.SetCookie(
		h.refreshCookieName(),
		raw,
		maxAge,
		"/auth",
		"",
		secure,
		true, // HttpOnly.
	)
}

func (h *AuthHandler) clearRefreshCookie(ctx *gin.Context) {
	secure := h.cfg.Env == "prod"
	ctx.SetSameSite(http.SameSiteStrictMode)
	ctx.SetCookie(
		h.refreshCookieName(),
		"",

		-1,
		"/auth",
		"",
		secure,
		true,
	)
}
