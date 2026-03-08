package service

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"

	"github.com/yourusername/student-internship-manager/internal/models"
)

type AuthService struct {
	db              *sqlx.DB
	jwtSecret       []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

var ErrInvalidCredentials = errors.New("invalid credentials")

func NewAuthService(db *sqlx.DB, jwtSecret string, accessTokenTTL, refreshTokenTTL time.Duration) *AuthService {
	return &AuthService{
		db:              db,
		jwtSecret:       []byte(jwtSecret),
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
	}
}

// Login authenticates user and returns JWT token
func (s *AuthService) Login(username, password string) (*models.LoginResponse, error) {
	// Fetch user from database
	var user models.User
	query := `SELECT id, username, password_hash, role, name FROM users WHERE username = $1`
	err := s.db.Get(&user, query, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Generate JWT token
	accessToken, err := s.generateJWT(user.ID, user.Role, "access", s.accessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	refreshToken, err := s.generateJWT(user.ID, user.Role, "refresh", s.refreshTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Prepare response
	response := &models.LoginResponse{
		Token:        accessToken,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: models.UserInfo{
			ID:       strconv.Itoa(user.ID),
			Username: user.Username,
			Role:     user.Role,
			Name:     user.Name,
		},
	}

	return response, nil
}

// generateJWT creates a JWT token with user ID stored as string claim
func (s *AuthService) generateJWT(userID int, role, tokenType string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"id":   strconv.Itoa(userID),
		"role": role,
		"type": tokenType,
		"exp":  time.Now().Add(ttl).Unix(),
		"iat":  time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken validates JWT and returns user ID as int
func (s *AuthService) validateTokenByType(tokenString, expectedType string) (int, string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return 0, "", err
	}

	if !token.Valid {
		return 0, "", errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, "", errors.New("invalid token claims")
	}

	// Extract ID as string and convert to int
	idStr, ok := claims["id"].(string)
	if !ok {
		return 0, "", errors.New("invalid user ID in token")
	}

	userID, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, "", errors.New("invalid user ID format")
	}

	// Extract role
	role, ok := claims["role"].(string)
	if !ok {
		return 0, "", errors.New("invalid role in token")
	}
	tokenType, ok := claims["type"].(string)
	if !ok {
		return 0, "", errors.New("invalid token type")
	}
	if tokenType != expectedType {
		return 0, "", errors.New("invalid token type for this endpoint")
	}

	return userID, role, nil
}

// ValidateToken validates access token and returns user ID as int
func (s *AuthService) ValidateToken(tokenString string) (int, string, error) {
	return s.validateTokenByType(tokenString, "access")
}

func (s *AuthService) RefreshTokens(refreshToken string) (*models.LoginResponse, error) {
	userID, role, err := s.validateTokenByType(refreshToken, "refresh")
	if err != nil {
		return nil, err
	}

	var user models.User
	query := `SELECT id, username, role, name FROM users WHERE id = $1`
	if err := s.db.Get(&user, query, userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	accessToken, err := s.generateJWT(user.ID, role, "access", s.accessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}
	newRefreshToken, err := s.generateJWT(user.ID, role, "refresh", s.refreshTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &models.LoginResponse{
		Token:        accessToken,
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		User: models.UserInfo{
			ID:       strconv.Itoa(user.ID),
			Username: user.Username,
			Role:     user.Role,
			Name:     user.Name,
		},
	}, nil
}
