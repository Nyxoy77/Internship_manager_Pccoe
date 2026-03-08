package models

type CreateStudentRequest struct {
	PRN         string `json:"prn" binding:"required"`
	Name        string `json:"name" binding:"required"`
	GuideName   string `json:"guide_name"`
	PassingYear int    `json:"passing_year" binding:"required"`
	Division    string `json:"division" binding:"required"`
}

type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Role     string `json:"role" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}
