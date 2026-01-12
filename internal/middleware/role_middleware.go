package middleware

import (
    "net/http"

    "github.com/gin-gonic/gin"
)

// RequireRole creates middleware that checks for specific role
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Get role from context (set by AuthMiddleware)
        role, exists := c.Get("role")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
            c.Abort()
            return
        }

        roleStr, ok := role.(string)
        if !ok {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid role data"})
            c.Abort()
            return
        }

        // Check if user's role is in allowed roles
        allowed := false
        for _, allowedRole := range allowedRoles {
            if roleStr == allowedRole {
                allowed = true
                break
            }
        }

        if !allowed {
            c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
            c.Abort()
            return
        }

        c.Next()
    }
}
