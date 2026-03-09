package service

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
	"github.com/google/uuid"
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
var ErrInvalidRefreshToken = errors.New("invalid refresh token")

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
	accessToken, _, err := s.generateJWT(user.ID, user.Role, "access", s.accessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	refreshToken, refreshExpiry, err := s.generateJWT(user.ID, user.Role, "refresh", s.refreshTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}
	if err := s.storeRefreshToken(user.ID, refreshToken, refreshExpiry); err != nil {
		return nil, fmt.Errorf("failed to persist refresh token: %w", err)
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
func (s *AuthService) generateJWT(userID int, role, tokenType string, ttl time.Duration) (string, time.Time, error) {
	expiry := time.Now().Add(ttl)
	claims := jwt.MapClaims{
		"id":   strconv.Itoa(userID),
		"role": role,
		"type": tokenType,
		"jti":  uuid.NewString(),
		"exp":  expiry.Unix(),
		"iat":  time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiry, nil
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
		return nil, ErrInvalidRefreshToken
	}
	active, err := s.isRefreshTokenActive(userID, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to validate refresh token state: %w", err)
	}
	if !active {
		return nil, ErrInvalidRefreshToken
	}

	var user models.User
	query := `SELECT id, username, role, name FROM users WHERE id = $1`
	if err := s.db.Get(&user, query, userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrInvalidRefreshToken
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	accessToken, _, err := s.generateJWT(user.ID, role, "access", s.accessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}
	newRefreshToken, newRefreshExpiry, err := s.generateJWT(user.ID, role, "refresh", s.refreshTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}
	if err := s.rotateRefreshToken(userID, refreshToken, newRefreshToken, newRefreshExpiry); err != nil {
		return nil, fmt.Errorf("failed to rotate refresh token: %w", err)
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

func (s *AuthService) hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *AuthService) storeRefreshToken(userID int, refreshToken string, expiresAt time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, s.hashToken(refreshToken), expiresAt)
	return err
}

func (s *AuthService) isRefreshTokenActive(userID int, refreshToken string) (bool, error) {
	var exists bool
	err := s.db.Get(&exists, `
		SELECT EXISTS (
			SELECT 1
			FROM refresh_tokens
			WHERE user_id = $1
			  AND token_hash = $2
			  AND revoked_at IS NULL
			  AND expires_at > NOW()
		)
	`, userID, s.hashToken(refreshToken))
	return exists, err
}

func (s *AuthService) rotateRefreshToken(userID int, oldToken, newToken string, newExpiresAt time.Time) error {
	oldHash := s.hashToken(oldToken)
	newHash := s.hashToken(newToken)

	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		UPDATE refresh_tokens
		SET revoked_at = NOW(), replaced_by_hash = $3
		WHERE user_id = $1
		  AND token_hash = $2
		  AND revoked_at IS NULL
	`, userID, oldHash, newHash)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrInvalidRefreshToken
	}

	if _, err := tx.Exec(`
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, newHash, newExpiresAt); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *AuthService) RevokeRefreshToken(refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return nil
	}
	_, err := s.db.Exec(`
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE token_hash = $1
		  AND revoked_at IS NULL
	`, s.hashToken(refreshToken))
	return err
}

func (s *AuthService) RefreshTokenTTLSeconds() int {
	return int(s.refreshTokenTTL.Seconds())
}
