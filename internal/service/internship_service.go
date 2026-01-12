package service

import (
	"fmt"
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

// CreateInternship creates a single internship entry
func (s *InternshipService) CreateInternship(req *models.CreateInternshipRequest, createdBy int) error {

	startDate, endDate, err := s.validateAndPrepareInternship(req)
	if err != nil {
		return err
	}

	tx, err := s.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = s.insertInternshipTx(tx, req, startDate, endDate, createdBy)
	if err != nil {
		return err
	}

	if err := s.recalculateCreditEligibilityTx(tx, req.PRN); err != nil {
		return err
	}

	return tx.Commit()
}

// BatchCreateInternships processes multiple internship entries with transaction [web:7]
func (s *InternshipService) BatchCreateInternships(requests []models.CreateInternshipRequest, createdBy int) *models.BatchUploadResponse {

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

		startDate, endDate, err := s.validateAndPrepareInternship(&req)
		if err != nil {
			response.Failed = len(requests)
			response.Errors = append(response.Errors, models.BatchUploadError{
				Row:   rowNum,
				Error: err.Error(),
			})
			return response
		}

		_, err = s.insertInternshipTx(tx, &req, startDate, endDate, createdBy)
		if err != nil {
			response.Failed = len(requests)
			response.Errors = append(response.Errors, models.BatchUploadError{
				Row:   rowNum,
				Error: "database insert failed",
			})
			return response
		}

		if err := s.recalculateCreditEligibilityTx(tx, req.PRN); err != nil {
			response.Failed = len(requests)
			response.Errors = append(response.Errors, models.BatchUploadError{
				Row:   rowNum,
				Error: "failed to recalculate credit eligibility",
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

func (s *InternshipService) validateAndPrepareInternship(req *models.CreateInternshipRequest) (time.Time, time.Time, error) {

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

func (s *InternshipService) insertInternshipTx(tx *sqlx.Tx, req *models.CreateInternshipRequest, startDate, endDate time.Time, createdBy int) (int, error) {
	var internshipID int
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
		RETURNING id
	`

	err := tx.Get(
		&internshipID,
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

	return internshipID, err
}

func (s *InternshipService) recalculateCreditEligibilityTx(
	tx *sqlx.Tx,
	prn string,
) error {

	type internshipRow struct {
		ID        int       `db:"id"`
		StartDate time.Time `db:"start_date"`
		EndDate   time.Time `db:"end_date"`
	}

	// 1️⃣ Fetch ALL internships for the student
	var internships []internshipRow
	err := tx.Select(&internships, `
		SELECT id, start_date, end_date
		FROM internships
		WHERE student_prn = $1
		ORDER BY start_date, end_date
	`, prn)
	if err != nil {
		return err
	}

	// 2️⃣ Reset all to NOT eligible
	_, err = tx.Exec(`
		UPDATE internships
		SET credit_eligible = FALSE
		WHERE student_prn = $1
	`, prn)
	if err != nil {
		return err
	}

	n := len(internships)
	if n == 0 {
		return nil
	}

	// 3️⃣ Build overlap groups
	i := 0
	for i < n {
		groupEnd := internships[i].EndDate
		group := []internshipRow{internships[i]}

		j := i + 1
		for j < n && !internships[j].StartDate.After(groupEnd) {
			group = append(group, internships[j])
			if internships[j].EndDate.After(groupEnd) {
				groupEnd = internships[j].EndDate
			}
			j++
		}

		// 4️⃣ Pick longest internship in this group
		longestID := group[0].ID
		maxDuration := int(group[0].EndDate.Sub(group[0].StartDate).Hours() / 24)

		for _, in := range group {
			dur := int(in.EndDate.Sub(in.StartDate).Hours() / 24)
			if dur > maxDuration {
				maxDuration = dur
				longestID = in.ID
			}
		}

		// 5️⃣ Mark ONLY the winner as eligible
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

// ApproveInternship approves an internship
func (s *InternshipService) ApproveInternship(internshipID int, approvedBy int) error {
	query := `
        UPDATE internships 
        SET status = 'approved', approved_by = $1, approved_at = NOW()
        WHERE id = $2 AND status = 'pending'
    `

	result, err := s.db.Exec(query, approvedBy, internshipID)
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("internship not found or already processed")
	}

	return nil
}

// RejectInternship rejects an internship
func (s *InternshipService) RejectInternship(internshipID int, approvedBy int) error {
	query := `
        UPDATE internships 
        SET status = 'rejected', approved_by = $1, approved_at = NOW()
        WHERE id = $2 AND status = 'pending'
    `

	result, err := s.db.Exec(query, approvedBy, internshipID)
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("internship not found or already processed")
	}

	return nil
}
