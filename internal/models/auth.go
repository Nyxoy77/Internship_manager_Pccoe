package models

import "time"

// LoginRequest represents the login request payload
type LoginRequest struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
}

// LoginResponse represents the login response
type LoginResponse struct {
    Token        string   `json:"token"` // backward compatible alias of accessToken
    AccessToken  string   `json:"accessToken"`
    RefreshToken string   `json:"refreshToken,omitempty"`
    User         UserInfo `json:"user"`
}

// UserInfo represents user information returned in responses
type UserInfo struct {
    ID       string `json:"id"`
    Username string `json:"username"`
    Role     string `json:"role"`
    Name     string `json:"name"`
}

// User represents the user database model
type User struct {
    ID           int       `db:"id"`
    Username     string    `db:"username"`
    PasswordHash string    `db:"password_hash"`
    Role         string    `db:"role"`
    Name         string    `db:"name"`
    CreatedAt    time.Time `db:"created_at"`
}
