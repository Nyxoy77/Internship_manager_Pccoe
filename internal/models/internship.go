package models

import (
	"time"
)

// CreateInternshipRequest represents single internship creation request
type CreateInternshipRequest struct {
	PRN            string  `json:"prn" binding:"required"`
	Organization   string  `json:"organization" binding:"required"`
	Description    string  `json:"description,omitempty"`
	StartDate      string  `json:"startDate" binding:"required,datetime=2006-01-02"`
	EndDate        string  `json:"endDate" binding:"required,datetime=2006-01-02"`
	Mode           string  `json:"mode" binding:"required,oneof=online offline hybrid"`
	Credits        int     `json:"credits" binding:"required,gt=0"`
	MonthlyStipend float64 `json:"monthlyStipend" binding:"required,gte=0"`
}

// Internship represents the internship database model
type Internship struct {
	ID              int        `db:"id" json:"id"`
	StudentPRN      string     `db:"student_prn" json:"studentPrn"`
	Organization    string     `db:"organization" json:"organization"`
	Description     *string    `db:"description" json:"description,omitempty"`
	StartDate       time.Time  `db:"start_date" json:"startDate"`
	EndDate         time.Time  `db:"end_date" json:"endDate"`
	Mode            string     `db:"mode" json:"mode"`
	Credits         int        `db:"credits" json:"credits"`
	MonthlyStipend  float64    `db:"monthly_stipend" json:"monthlyStipend"`
	StipendCurrency string     `db:"stipend_currency" json:"stipendCurrency"`
	Status          string     `db:"status" json:"status"`
	CreatedBy       int        `db:"created_by" json:"createdBy"`
	CreatedAt       time.Time  `db:"created_at" json:"createdAt"`
	ApprovedBy      *int       `db:"approved_by" json:"approvedBy,omitempty"`
	ApprovedAt      *time.Time `db:"approved_at" json:"approvedAt,omitempty"`
}

// InternshipWithStudentName extends Internship with student name for display
type InternshipWithStudentName struct {
	Internship
	StudentName string `db:"student_name" json:"studentName"`
}

// BatchUploadResponse represents the response for batch upload
type BatchUploadResponse struct {
	TotalRows int                `json:"totalRows"`
	Inserted  int                `json:"inserted"`
	Failed    int                `json:"failed"`
	Errors    []BatchUploadError `json:"errors"`
}

// BatchUploadError represents an error in batch upload
type BatchUploadError struct {
	Row   int    `json:"row"`
	Error string `json:"error"`
}

// ApprovalRequest represents approval/rejection action
type ApprovalRequest struct {
	// No body needed, action determined by endpoint
}
