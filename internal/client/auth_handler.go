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

const refreshCookieName = "refresh_token"

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
	setRefreshCookie(c, response.RefreshToken, h.authService.RefreshTokenTTLSeconds())
	response.RefreshToken = ""

	c.JSON(http.StatusOK, response)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie(refreshCookieName)
	if err != nil || strings.TrimSpace(refreshToken) == "" {
		errorResponse(c, http.StatusUnauthorized, "refresh token is required")
		return
	}

	response, err := h.authService.RefreshTokens(refreshToken)
	if err != nil {
		if errors.Is(err, service.ErrInvalidRefreshToken) || errors.Is(err, service.ErrInvalidCredentials) {
			clearRefreshCookie(c)
			errorResponse(c, http.StatusUnauthorized, "invalid refresh token")
			return
		}
		errorResponse(c, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}
	setRefreshCookie(c, response.RefreshToken, h.authService.RefreshTokenTTLSeconds())
	response.RefreshToken = ""
	c.JSON(http.StatusOK, response)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if refreshToken, err := c.Cookie(refreshCookieName); err == nil {
		_ = h.authService.RevokeRefreshToken(refreshToken)
	}
	clearRefreshCookie(c)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func setRefreshCookie(c *gin.Context, token string, maxAgeSeconds int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		refreshCookieName,
		token,
		maxAgeSeconds,
		"/",
		"",
		c.Request.TLS != nil,
		true,
	)
}

func clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, "", -1, "/", "", c.Request.TLS != nil, true)
}
