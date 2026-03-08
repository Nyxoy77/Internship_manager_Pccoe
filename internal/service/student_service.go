package service

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/yourusername/student-internship-manager/internal/models"
)

type StudentService struct {
	db *sqlx.DB
}

func NewStudentService(db *sqlx.DB) *StudentService {
	return &StudentService{db: db}
}

// GetStudentSummary returns student info with total approved credits
func (s *StudentService) GetStudentSummary(prn string) (*models.StudentSummaryResponse, error) {
	// Query student with aggregated credits from approved internships
	query := `
        SELECT 
            s.prn,
            s.name,
            s.guide_name,
            s.passing_year,
            s.division,
            COALESCE(SUM(CASE WHEN i.status = 'approved' AND i.credit_eligible = TRUE THEN i.credits ELSE 0 END), 0) as total_credits
        FROM students s
        LEFT JOIN internships i ON s.prn = i.student_prn
        WHERE s.prn = $1
        GROUP BY s.prn, s.name, s.guide_name, s.passing_year, s.division
    `

	var result struct {
		PRN          string `db:"prn"`
		Name         string `db:"name"`
		GuideName    string `db:"guide_name"`
		PassingYear  int    `db:"passing_year"`
		Division     string `db:"division"`
		TotalCredits int    `db:"total_credits"`
	}

	err := s.db.Get(&result, query, prn)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("student not found")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	internshipQuery := `
    SELECT
        i.id,
        i.organization,
        i.description,
        i.start_date,
        i.end_date,
        i.mode,
        i.credits,
        i.monthly_stipend,
        i.status,
        i.created_at,
        i.approved_at,
        i.review_note,
        (c.id IS NOT NULL) AS has_certificate
    FROM internships i
    LEFT JOIN certificates c
        ON c.internship_id = i.id
    WHERE i.student_prn = $1
    ORDER BY i.start_date DESC
`

	var internships []*models.StudentInternshipResponse
	if err := s.db.Select(&internships, internshipQuery, prn); err != nil {
		return nil, fmt.Errorf("failed to fetch internships: %w", err)
	}

	response := &models.StudentSummaryResponse{
		PRN:          result.PRN,
		Name:         result.Name,
		GuideName:    result.GuideName,
		Year:         result.PassingYear,
		Division:     result.Division,
		TotalCredits: result.TotalCredits,
		Internships:  internships,
	}

	return response, nil
}

// ListStudents returns students filtered by year and division with credits
func (s *StudentService) ListStudents(passingYear *int, division string) ([]models.StudentListItem, error) {
	// Build query with optional filters
	query := `
        SELECT 
            s.prn,
            s.name,
            s.guide_name,
            s.passing_year,
            s.division,
            COALESCE(SUM(CASE WHEN i.status = 'approved' AND i.credit_eligible = TRUE THEN i.credits ELSE 0 END), 0) as total_credits
        FROM students s
        LEFT JOIN internships i ON s.prn = i.student_prn
    `

	// Add WHERE clause for filters
	var conditions []string
	var args []interface{}
	argPos := 1

	if passingYear != nil {
		conditions = append(conditions, fmt.Sprintf("s.passing_year = $%d", argPos))
		args = append(args, *passingYear)
		argPos++
	}

	if division != "" {
		conditions = append(conditions, fmt.Sprintf("s.division = $%d", argPos))
		args = append(args, division)
		argPos++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " GROUP BY s.prn, s.name, s.guide_name, s.passing_year, s.division ORDER BY s.prn"

	var students []models.StudentListItem
	err := s.db.Select(&students, query, args...)
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	return students, nil
}

// StudentExists checks if a student with given PRN exists
func (s *StudentService) StudentExists(prn string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM students WHERE prn = $1)`
	err := s.db.Get(&exists, query, prn)
	if err != nil {
		return false, fmt.Errorf("database error: %w", err)
	}
	return exists, nil
}
