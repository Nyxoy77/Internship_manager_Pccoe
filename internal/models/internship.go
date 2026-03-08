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
	MonthlyStipend float64 `json:"monthlyStipend" binding:"gte=0"`
	ProcessedRow   int     `json:"-"`
	SheetRow       int     `json:"-"`
	RawStartDate   string  `json:"-"`
	RawEndDate     string  `json:"-"`
	RawMode        string  `json:"-"`
}

// Internship represents the internship database model
type Internship struct {
	ID              int        `db:"id" json:"id"`
	StudentPRN      string     `db:"student_prn" json:"studentPrn"`
	Organization    string     `db:"organization" json:"organization"`
	Description     *string    `db:"description" json:"description,omitempty"`
	GuideName       string     `db:"guide_name" json:"guideName"`
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
	ReviewNote      *string    `db:"review_note" json:"reviewNote,omitempty"`
	WorkflowStatus  string     `db:"workflow_status" json:"workflowStatus"`
}

// InternshipWithStudentName extends Internship with student name for display
type InternshipWithStudentName struct {
	Internship
	StudentName string `db:"student_name" json:"studentName"`
	Year        int    `db:"year" json:"year"`
	Division    string `db:"division" json:"division"`
}

type CreateInternshipResponse struct {
	Message  string   `json:"message"`
	Warnings []string `json:"warnings,omitempty"`
}

// BatchUploadResponse represents the response for batch upload
type BatchUploadResponse struct {
	TotalRows int                  `json:"totalRows"`
	Inserted  int                  `json:"inserted"`
	Failed    int                  `json:"failed"`
	Errors    []BatchUploadError   `json:"errors"`
	Warnings  []BatchUploadWarning `json:"warnings,omitempty"`
}

// BatchUploadError represents an error in batch upload
type BatchUploadError struct {
	Row          int    `json:"row"`
	ProcessedRow int    `json:"processedRow,omitempty"`
	SheetRow     int    `json:"sheetRow,omitempty"`
	Category     string `json:"category,omitempty"`
	Field        string `json:"field,omitempty"`
	RawValue     string `json:"rawValue,omitempty"`
	Suggestion   string `json:"suggestion,omitempty"`
	Error        string `json:"error"`
}

type BatchUploadWarning struct {
	Row          int      `json:"row"`
	ProcessedRow int      `json:"processedRow,omitempty"`
	SheetRow     int      `json:"sheetRow,omitempty"`
	Message      string   `json:"message"`
	Items        []string `json:"items,omitempty"`
}

// ApprovalRequest represents approval/rejection action
type ApprovalRequest struct {
	ReviewNote string `json:"reviewNote"`
}

type BulkReviewRequest struct {
	InternshipIDs []int  `json:"internshipIds" binding:"required,min=1"`
	Action        string `json:"action" binding:"required,oneof=approve reject"`
	ReviewNote    string `json:"reviewNote"`
	Confirm       bool   `json:"confirm"`
}

type BulkReviewResponse struct {
	Message       string `json:"message"`
	ProcessedRows int    `json:"processedRows"`
}

type InternshipListResponse struct {
	Items    []InternshipWithStudentName `json:"items"`
	Page     int                         `json:"page"`
	PageSize int                         `json:"pageSize"`
	Total    int                         `json:"total"`
}

type InternshipAuditEvent struct {
	ID         int       `db:"id" json:"id"`
	Internship int       `db:"internship_id" json:"internshipId"`
	Action     string    `db:"action" json:"action"`
	Note       string    `db:"note" json:"note"`
	ActorUser  *int      `db:"actor_user_id" json:"actorUserId,omitempty"`
	ActorRole  *string   `db:"actor_role" json:"actorRole,omitempty"`
	CreatedAt  time.Time `db:"created_at" json:"createdAt"`
}
