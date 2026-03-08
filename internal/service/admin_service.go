package service

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/yourusername/student-internship-manager/internal/models"
)

type AdminService struct {
	db *sqlx.DB
}

func NewAdminService(db *sqlx.DB) *AdminService {
	return &AdminService{db: db}
}

/* -----------------------------
   1️⃣ Individual student create
--------------------------------*/

func (s *AdminService) CreateStudent(req *models.CreateStudentRequest) error {

	req.Division = strings.ToUpper(req.Division)
	req.GuideName = strings.TrimSpace(req.GuideName)

	if req.PassingYear < 2000 {
		return fmt.Errorf("invalid passing year")
	}

	query := `
		INSERT INTO students (prn, name, guide_name, passing_year, division)
		VALUES ($1,$2,$3,$4,$5)
	`

	_, err := s.db.Exec(
		query,
		req.PRN,
		req.Name,
		req.GuideName,
		req.PassingYear,
		req.Division,
	)

	if err != nil {
		if strings.Contains(err.Error(), "students_pkey") {
			return fmt.Errorf("student with PRN %s already exists", req.PRN)
		}
		return err
	}

	return nil
}

/* -----------------------------
   2️⃣ Batch upload (CSV/Excel)
--------------------------------*/

func (s *AdminService) BatchCreateStudents(
	requests []models.CreateStudentRequest,
) *models.BatchUploadResponse {

	resp := &models.BatchUploadResponse{
		TotalRows: len(requests),
	}

	tx, err := s.db.Beginx()
	if err != nil {
		resp.Failed = len(requests)
		resp.Errors = append(resp.Errors, models.BatchUploadError{
			Row:   0,
			Error: "failed to start transaction",
		})
		return resp
	}
	defer tx.Rollback()

	for i, req := range requests {
		rowNum := i + 1
		req.PRN = strings.TrimSpace(req.PRN)
		req.Name = strings.TrimSpace(req.Name)
		req.GuideName = strings.TrimSpace(req.GuideName)
		req.Division = strings.ToUpper(strings.TrimSpace(req.Division))

		if req.PRN == "" || req.Name == "" {
			resp.Errors = append(resp.Errors, models.BatchUploadError{
				Row:   rowNum,
				Error: "missing required fields",
			})
			resp.Failed = len(requests)
			return resp // FAIL FAST
		}

		_, err := tx.Exec(`
			INSERT INTO students (prn, name, guide_name, passing_year, division)
			VALUES ($1,$2,$3,$4,$5)
		`, req.PRN, req.Name, req.GuideName, req.PassingYear, req.Division)

		if err != nil {
			resp.Errors = append(resp.Errors, models.BatchUploadError{
				Row:   rowNum,
				Error: "insert failed (duplicate PRN or constraint violation)",
			})
			resp.Failed = len(requests)
			return resp // FAIL FAST
		}

		resp.Inserted++
	}

	// Only reached if ALL rows succeeded
	if err := tx.Commit(); err != nil {
		resp.Inserted = 0
		resp.Failed = len(requests)
		resp.Errors = append(resp.Errors, models.BatchUploadError{
			Row:   0,
			Error: "transaction commit failed",
		})
		return resp
	}

	return resp
}
