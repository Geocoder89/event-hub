package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/geocoder89/eventhub/internal/auth"
	"github.com/geocoder89/eventhub/internal/config"
	"github.com/geocoder89/eventhub/internal/domain/user"
	"github.com/geocoder89/eventhub/internal/security"
	"github.com/gin-gonic/gin"
)

type UserReader interface {
	GetByEmail(ctx context.Context, email string) (user.User, error)
}

type AuthHandler struct {
	users UserReader
	jwt   *auth.Manager
}

func NewAuthHandler(users UserReader, jwtManager *auth.Manager) *AuthHandler {
	return &AuthHandler{
		users: users,
		jwt:   jwtManager,
	}
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
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
		fmt.Println(err)
		return
	}

	err = security.CheckPassword(foundUser.PasswordHash, req.Password)

	if err != nil {
		RespondUnAuthorized(ctx, "invalid_credentials", "Email or password is incorrect.")
		fmt.Println(err)
		return
	}

	token, err := h.jwt.GenerateAccessToken(foundUser.ID, foundUser.Email, foundUser.Role)

	if err != nil {
		RespondInternal(ctx, "Could not generate access token")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"accessToken": token,
	})
}
