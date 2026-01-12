package models

import "time"

// Student represents the student database model
type Student struct {
	PRN       string    `db:"prn" json:"prn"`
	Name      string    `db:"name" json:"name"`
	Year      int       `db:"year" json:"year"`
	Division  string    `db:"division" json:"division"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// StudentSummaryResponse represents student summary with credits
type StudentSummaryResponse struct {
	PRN          string                       `json:"prn"`
	Name         string                       `json:"name"`
	Year         int                          `json:"year"`
	Division     string                       `json:"division"`
	TotalCredits int                          `json:"totalCredits"`
	Internships  []*StudentInternshipResponse `json:"studentInternshipsResponse"`
}

// StudentListItem represents a student in the list view
type StudentListItem struct {
	PRN          string `json:"prn" db:"prn"`
	Name         string `json:"name" db:"name"`
	Year         int    `json:"year" db:"passing_year"`
	Division     string `json:"division" db:"division"`
	TotalCredits int    `json:"totalCredits" db:"total_credits"`
}

type StudentInternshipResponse struct {
	ID              int        `db:"id" json:"id"`
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
