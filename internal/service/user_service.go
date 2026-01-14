package service

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/yourusername/student-internship-manager/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	db *sqlx.DB
}

func NewUserService(db *sqlx.DB) *UserService {
	return &UserService{db: db}
}

// New user creation to be done strictly via admin.
func (s *UserService) CreateUser(req *models.CreateUserRequest) error {

	req.Username = strings.TrimSpace(req.Username)
	req.Name = strings.TrimSpace(req.Name)
	req.Role = strings.ToLower(strings.TrimSpace(req.Role))

	if req.Role != "admin" && req.Role != "manager" {
		return fmt.Errorf("invalid role")
	}

	hashed, err := bcrypt.GenerateFromPassword(
		[]byte(req.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return fmt.Errorf("failed to hash password")
	}

	_, err = s.db.Exec(`
		INSERT INTO users (username, password_hash, role, name)
		VALUES ($1,$2,$3,$4)
	`, req.Username, string(hashed), req.Role, req.Name)

	if err != nil {
		if strings.Contains(err.Error(), "users_username_key") {
			return fmt.Errorf("username already exists")
		}
		return err
	}

	return nil
}

// Password change for both admin and manager IAM and self password provisioning.
func (s *UserService) ChangePassword(
	userID int,
	req *models.ChangePasswordRequest,
) error {

	var passwordHash string
	err := s.db.Get(&passwordHash, `
		SELECT password_hash
		FROM users
		WHERE id = $1
	`, userID)
	
	if err != nil {
		return fmt.Errorf("user not found")
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(passwordHash),
		[]byte(req.OldPassword),
	); err != nil {
		return fmt.Errorf("old password is incorrect")
	}

	newHash, err := bcrypt.GenerateFromPassword(
		[]byte(req.NewPassword),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return fmt.Errorf("failed to hash new password")
	}

	_, err = s.db.Exec(`
		UPDATE users
		SET password_hash = $1
		WHERE id = $2
	`, string(newHash), userID)

	return err
}
