package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/yourusername/student-internship-manager/internal/models"
)

type InternshipService struct {
	db             *sqlx.DB
	studentService *StudentService
}

func NewInternshipService(db *sqlx.DB, studentService *StudentService) *InternshipService {
	return &InternshipService{
		db:             db,
		studentService: studentService,
	}
}

func (s *InternshipService) CreateInternship(
	req *models.CreateInternshipRequest,
	createdBy int,
) error {

	startDate, endDate, err := s.validateAndPrepareInternship(req)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO internships (
			student_prn,
			organization,
			description,
			start_date,
			end_date,
			mode,
			credits,
			monthly_stipend,
			status,
			credit_eligible,
			created_by
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'pending',FALSE,$9)
	`

	_, err = s.db.Exec(
		query,
		req.PRN,
		req.Organization,
		req.Description,
		startDate,
		endDate,
		req.Mode,
		req.Credits,
		req.MonthlyStipend,
		createdBy,
	)

	return err
}

func (s *InternshipService) BatchCreateInternships(
	requests []models.CreateInternshipRequest,
	createdBy int,
) *models.BatchUploadResponse {

	response := &models.BatchUploadResponse{
		TotalRows: len(requests),
	}

	tx, err := s.db.Beginx()
	if err != nil {
		response.Failed = len(requests)
		response.Errors = append(response.Errors, models.BatchUploadError{
			Row:   0,
			Error: "failed to start database transaction",
		})
		return response
	}
	defer tx.Rollback()

	for i, req := range requests {
		rowNum := i + 1

		// sanitization
		req.PRN = strings.TrimSpace(req.PRN)
		req.Organization = strings.TrimSpace(req.Organization)
		req.Description = strings.TrimSpace(req.Description)
		req.Mode = strings.ToLower(strings.TrimSpace(req.Mode))

		startDate, endDate, err := s.validateAndPrepareInternship(&req)
		if err != nil {
			response.Failed = len(requests)
			response.Errors = append(response.Errors, models.BatchUploadError{
				Row:   rowNum,
				Error: err.Error(),
			})
			return response
		}

		_, err = tx.Exec(`
			INSERT INTO internships (
				student_prn,
				organization,
				description,
				start_date,
				end_date,
				mode,
				credits,
				monthly_stipend,
				status,
				credit_eligible,
				created_by
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'pending',FALSE,$9)
		`,
			req.PRN,
			req.Organization,
			req.Description,
			startDate,
			endDate,
			req.Mode,
			req.Credits,
			req.MonthlyStipend,
			createdBy,
		)

		if err != nil {
			response.Failed = len(requests)
			response.Errors = append(response.Errors, models.BatchUploadError{
				Row:   rowNum,
				Error: "database insert failed",
			})
			return response
		}

		response.Inserted++
	}

	if err := tx.Commit(); err != nil {
		response.Failed = len(requests)
		response.Inserted = 0
		response.Errors = append(response.Errors, models.BatchUploadError{
			Row:   0,
			Error: "failed to commit transaction",
		})
	}

	return response
}

func (s *InternshipService) ApproveInternship(
	internshipID int,
	approvedBy int,
) error {

	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var prn string
	err = tx.Get(&prn, `
		UPDATE internships
		SET status = 'approved',
		    approved_by = $1,
		    approved_at = NOW()
		WHERE id = $2
		  AND status = 'pending'
		RETURNING student_prn
	`, approvedBy, internshipID)
	if err != nil {
		return fmt.Errorf("internship not found or already processed")
	}

	if err := s.recalculateCreditEligibilityTx(tx, prn); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *InternshipService) RejectInternship(
	internshipID int,
	approvedBy int,
) error {

	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var prn string
	err = tx.Get(&prn, `
		UPDATE internships
		SET status = 'rejected',
		    approved_by = $1,
		    approved_at = NOW()
		WHERE id = $2
		  AND status = 'pending'
		RETURNING student_prn
	`, approvedBy, internshipID)
	if err != nil {
		return fmt.Errorf("internship not found or already processed")
	}

	// Approved competitor removed → recalc
	if err := s.recalculateCreditEligibilityTx(tx, prn); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *InternshipService) recalculateCreditEligibilityTx(tx *sqlx.Tx, prn string) error {

	type internshipRow struct {
		ID        int       `db:"id"`
		StartDate time.Time `db:"start_date"`
		EndDate   time.Time `db:"end_date"`
	}

	var internships []internshipRow
	err := tx.Select(&internships, `
		SELECT id, start_date, end_date
		FROM internships
		WHERE student_prn = $1
		  AND status = 'approved'
		ORDER BY start_date, end_date
	`, prn)
	if err != nil {
		return err
	}

	// Reset only approved internships
	_, err = tx.Exec(`
		UPDATE internships
		SET credit_eligible = FALSE
		WHERE student_prn = $1
		  AND status = 'approved'
	`, prn)
	if err != nil {
		return err
	}

	n := len(internships)
	if n == 0 {
		return nil
	}

	i := 0
	for i < n {
		groupEnd := internships[i].EndDate
		group := []internshipRow{internships[i]}

		j := i + 1
		for j < n && (internships[j].StartDate.Before(groupEnd)) {
			group = append(group, internships[j])
			if internships[j].EndDate.After(groupEnd) {
				groupEnd = internships[j].EndDate
			}
			j++
		}

		longestID := group[0].ID
		maxDuration := int(group[0].EndDate.Sub(group[0].StartDate).Hours() / 24)

		for _, in := range group {
			dur := int(in.EndDate.Sub(in.StartDate).Hours() / 24)
			if dur > maxDuration {
				maxDuration = dur
				longestID = in.ID
			}
		}

		_, err := tx.Exec(`
			UPDATE internships
			SET credit_eligible = TRUE
			WHERE id = $1
		`, longestID)
		if err != nil {
			return err
		}

		i = j
	}

	return nil
}

func (s *InternshipService) validateAndPrepareInternship(
	req *models.CreateInternshipRequest,
) (time.Time, time.Time, error) {

	exists, err := s.studentService.StudentExists(req.PRN)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !exists {
		return time.Time{}, time.Time{}, fmt.Errorf("student with PRN %s not found", req.PRN)
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start date format")
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end date format")
	}

	if endDate.Before(startDate) {
		return time.Time{}, time.Time{}, fmt.Errorf("end date must be after or equal to start date")
	}

	if req.Credits <= 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("credits must be greater than 0")
	}

	switch req.Mode {
	case "online", "offline", "hybrid":
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("invalid internship mode")
	}

	if req.MonthlyStipend < 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("monthly stipend cannot be negative")
	}

	return startDate, endDate, nil
}

// GetPendingInternships returns all pending internships
func (s *InternshipService) GetPendingInternships() ([]models.InternshipWithStudentName, error) {
	query := `
		SELECT 
			i.id,
			i.student_prn,
			i.organization,
			i.description,
			i.start_date,
			i.end_date,
			i.mode,
			i.credits,
			i.monthly_stipend,
			i.stipend_currency,
			i.status,
			i.created_by,
			i.created_at,
			i.approved_by,
			i.approved_at,
			s.name AS student_name
		FROM internships i
		JOIN students s ON i.student_prn = s.prn
		WHERE i.status = 'pending'
		ORDER BY i.created_at DESC
	`

	var internships []models.InternshipWithStudentName
	if err := s.db.Select(&internships, query); err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	return internships, nil
}
