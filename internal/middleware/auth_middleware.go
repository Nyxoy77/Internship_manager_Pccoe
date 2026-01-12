package middleware

import (
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"

    "github.com/yourusername/student-internship-manager/internal/service"
)

// AuthMiddleware validates JWT and extracts user information [web:6][web:9]
func AuthMiddleware(authService *service.AuthService) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract Authorization header
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
            c.Abort()
            return
        }

        // Check Bearer prefix
        parts := strings.Split(authHeader, " ")
        if len(parts) != 2 || parts[0] != "Bearer" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
            c.Abort()
            return
        }

        tokenString := parts[1]

        // Validate token and extract user info
        userID, role, err := authService.ValidateToken(tokenString)
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
            c.Abort()
            return
        }

        // Store userID as int and role in context
        c.Set("userID", userID)
        c.Set("role", role)

        c.Next()
    }
}
