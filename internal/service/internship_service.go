package service

import (
	"fmt"
	"strconv"
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
) (*models.CreateInternshipResponse, error) {

	startDate, endDate, err := s.validateAndPrepareInternship(req)
	if err != nil {
		return nil, err
	}

	warnings, err := s.detectSubmissionWarnings(req.PRN, req.Organization, startDate, endDate)
	if err != nil {
		return nil, err
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
			workflow_status,
			credit_eligible,
			created_by
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'pending','certificate_pending',FALSE,$9)
		RETURNING id
	`

	var internshipID int
	err = s.db.Get(
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

	if err != nil {
		return nil, err
	}
	_ = logInternshipAudit(s.db, internshipID, "internship_created", req.Description, &createdBy)

	return &models.CreateInternshipResponse{
		Message:  "Internship created successfully",
		Warnings: warnings,
	}, nil
}

func isEmptyInternshipRow(req *models.CreateInternshipRequest) bool {
	return strings.TrimSpace(req.Organization) == "" &&
		strings.TrimSpace(req.StartDate) == "" &&
		strings.TrimSpace(req.EndDate) == "" &&
		req.Credits == 0 &&
		strings.TrimSpace(req.Mode) == "" &&
		req.MonthlyStipend == 0
}

func makeBatchError(processedRow int, sheetRow int, message string) models.BatchUploadError {
	return models.BatchUploadError{
		Row:          processedRow,
		ProcessedRow: processedRow,
		SheetRow:     sheetRow,
		Error:        message,
	}
}

func missingRequiredFields(req *models.CreateInternshipRequest) []string {
	var missing []string
	if strings.TrimSpace(req.PRN) == "" {
		missing = append(missing, "prn")
	}
	if strings.TrimSpace(req.Organization) == "" {
		missing = append(missing, "organization")
	}
	if strings.TrimSpace(req.StartDate) == "" {
		missing = append(missing, "startDate")
	}
	if strings.TrimSpace(req.EndDate) == "" {
		missing = append(missing, "endDate")
	}
	if strings.TrimSpace(req.Mode) == "" {
		missing = append(missing, "mode")
	}
	if req.Credits <= 0 {
		missing = append(missing, "credits")
	}
	return missing
}

func classifyBatchValidationError(req *models.CreateInternshipRequest, err error) (category, field, rawValue, suggestion string) {
	msg := strings.ToLower(strings.TrimSpace(err.Error()))

	if strings.Contains(msg, "student with prn") && strings.Contains(msg, "not found") {
		return "reference_not_found", "prn", req.PRN, "Upload student master first or correct PRN in the sheet"
	}
	if strings.Contains(msg, "invalid internship mode") {
		return "invalid_value", "mode", req.RawMode, "Use one of: online, offline, hybrid"
	}
	if strings.Contains(msg, "invalid start date format") {
		return "invalid_value", "startDate", req.RawStartDate, "Use YYYY-MM-DD or a valid Excel date value"
	}
	if strings.Contains(msg, "invalid end date format") {
		return "invalid_value", "endDate", req.RawEndDate, "Use YYYY-MM-DD or a valid Excel date value"
	}
	if strings.Contains(msg, "end date must be after or equal to start date") {
		return "invalid_value", "endDate", req.RawEndDate, "End date must be on or after start date"
	}
	if strings.Contains(msg, "credits must be greater than 0") {
		return "invalid_value", "credits", strconv.Itoa(req.Credits), "Provide credits greater than zero"
	}
	if strings.Contains(msg, "monthly stipend cannot be negative") {
		return "invalid_value", "monthlyStipend", fmt.Sprintf("%g", req.MonthlyStipend), "Provide stipend >= 0"
	}
	return "validation_error", "", "", ""
}

// func (s *InternshipService) BatchCreateInternships(
// 	requests []models.CreateInternshipRequest,
// 	createdBy int,
// ) *models.BatchUploadResponse {

// 	response := &models.BatchUploadResponse{
// 		TotalRows: len(requests),
// 	}

// 	tx, err := s.db.Beginx()
// 	if err != nil {
// 		response.Failed = len(requests)
// 		response.Errors = append(response.Errors, models.BatchUploadError{
// 			Row:   0,
// 			Error: "failed to start database transaction",
// 		})
// 		return response
// 	}
// 	defer tx.Rollback()

// 	for i, req := range requests {
// 		rowNum := i + 1

// 		// sanitization
// 		req.PRN = strings.TrimSpace(req.PRN)
// 		req.Organization = strings.TrimSpace(req.Organization)
// 		req.Description = strings.TrimSpace(req.Description)
// 		req.Mode = strings.ToLower(strings.TrimSpace(req.Mode))

// 		startDate, endDate, err := s.validateAndPrepareInternship(&req)
// 		if err != nil {
// 			response.Failed = len(requests)
// 			response.Errors = append(response.Errors, models.BatchUploadError{
// 				Row:   rowNum,
// 				Error: err.Error(),
// 			})
// 			return response
// 		}

// 		_, err = tx.Exec(`
// 			INSERT INTO internships (
// 				student_prn,
// 				organization,
// 				description,
// 				start_date,
// 				end_date,
// 				mode,
// 				credits,
// 				monthly_stipend,
// 				status,
// 				credit_eligible,
// 				created_by
// 			)
// 			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'pending',FALSE,$9)
// 		`,
// 			req.PRN,
// 			req.Organization,
// 			req.Description,
// 			startDate,
// 			endDate,
// 			req.Mode,
// 			req.Credits,
// 			req.MonthlyStipend,
// 			createdBy,
// 		)

// 		if err != nil {
// 			response.Failed = len(requests)
// 			response.Errors = append(response.Errors, models.BatchUploadError{
// 				Row:   rowNum,
// 				Error: "database insert failed",
// 			})
// 			return response
// 		}

// 		response.Inserted++
// 	}

// 	if err := tx.Commit(); err != nil {
// 		response.Failed = len(requests)
// 		response.Inserted = 0
// 		response.Errors = append(response.Errors, models.BatchUploadError{
// 			Row:   0,
// 			Error: "failed to commit transaction",
// 		})
// 	}

// 	return response
// }

func (s *InternshipService) BatchCreateInternships(
	requests []models.CreateInternshipRequest,
	createdBy int,
) *models.BatchUploadResponse {

	response := &models.BatchUploadResponse{
		TotalRows: len(requests),
	}

	type preparedInternship struct {
		req       models.CreateInternshipRequest
		startDate time.Time
		endDate   time.Time
	}
	prepared := make([]preparedInternship, 0, len(requests))

	for i, req := range requests {
		rowNum := i + 1
		if req.ProcessedRow > 0 {
			rowNum = req.ProcessedRow
		}
		sheetRow := req.SheetRow

		// Sanitization
		req.PRN = strings.TrimSpace(req.PRN)
		req.Organization = strings.TrimSpace(req.Organization)
		req.Description = strings.TrimSpace(req.Description)
		req.Mode = strings.ToLower(strings.TrimSpace(req.Mode))

		// 🚀 Skip completely empty internship rows
		if isEmptyInternshipRow(&req) {
			continue
		}

		missing := missingRequiredFields(&req)
		if len(missing) > 0 {
			errItem := makeBatchError(rowNum, sheetRow, "incomplete internship row")
			errItem.Category = "incomplete_row"
			errItem.Field = strings.Join(missing, ",")
			errItem.Suggestion = "Fill required fields: prn, organization, startDate, endDate, mode, credits"
			response.Errors = append(response.Errors, errItem)
			continue
		}

		startDate, endDate, err := s.validateAndPrepareInternship(&req)
		if err != nil {
			errItem := makeBatchError(rowNum, sheetRow, err.Error())
			errItem.Category, errItem.Field, errItem.RawValue, errItem.Suggestion = classifyBatchValidationError(&req, err)
			response.Errors = append(response.Errors, errItem)
			continue
		}

		warnings, warnErr := s.detectSubmissionWarnings(req.PRN, req.Organization, startDate, endDate)
		if warnErr != nil {
			errItem := makeBatchError(rowNum, sheetRow, "failed to evaluate submission warnings")
			errItem.Category = "processing_error"
			response.Errors = append(response.Errors, errItem)
			continue
		}
		if len(warnings) > 0 {
			response.Warnings = append(response.Warnings, models.BatchUploadWarning{
				Row:          rowNum,
				ProcessedRow: rowNum,
				SheetRow:     sheetRow,
				Message:      "potential conflict detected",
				Items:        warnings,
			})
		}

		prepared = append(prepared, preparedInternship{
			req:       req,
			startDate: startDate,
			endDate:   endDate,
		})
	}

	if len(response.Errors) > 0 {
		response.Inserted = 0
		response.Failed = len(response.Errors)
		summary := makeBatchError(0, 0, "batch aborted: fix listed rows and retry; no internships were inserted")
		summary.Category = "batch_aborted"
		summary.Suggestion = "Resolve all row errors first, then upload again"
		response.Errors = append([]models.BatchUploadError{summary}, response.Errors...)
		return response
	}

	tx, err := s.db.Beginx()
	if err != nil {
		response.Failed = len(requests)
		response.Errors = append(response.Errors, makeBatchError(0, 0, "failed to start database transaction"))
		return response
	}
	defer tx.Rollback()

	for _, item := range prepared {
		var insertedID int
		err := tx.Get(&insertedID, `
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
					workflow_status,
					credit_eligible,
					created_by
				)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'pending','certificate_pending',FALSE,$9)
				RETURNING id
			`,
			item.req.PRN,
			item.req.Organization,
			item.req.Description,
			item.startDate,
			item.endDate,
			item.req.Mode,
			item.req.Credits,
			item.req.MonthlyStipend,
			createdBy,
		)

		if err != nil {
			rowNum := item.req.ProcessedRow
			if rowNum == 0 {
				rowNum = 1
			}
			errItem := makeBatchError(rowNum, item.req.SheetRow, "database insert failed")
			errItem.Category = "processing_error"
			errItem.Suggestion = "Check duplicate records/constraints and retry"
			response.Errors = append(response.Errors, errItem)
			response.Inserted = 0
			response.Failed = len(prepared)
			return response
		}
		_ = logInternshipAudit(tx, insertedID, "internship_created", item.req.Description, &createdBy)

		response.Inserted++
	}

	// Commit only after all rows inserted successfully.
	if err := tx.Commit(); err != nil {
		response.Inserted = 0
		response.Failed = len(prepared)
		errItem := makeBatchError(0, 0, "failed to commit transaction")
		errItem.Category = "processing_error"
		response.Errors = append(response.Errors, errItem)
	}

	return response
}

func (s *InternshipService) ApproveInternship(
	internshipID int,
	approvedBy int,
	reviewNote string,
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
		    workflow_status = 'approved',
		    approved_by = $1,
		    approved_at = NOW(),
		    review_note = NULLIF(TRIM($3), '')
		WHERE id = $2
		  AND status = 'pending'
		RETURNING COALESCE(student_prn, '')
	`, approvedBy, internshipID, reviewNote)
	if err != nil {
		return fmt.Errorf("internship not found or already processed")
	}
	_ = logInternshipAudit(tx, internshipID, "approved", reviewNote, &approvedBy)

	if prn != "" {
		if err := s.recalculateCreditEligibilityTx(tx, prn); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *InternshipService) RejectInternship(
	internshipID int,
	approvedBy int,
	reviewNote string,
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
		    workflow_status = 'rejected',
		    approved_by = $1,
		    approved_at = NOW(),
		    review_note = NULLIF(TRIM($3), '')
		WHERE id = $2
		  AND status = 'pending'
		RETURNING COALESCE(student_prn, '')
	`, approvedBy, internshipID, reviewNote)
	if err != nil {
		return fmt.Errorf("internship not found or already processed")
	}
	_ = logInternshipAudit(tx, internshipID, "rejected", reviewNote, &approvedBy)

	// Approved competitor removed → recalc
	if prn != "" {
		if err := s.recalculateCreditEligibilityTx(tx, prn); err != nil {
			return err
		}
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
		// Treat touching ranges as overlapping (inclusive boundary), matching upload overlap detection.
		for j < n && !internships[j].StartDate.After(groupEnd) {
			group = append(group, internships[j])
			if internships[j].EndDate.After(groupEnd) {
				groupEnd = internships[j].EndDate
			}
			j++
		}

		longestID := group[0].ID
		// Inclusive day-span so a same-day internship counts as 1 day.
		maxDuration := int(group[0].EndDate.Sub(group[0].StartDate).Hours()/24) + 1

		for _, in := range group {
			dur := int(in.EndDate.Sub(in.StartDate).Hours()/24) + 1
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

// func (s *InternshipService) validateAndPrepareInternship(
// 	req *models.CreateInternshipRequest,
// ) (time.Time, time.Time, error) {

// 	exists, err := s.studentService.StudentExists(req.PRN)
// 	if err != nil {
// 		return time.Time{}, time.Time{}, err
// 	}
// 	if !exists {
// 		return time.Time{}, time.Time{}, fmt.Errorf("student with PRN %s not found", req.PRN)
// 	}

// 	startDate, err := time.Parse("2006-01-02", req.StartDate)
// 	if err != nil {
// 		return time.Time{}, time.Time{}, fmt.Errorf("invalid start date format")
// 	}

// 	endDate, err := time.Parse("2006-01-02", req.EndDate)
// 	if err != nil {
// 		return time.Time{}, time.Time{}, fmt.Errorf("invalid end date format")
// 	}

// 	if endDate.Before(startDate) {
// 		return time.Time{}, time.Time{}, fmt.Errorf("end date must be after or equal to start date")
// 	}

// 	if req.Credits <= 0 {
// 		return time.Time{}, time.Time{}, fmt.Errorf("credits must be greater than 0")
// 	}

// 	switch req.Mode {
// 	case "online", "offline", "hybrid":
// 	default:
// 		return time.Time{}, time.Time{}, fmt.Errorf("invalid internship mode")
// 	}

// 	if req.MonthlyStipend < 0 {
// 		return time.Time{}, time.Time{}, fmt.Errorf("monthly stipend cannot be negative")
// 	}

// 	return startDate, endDate, nil
// }

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

	parseFlexibleDate := func(input string) (time.Time, error) {
		input = strings.TrimSpace(input)

		formats := []string{
			"2006-01-02",
			"02-01-2006",
			"02/01/2006",
			"2006/01/02",
			"2-1-2006",
			"2/1/2006",
		}

		for _, f := range formats {
			if t, err := time.Parse(f, input); err == nil {
				return t, nil
			}
		}

		return time.Time{}, fmt.Errorf("invalid date format")
	}

	startDate, err := parseFlexibleDate(req.StartDate)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start date format")
	}

	endDate, err := parseFlexibleDate(req.EndDate)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end date format")
	}

	if endDate.Before(startDate) {
		return time.Time{}, time.Time{}, fmt.Errorf("end date must be after or equal to start date")
	}

	if req.Credits <= 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("credits must be greater than 0")
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	switch mode {
	case "online", "offline", "hybrid":
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("invalid internship mode")
	}
	req.Mode = mode

	if req.MonthlyStipend < 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("monthly stipend cannot be negative")
	}

	return startDate, endDate, nil
}

func (s *InternshipService) detectSubmissionWarnings(prn, organization string, startDate, endDate time.Time) ([]string, error) {
	var warnings []string

	var overlapCount int
	err := s.db.Get(&overlapCount, `
		SELECT COUNT(1)
		FROM internships
		WHERE student_prn = $1
		  AND status IN ('pending', 'approved')
		  AND daterange(start_date, end_date, '[]') && daterange($2::date, $3::date, '[]')
	`, prn, startDate, endDate)
	if err != nil {
		return nil, err
	}
	if overlapCount > 0 {
		warnings = append(warnings, "date overlap with an existing pending/approved internship")
	}

	var duplicateCount int
	err = s.db.Get(&duplicateCount, `
		SELECT COUNT(1)
		FROM internships
		WHERE student_prn = $1
		  AND status IN ('pending', 'approved')
		  AND LOWER(TRIM(organization)) = LOWER(TRIM($2))
		  AND start_date = $3::date
		  AND end_date = $4::date
	`, prn, organization, startDate, endDate)
	if err != nil {
		return nil, err
	}
	if duplicateCount > 0 {
		warnings = append(warnings, "possible duplicate internship (same organization and date range)")
	}

	return warnings, nil
}

func (s *InternshipService) ListInternships(
	page int,
	pageSize int,
	status string,
	workflowStatus string,
	organization string,
	guide string,
	prn string,
	dateFrom string,
	dateTo string,
	year *int,
	division string,
) (*models.InternshipListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	whereClause, args, argPos := buildInternshipListWhereClause(
		status,
		workflowStatus,
		organization,
		guide,
		prn,
		dateFrom,
		dateTo,
		year,
		division,
	)

	countQuery := `
		SELECT COUNT(1)
		FROM internships i
		JOIN students s ON s.prn = i.student_prn
	` + whereClause

	var total int
	if err := s.db.Get(&total, countQuery, args...); err != nil {
		return nil, fmt.Errorf("failed to count internships: %w", err)
	}

	offset := (page - 1) * pageSize
	dataArgs := append(args, pageSize, offset)
	query := `
		SELECT
			i.id,
			i.student_prn,
			i.organization,
			i.description,
			s.guide_name,
			i.start_date,
			i.end_date,
			i.mode,
			i.credits,
			i.monthly_stipend,
			i.stipend_currency,
			i.status,
			i.workflow_status,
			i.created_by,
			i.created_at,
			i.approved_by,
			i.approved_at,
			i.review_note,
			s.name AS student_name,
			s.passing_year AS year,
			s.division
		FROM internships i
		JOIN students s ON i.student_prn = s.prn
	` + whereClause + `
		ORDER BY i.created_at DESC
		LIMIT $` + strconv.Itoa(argPos) + ` OFFSET $` + strconv.Itoa(argPos+1)

	var items []models.InternshipWithStudentName
	if err := s.db.Select(&items, query, dataArgs...); err != nil {
		return nil, fmt.Errorf("failed to list internships: %w", err)
	}

	return &models.InternshipListResponse{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func (s *InternshipService) ListInternshipsForExport(
	status string,
	workflowStatus string,
	organization string,
	guide string,
	prn string,
	dateFrom string,
	dateTo string,
	year *int,
	division string,
) ([]models.InternshipWithStudentName, error) {
	whereClause, args, _ := buildInternshipListWhereClause(
		status,
		workflowStatus,
		organization,
		guide,
		prn,
		dateFrom,
		dateTo,
		year,
		division,
	)

	query := `
		SELECT
			i.id,
			i.student_prn,
			i.organization,
			i.description,
			s.guide_name,
			i.start_date,
			i.end_date,
			i.mode,
			i.credits,
			i.monthly_stipend,
			i.stipend_currency,
			i.status,
			i.workflow_status,
			i.created_by,
			i.created_at,
			i.approved_by,
			i.approved_at,
			i.review_note,
			s.name AS student_name,
			s.passing_year AS year,
			s.division
		FROM internships i
		JOIN students s ON i.student_prn = s.prn
	` + whereClause + `
		ORDER BY i.created_at DESC
	`

	var items []models.InternshipWithStudentName
	if err := s.db.Select(&items, query, args...); err != nil {
		return nil, fmt.Errorf("failed to list internships for export: %w", err)
	}
	return items, nil
}

func buildInternshipListWhereClause(
	status string,
	workflowStatus string,
	organization string,
	guide string,
	prn string,
	dateFrom string,
	dateTo string,
	year *int,
	division string,
) (string, []interface{}, int) {
	where := []string{"1=1"}
	args := []interface{}{}
	argPos := 1

	if status != "" {
		where = append(where, "i.status = $"+strconv.Itoa(argPos))
		args = append(args, status)
		argPos++
	}
	if strings.TrimSpace(workflowStatus) != "" {
		where = append(where, "i.workflow_status = $"+strconv.Itoa(argPos))
		args = append(args, strings.TrimSpace(workflowStatus))
		argPos++
	}
	if organization != "" {
		where = append(where, "LOWER(i.organization) LIKE LOWER($"+strconv.Itoa(argPos)+")")
		args = append(args, "%"+strings.TrimSpace(organization)+"%")
		argPos++
	}
	if guide != "" {
		where = append(where, "LOWER(s.guide_name) LIKE LOWER($"+strconv.Itoa(argPos)+")")
		args = append(args, "%"+strings.TrimSpace(guide)+"%")
		argPos++
	}
	if prn != "" {
		where = append(where, "LOWER(i.student_prn) LIKE LOWER($"+strconv.Itoa(argPos)+")")
		args = append(args, "%"+strings.TrimSpace(prn)+"%")
		argPos++
	}
	if dateFrom != "" {
		where = append(where, "i.start_date >= $"+strconv.Itoa(argPos)+"::date")
		args = append(args, dateFrom)
		argPos++
	}
	if dateTo != "" {
		where = append(where, "i.end_date <= $"+strconv.Itoa(argPos)+"::date")
		args = append(args, dateTo)
		argPos++
	}
	if year != nil {
		where = append(where, "s.passing_year = $"+strconv.Itoa(argPos))
		args = append(args, *year)
		argPos++
	}
	if strings.TrimSpace(division) != "" {
		where = append(where, "LOWER(s.division) = LOWER($"+strconv.Itoa(argPos)+")")
		args = append(args, strings.TrimSpace(division))
		argPos++
	}

	return " WHERE " + strings.Join(where, " AND "), args, argPos
}

func (s *InternshipService) BulkReviewInternships(req *models.BulkReviewRequest, approvedBy int) (*models.BulkReviewResponse, error) {
	if !req.Confirm {
		return nil, fmt.Errorf("bulk operation requires explicit confirmation")
	}
	if len(req.InternshipIDs) == 0 {
		return nil, fmt.Errorf("no internship IDs provided")
	}
	if len(req.InternshipIDs) > 50 {
		return nil, fmt.Errorf("bulk operation limit exceeded (max 50)")
	}

	ids := uniqueInts(req.InternshipIDs)
	if len(ids) == 0 {
		return nil, fmt.Errorf("no valid internship IDs provided")
	}

	tx, err := s.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	query, args, err := sqlx.In(`
		SELECT id, student_prn
		FROM internships
		WHERE id IN (?)
		  AND status = 'pending'
		FOR UPDATE
	`, ids)
	if err != nil {
		return nil, err
	}
	query = tx.Rebind(query)

	type row struct {
		ID  int    `db:"id"`
		PRN string `db:"student_prn"`
	}
	var rows []row
	if err := tx.Select(&rows, query, args...); err != nil {
		return nil, err
	}
	if len(rows) != len(ids) {
		return nil, fmt.Errorf("some internships are missing or already processed")
	}

	status := "approved"
	if req.Action == "reject" {
		status = "rejected"
	}

	updateQuery, updateArgs, err := sqlx.In(`
		UPDATE internships
		SET status = ?,
		    workflow_status = ?,
		    approved_by = ?,
		    approved_at = NOW(),
		    review_note = NULLIF(TRIM(?), '')
		WHERE id IN (?)
		  AND status = 'pending'
	`, status, status, approvedBy, req.ReviewNote, ids)
	if err != nil {
		return nil, err
	}
	updateQuery = tx.Rebind(updateQuery)

	res, err := tx.Exec(updateQuery, updateArgs...)
	if err != nil {
		return nil, err
	}
	affected, _ := res.RowsAffected()
	if int(affected) != len(ids) {
		return nil, fmt.Errorf("bulk operation aborted because records changed during processing")
	}

	prnSet := map[string]struct{}{}
	for _, r := range rows {
		prnSet[r.PRN] = struct{}{}
	}
	for studentPRN := range prnSet {
		if err := s.recalculateCreditEligibilityTx(tx, studentPRN); err != nil {
			return nil, err
		}
	}
	auditAction := "approved"
	if req.Action == "reject" {
		auditAction = "rejected"
	}
	for _, id := range ids {
		_ = logInternshipAudit(tx, id, auditAction, req.ReviewNote, &approvedBy)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	actionLabel := "Approved"
	if req.Action == "reject" {
		actionLabel = "Rejected"
	}

	return &models.BulkReviewResponse{
		Message:       fmt.Sprintf("%s operation completed", actionLabel),
		ProcessedRows: len(ids),
	}, nil
}

func uniqueInts(input []int) []int {
	seen := make(map[int]struct{}, len(input))
	out := make([]int, 0, len(input))
	for _, id := range input {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *InternshipService) GetInternshipAudit(internshipID int) ([]models.InternshipAuditEvent, error) {
	var logs []models.InternshipAuditEvent
	if err := s.db.Select(&logs, `
		SELECT id, internship_id, action, COALESCE(note, '') AS note, actor_user_id, actor_role, created_at
		FROM internship_audit_logs
		WHERE internship_id = $1
		ORDER BY created_at DESC
	`, internshipID); err != nil {
		return nil, fmt.Errorf("failed to load internship audit trail: %w", err)
	}
	if logs == nil {
		return []models.InternshipAuditEvent{}, nil
	}
	return logs, nil
}

// GetPendingInternships returns all pending internships
func (s *InternshipService) GetPendingInternships() ([]models.InternshipWithStudentName, error) {
	resp, err := s.ListInternships(1, 1000, "pending", "", "", "", "", "", "", nil, "")
	if err != nil {
		return nil, err
	}
	return resp.Items, nil
}
