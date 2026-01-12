package client

import (
    "net/http"

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
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
        return
    }

    // Authenticate user
    response, err := h.authService.Login(req.Username, req.Password)
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, response)
}
