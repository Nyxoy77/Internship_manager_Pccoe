package service

import (
	"database/sql"
	"fmt"
	"strconv"
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
	        i.workflow_status,
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

// ListStudents returns students filtered by lookup fields with pagination.
func (s *StudentService) ListStudents(
	page int,
	pageSize int,
	passingYear *int,
	division string,
	prn string,
	name string,
	guide string,
) (*models.StudentListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	whereClause, args, argPos := buildStudentListWhereClause(passingYear, division, prn, name, guide)

	countQuery := `SELECT COUNT(1) FROM students s` + whereClause
	var total int
	if err := s.db.Get(&total, countQuery, args...); err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	offset := (page - 1) * pageSize
	dataArgs := append(args, pageSize, offset)

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
    ` + whereClause + `
        GROUP BY s.prn, s.name, s.guide_name, s.passing_year, s.division
        ORDER BY s.prn
        LIMIT $` + strconv.Itoa(argPos) + ` OFFSET $` + strconv.Itoa(argPos+1)

	var students []models.StudentListItem
	if err := s.db.Select(&students, query, dataArgs...); err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &models.StudentListResponse{
		Items:    students,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// ListStudentCreditReport returns student credit summary rows with filters and pagination.
func (s *StudentService) ListStudentCreditReport(
	page int,
	pageSize int,
	passingYear *int,
	division string,
	prn string,
	name string,
	guide string,
	creditFilter string,
) (*models.StudentListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	whereClause, args, _ := buildStudentListWhereClause(passingYear, division, prn, name, guide)
	baseCTE := `
		WITH credit_rows AS (
			SELECT
				s.prn,
				s.name,
				s.guide_name,
				s.passing_year,
				s.division,
				COALESCE(SUM(CASE WHEN i.status = 'approved' AND i.credit_eligible = TRUE THEN i.credits ELSE 0 END), 0) as total_credits
			FROM students s
			LEFT JOIN internships i ON s.prn = i.student_prn
			` + whereClause + `
			GROUP BY s.prn, s.name, s.guide_name, s.passing_year, s.division
		)
	`

	creditWhere := buildCreditFilterWhere(creditFilter)

	countQuery := baseCTE + ` SELECT COUNT(1) FROM credit_rows ` + creditWhere
	var total int
	if err := s.db.Get(&total, countQuery, args...); err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	offset := (page - 1) * pageSize
	dataArgs := append(args, pageSize, offset)
	dataQuery := baseCTE + `
		SELECT prn, name, guide_name, passing_year, division, total_credits
		FROM credit_rows
	` + creditWhere + `
		ORDER BY prn
		LIMIT $` + strconv.Itoa(len(args)+1) + ` OFFSET $` + strconv.Itoa(len(args)+2)

	var students []models.StudentListItem
	if err := s.db.Select(&students, dataQuery, dataArgs...); err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &models.StudentListResponse{
		Items:    students,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func (s *StudentService) ListStudentCreditReportForExport(
	passingYear *int,
	division string,
	prn string,
	name string,
	guide string,
	creditFilter string,
) ([]models.StudentListItem, error) {
	whereClause, args, _ := buildStudentListWhereClause(passingYear, division, prn, name, guide)
	baseCTE := `
		WITH credit_rows AS (
			SELECT
				s.prn,
				s.name,
				s.guide_name,
				s.passing_year,
				s.division,
				COALESCE(SUM(CASE WHEN i.status = 'approved' AND i.credit_eligible = TRUE THEN i.credits ELSE 0 END), 0) as total_credits
			FROM students s
			LEFT JOIN internships i ON s.prn = i.student_prn
			` + whereClause + `
			GROUP BY s.prn, s.name, s.guide_name, s.passing_year, s.division
		)
	`

	query := baseCTE + `
		SELECT prn, name, guide_name, passing_year, division, total_credits
		FROM credit_rows
	` + buildCreditFilterWhere(creditFilter) + `
		ORDER BY prn
	`

	var students []models.StudentListItem
	if err := s.db.Select(&students, query, args...); err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	return students, nil
}

func buildCreditFilterWhere(creditFilter string) string {
	switch strings.ToLower(strings.TrimSpace(creditFilter)) {
	case "zero":
		return " WHERE total_credits = 0"
	case "non_zero":
		return " WHERE total_credits > 0"
	default:
		return ""
	}
}

func buildStudentListWhereClause(passingYear *int, division, prn, name, guide string) (string, []interface{}, int) {
	var conditions []string
	var args []interface{}
	argPos := 1

	if passingYear != nil {
		conditions = append(conditions, "s.passing_year = $"+strconv.Itoa(argPos))
		args = append(args, *passingYear)
		argPos++
	}
	if strings.TrimSpace(division) != "" {
		conditions = append(conditions, "LOWER(s.division) = LOWER($"+strconv.Itoa(argPos)+")")
		args = append(args, strings.TrimSpace(division))
		argPos++
	}
	if strings.TrimSpace(prn) != "" {
		conditions = append(conditions, "LOWER(s.prn) LIKE LOWER($"+strconv.Itoa(argPos)+")")
		args = append(args, "%"+strings.TrimSpace(prn)+"%")
		argPos++
	}
	if strings.TrimSpace(name) != "" {
		conditions = append(conditions, "LOWER(s.name) LIKE LOWER($"+strconv.Itoa(argPos)+")")
		args = append(args, "%"+strings.TrimSpace(name)+"%")
		argPos++
	}
	if strings.TrimSpace(guide) != "" {
		conditions = append(conditions, "LOWER(s.guide_name) LIKE LOWER($"+strconv.Itoa(argPos)+")")
		args = append(args, "%"+strings.TrimSpace(guide)+"%")
		argPos++
	}

	if len(conditions) == 0 {
		return "", args, argPos
	}
	return " WHERE " + strings.Join(conditions, " AND "), args, argPos
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
