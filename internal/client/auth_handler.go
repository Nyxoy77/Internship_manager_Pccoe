package client

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yourusername/student-internship-manager/internal/models"
	"github.com/yourusername/student-internship-manager/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// Login handles user authentication
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest

	// Bind and validate request
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request payload")
		return
	}
	req.Password = strings.TrimSpace(req.Password)
	req.Username = strings.TrimSpace(req.Username)
	// Authenticate user
	response, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			errorResponse(c, http.StatusUnauthorized, "invalid credentials")
			return
		}
		log.Printf("login failed for user %q: %v", req.Username, err)
		errorResponse(c, http.StatusInternalServerError, "login failed")
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req models.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request payload")
		return
	}
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		errorResponse(c, http.StatusBadRequest, "refresh token is required")
		return
	}

	response, err := h.authService.RefreshTokens(req.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			errorResponse(c, http.StatusUnauthorized, "invalid refresh token")
			return
		}
		errorResponse(c, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}
	c.JSON(http.StatusOK, response)
}
