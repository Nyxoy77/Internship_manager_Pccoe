package models

type CreateStudentRequest struct {
	PRN         string `json:"prn" binding:"required"`
	Name        string `json:"name" binding:"required"`
	PassingYear int    `json:"passing_year" binding:"required"`
	Division    string `json:"division" binding:"required"`
}
