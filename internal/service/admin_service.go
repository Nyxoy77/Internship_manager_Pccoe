package service

import (
	"fmt"
	"strconv"
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

	seenPRN := map[string]int{}
	valid := make([]models.CreateStudentRequest, 0, len(requests))

	for i, req := range requests {
		rowNum := i + 1
		if req.ProcessedRow > 0 {
			rowNum = req.ProcessedRow
		}
		sheetRow := req.SheetRow
		req.PRN = strings.TrimSpace(req.PRN)
		req.Name = strings.TrimSpace(req.Name)
		req.GuideName = strings.TrimSpace(req.GuideName)
		req.Division = strings.ToUpper(strings.TrimSpace(req.Division))

		missing := []string{}
		if req.PRN == "" {
			missing = append(missing, "prn")
		}
		if req.Name == "" {
			missing = append(missing, "name")
		}
		if req.Division == "" {
			missing = append(missing, "division")
		}
		if req.PassingYear == 0 {
			missing = append(missing, "passing_year")
		}
		if len(missing) > 0 {
			resp.Errors = append(resp.Errors, models.BatchUploadError{
				Row:          rowNum,
				ProcessedRow: rowNum,
				SheetRow:     sheetRow,
				Category:     "incomplete_row",
				Field:        strings.Join(missing, ","),
				Suggestion:   "Fill required fields: prn, name, passing_year, division",
				Error:        "missing required fields",
			})
			continue
		}
		if len(req.PRN) > 20 {
			resp.Errors = append(resp.Errors, models.BatchUploadError{
				Row:          rowNum,
				ProcessedRow: rowNum,
				SheetRow:     sheetRow,
				Category:     "invalid_value",
				Field:        "prn",
				RawValue:     req.RawPRN,
				Suggestion:   "PRN must be 20 characters or fewer",
				Error:        "invalid PRN format",
			})
			continue
		}
		if req.PassingYear < 2000 || req.PassingYear > 2100 {
			rawYear := req.RawYear
			if rawYear == "" {
				rawYear = strconv.Itoa(req.PassingYear)
			}
			resp.Errors = append(resp.Errors, models.BatchUploadError{
				Row:          rowNum,
				ProcessedRow: rowNum,
				SheetRow:     sheetRow,
				Category:     "invalid_value",
				Field:        "passing_year",
				RawValue:     rawYear,
				Suggestion:   "Use a valid year between 2000 and 2100",
				Error:        "invalid passing year",
			})
			continue
		}
		switch req.Division {
		case "A", "B", "C", "D":
		default:
			rawDivision := req.RawDivision
			if rawDivision == "" {
				rawDivision = req.Division
			}
			resp.Errors = append(resp.Errors, models.BatchUploadError{
				Row:          rowNum,
				ProcessedRow: rowNum,
				SheetRow:     sheetRow,
				Category:     "invalid_value",
				Field:        "division",
				RawValue:     rawDivision,
				Suggestion:   "Use one of: A, B, C, D",
				Error:        "invalid division",
			})
			continue
		}
		if len(req.GuideName) > 120 {
			resp.Errors = append(resp.Errors, models.BatchUploadError{
				Row:          rowNum,
				ProcessedRow: rowNum,
				SheetRow:     sheetRow,
				Category:     "invalid_value",
				Field:        "guide_name",
				RawValue:     req.RawGuideName,
				Suggestion:   "Guide name should be 120 characters or fewer",
				Error:        "guide name too long",
			})
			continue
		}
		if prevRow, exists := seenPRN[req.PRN]; exists {
			resp.Errors = append(resp.Errors, models.BatchUploadError{
				Row:          rowNum,
				ProcessedRow: rowNum,
				SheetRow:     sheetRow,
				Category:     "duplicate_in_file",
				Field:        "prn",
				RawValue:     req.PRN,
				Suggestion:   fmt.Sprintf("Remove duplicate PRN from this upload (first seen at processed row %d)", prevRow),
				Error:        "duplicate PRN in upload file",
			})
			continue
		}
		seenPRN[req.PRN] = rowNum
		valid = append(valid, req)
	}

	for _, req := range valid {
		var exists bool
		err := s.db.Get(&exists, `SELECT EXISTS (SELECT 1 FROM students WHERE prn = $1)`, req.PRN)
		if err != nil {
			resp.Errors = append(resp.Errors, models.BatchUploadError{
				Row:          req.ProcessedRow,
				ProcessedRow: req.ProcessedRow,
				SheetRow:     req.SheetRow,
				Category:     "processing_error",
				Error:        "failed to validate PRN uniqueness in database",
			})
			continue
		}
		if exists {
			resp.Errors = append(resp.Errors, models.BatchUploadError{
				Row:          req.ProcessedRow,
				ProcessedRow: req.ProcessedRow,
				SheetRow:     req.SheetRow,
				Category:     "duplicate_in_db",
				Field:        "prn",
				RawValue:     req.PRN,
				Suggestion:   "PRN already exists in student master",
				Error:        "student with this PRN already exists",
			})
		}
	}

	if len(resp.Errors) > 0 {
		resp.Inserted = 0
		resp.Failed = len(resp.Errors)
		resp.Errors = append([]models.BatchUploadError{{
			Row:          0,
			ProcessedRow: 0,
			SheetRow:     0,
			Category:     "batch_aborted",
			Error:        "batch aborted: fix listed rows and retry; no students were inserted",
			Suggestion:   "Resolve all row errors first, then upload again",
		}}, resp.Errors...)
		return resp
	}

	tx, err := s.db.Beginx()
	if err != nil {
		resp.Failed = len(valid)
		resp.Errors = append(resp.Errors, models.BatchUploadError{
			Row:          0,
			ProcessedRow: 0,
			SheetRow:     0,
			Category:     "processing_error",
			Error:        "failed to start transaction",
		})
		return resp
	}
	defer tx.Rollback()

	for _, req := range valid {
		_, err := tx.Exec(`
			INSERT INTO students (prn, name, guide_name, passing_year, division)
			VALUES ($1,$2,$3,$4,$5)
		`, req.PRN, req.Name, req.GuideName, req.PassingYear, req.Division)
		if err != nil {
			resp.Inserted = 0
			resp.Failed = len(valid)
			resp.Errors = append(resp.Errors, models.BatchUploadError{
				Row:          req.ProcessedRow,
				ProcessedRow: req.ProcessedRow,
				SheetRow:     req.SheetRow,
				Category:     "processing_error",
				Error:        "insert failed while applying transaction",
				Suggestion:   "Retry upload after resolving duplicate/constraint issues",
			})
			return resp
		}
		resp.Inserted++
	}

	if err := tx.Commit(); err != nil {
		resp.Inserted = 0
		resp.Failed = len(valid)
		resp.Errors = append(resp.Errors, models.BatchUploadError{
			Row:          0,
			ProcessedRow: 0,
			SheetRow:     0,
			Category:     "processing_error",
			Error:        "transaction commit failed",
		})
		return resp
	}

	return resp
}
