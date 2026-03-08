package client

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"

	"github.com/yourusername/student-internship-manager/internal/models"
	"github.com/yourusername/student-internship-manager/internal/service"
)

type InternshipHandler struct {
	internshipService *service.InternshipService
}

func NewInternshipHandler(internshipService *service.InternshipService) *InternshipHandler {
	return &InternshipHandler{
		internshipService: internshipService,
	}
}

// CreateInternship handles single internship creation
func (h *InternshipHandler) CreateInternship(c *gin.Context) {
	// Extract userID from context (set by middleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	createdBy, ok := userID.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	var req models.CreateInternshipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	response, err := h.internshipService.CreateInternship(&req, createdBy)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "must be") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		log.Printf("failed to create internship : %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create internship"})
		return
	}

	c.JSON(http.StatusCreated, response)
}

// BatchUploadInternships handles CSV/XLSX file upload [web:10]
func (h *InternshipHandler) BatchUploadInternships(c *gin.Context) {
	// Extract userID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	createdBy, ok := userID.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get uploaded file (support both "file" and legacy "File")
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		file, header, err = c.Request.FormFile("File")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
			return
		}
	}
	defer file.Close()

	// Determine file type
	filename := header.Filename
	var requests []models.CreateInternshipRequest

	if strings.HasSuffix(strings.ToLower(filename), ".csv") {
		requests, err = h.parseCSV(file)
	} else if strings.HasSuffix(strings.ToLower(filename), ".xlsx") {
		requests, err = h.parseXLSX(file)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only .csv and .xlsx files are supported"})
		return
	}

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to parse file: %v", err)})
		return
	}

	// Process batch upload
	response := h.internshipService.BatchCreateInternships(requests, createdBy)

	c.JSON(http.StatusOK, response)
}

// parseCSV parses CSV file and returns internship requests
// func (h *InternshipHandler) parseCSV(file io.Reader) ([]models.CreateInternshipRequest, error) {
// 	reader := csv.NewReader(file)
// 	records, err := reader.ReadAll()
// 	if err != nil {
// 		return nil, err
// 	}

// 	if len(records) < 2 {
// 		return nil, fmt.Errorf("file must contain header and at least one data row")
// 	}

// 	// Skip header row
// 	var requests []models.CreateInternshipRequest
// 	for i := 1; i < len(records); i++ {
// 		record := records[i]
// 		if len(record) < 6 {
// 			continue
// 		}

// 		credits, _ := strconv.Atoi(record[5])
// 		raw := strings.TrimSpace(record[7])

// 		stipend, err := strconv.ParseFloat(raw, 64)
// 		if err != nil {
// 			return nil, fmt.Errorf("invalid stipend value '%s'", raw)
// 		}

// 		// Intentionally ignored the student name as it is not required for updation. Prn forms single source of truth
// 		req := models.CreateInternshipRequest{
// 			PRN:            strings.TrimSpace(record[0]),
// 			Organization:   strings.TrimSpace(record[2]),
// 			StartDate:      strings.TrimSpace(record[3]),
// 			EndDate:        strings.TrimSpace(record[4]),
// 			Credits:        credits,
// 			Mode:           strings.TrimSpace(record[6]),
// 			MonthlyStipend: stipend,
// 			Description:    strings.TrimSpace(record[8]),
// 		}

// 		// if len(record) > 6 {
// 		// 	req.Description = strings.TrimSpace(record[6])
// 		// }

// 		requests = append(requests, req)
// 	}

// 	return requests, nil
// }

func (h *InternshipHandler) parseCSV(file io.Reader) ([]models.CreateInternshipRequest, error) {
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("file must contain header and at least one data row")
	}

	index := buildInternshipColumnIndex(records[0])
	// Backward compatible fallback for the old fixed-column upload format.
	if !index.hasHeaders {
		var requests []models.CreateInternshipRequest
		for i := 1; i < len(records); i++ {
			record := records[i]
			if len(record) == 0 {
				continue
			}
			credits := parseCreditsValue(safeGet(record, 5))
			stipend, _ := normalizeStipend(safeGet(record, 7))
			rawStartDate := safeGet(record, 3)
			rawEndDate := safeGet(record, 4)
			rawMode := safeGet(record, 6)
			req := models.CreateInternshipRequest{
				PRN:            safeGet(record, 0),
				Organization:   safeGet(record, 2),
				StartDate:      parseDateValue(rawStartDate),
				EndDate:        parseDateValue(rawEndDate),
				Credits:        credits,
				Mode:           normalizeMode(rawMode),
				MonthlyStipend: stipend,
				Description:    safeGet(record, 8),
				RawStartDate:   rawStartDate,
				RawEndDate:     rawEndDate,
				RawMode:        rawMode,
			}
			if isRowEffectivelyEmpty(req) {
				continue
			}
			req.ProcessedRow = len(requests) + 1
			req.SheetRow = i + 1
			requests = append(requests, req)
		}
		return requests, nil
	}

	var requests []models.CreateInternshipRequest
	for i := 1; i < len(records); i++ {
		req := mapRowToInternshipRequest(records[i], index)
		if isRowEffectivelyEmpty(req) {
			continue
		}
		req.ProcessedRow = len(requests) + 1
		req.SheetRow = i + 1
		requests = append(requests, req)
	}

	return requests, nil
}

func safeGet(row []string, index int) string {
	if index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func normalizeMode(mode string) string {
	return strings.ToLower(strings.TrimSpace(mode))
}

func normalizeStipend(raw string) (float64, error) {
	clean := strings.TrimSpace(raw)
	clean = strings.ReplaceAll(clean, "/-", "")
	clean = strings.ReplaceAll(clean, ",", "")
	if clean == "" {
		return 0, nil
	}
	return strconv.ParseFloat(clean, 64)
}

// parseXLSX parses Excel file and returns internship requests
// func (h *InternshipHandler) parseXLSX(file io.Reader) ([]models.CreateInternshipRequest, error) {
// 	// Read file into memory
// 	data, err := io.ReadAll(file)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Open Excel file
// 	f, err := excelize.OpenReader(strings.NewReader(string(data)))
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer f.Close()

// 	// Get first sheet
// 	sheets := f.GetSheetList()
// 	if len(sheets) == 0 {
// 		return nil, fmt.Errorf("no sheets found in Excel file")
// 	}

// 	rows, err := f.GetRows(sheets[0])
// 	if err != nil {
// 		return nil, err
// 	}

// 	if len(rows) < 2 {
// 		return nil, fmt.Errorf("file must contain header and at least one data row")
// 	}

// 	// Skip header row
// 	var requests []models.CreateInternshipRequest
// 	for i := 1; i < len(rows); i++ {
// 		row := rows[i]
// 		if len(row) < 6 {
// 			continue
// 		}

// 		credits, _ := strconv.Atoi(row[5])
// 		raw := strings.TrimSpace(row[7])

// 		stipend, err := strconv.ParseFloat(raw, 64)
// 		if err != nil {
// 			return nil, fmt.Errorf("invalid stipend value '%s'", raw)
// 		}
// 		req := models.CreateInternshipRequest{
// 			PRN:            strings.TrimSpace(row[0]),
// 			Organization:   strings.TrimSpace(row[2]),
// 			StartDate:      strings.TrimSpace(row[3]),
// 			EndDate:        strings.TrimSpace(row[4]),
// 			Credits:        credits,
// 			Mode:           strings.TrimSpace(row[6]),
// 			MonthlyStipend: stipend,
// 			Description:    strings.TrimSpace(row[8]),
// 		}

// 		// if len(row) > 6 {
// 		// 	req.Description = strings.TrimSpace(row[6])
// 		// }

// 		requests = append(requests, req)
// 	}

// 	return requests, nil
// }

func (h *InternshipHandler) parseXLSX(file io.Reader) ([]models.CreateInternshipRequest, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	f, err := excelize.OpenReader(strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("file must contain header and at least one data row")
	}

	index := buildInternshipColumnIndex(rows[0])
	// Backward compatible fallback for the old fixed-column upload format.
	if !index.hasHeaders {
		var requests []models.CreateInternshipRequest
		for i := 1; i < len(rows); i++ {
			row := rows[i]
			if len(row) == 0 {
				continue
			}
			credits := parseCreditsValue(safeGet(row, 5))
			stipend, _ := normalizeStipend(safeGet(row, 7))
			rawStartDate := safeGet(row, 3)
			rawEndDate := safeGet(row, 4)
			rawMode := safeGet(row, 6)
			req := models.CreateInternshipRequest{
				PRN:            safeGet(row, 0),
				Organization:   safeGet(row, 2),
				StartDate:      parseDateValue(rawStartDate),
				EndDate:        parseDateValue(rawEndDate),
				Credits:        credits,
				Mode:           normalizeMode(rawMode),
				MonthlyStipend: stipend,
				Description:    safeGet(row, 8),
				RawStartDate:   rawStartDate,
				RawEndDate:     rawEndDate,
				RawMode:        rawMode,
			}
			if isRowEffectivelyEmpty(req) {
				continue
			}
			req.ProcessedRow = len(requests) + 1
			req.SheetRow = i + 1
			requests = append(requests, req)
		}
		return requests, nil
	}

	var requests []models.CreateInternshipRequest
	for i := 1; i < len(rows); i++ {
		req := mapRowToInternshipRequest(rows[i], index)
		if isRowEffectivelyEmpty(req) {
			continue
		}
		req.ProcessedRow = len(requests) + 1
		req.SheetRow = i + 1
		requests = append(requests, req)
	}
	return requests, nil
}

type internshipColumnIndex struct {
	prn          int
	organization int
	startDate    int
	endDate      int
	credits      int
	description  int
	mode         int
	stipendFlag  int
	stipend      int
	hasHeaders   bool
}

func buildInternshipColumnIndex(header []string) internshipColumnIndex {
	idx := internshipColumnIndex{
		prn:          -1,
		organization: -1,
		startDate:    -1,
		endDate:      -1,
		credits:      -1,
		description:  -1,
		mode:         -1,
		stipendFlag:  -1,
		stipend:      -1,
	}
	for i, raw := range header {
		h := normalizeHeader(raw)
		switch h {
		case "prn", "studentprn":
			idx.prn = i
		case "organization", "company", "organisation":
			idx.organization = i
		case "startdate", "internshipstartdate":
			idx.startDate = i
		case "enddate", "internshipenddate":
			idx.endDate = i
		case "credits", "credit":
			idx.credits = i
		case "description", "internshipdescription":
			idx.description = i
		case "mode", "modeofinternship", "internshipmode":
			idx.mode = i
		case "stipendyn", "stipendyorn", "stipendyesno", "stipend":
			idx.stipendFlag = i
		case "amount", "stipendamount", "monthlystipend":
			idx.stipend = i
		}
	}
	idx.hasHeaders = idx.prn >= 0 && (idx.organization >= 0 || idx.mode >= 0 || idx.startDate >= 0)
	return idx
}

func normalizeHeader(input string) string {
	out := strings.ToLower(strings.TrimSpace(input))
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", "/", "", "(", "", ")", "", ".", "")
	return replacer.Replace(out)
}

func mapRowToInternshipRequest(row []string, idx internshipColumnIndex) models.CreateInternshipRequest {
	prn := safeGet(row, idx.prn)
	organization := safeGet(row, idx.organization)
	start := parseDateValue(safeGet(row, idx.startDate))
	end := parseDateValue(safeGet(row, idx.endDate))
	credits := parseCreditsValue(safeGet(row, idx.credits))
	description := safeGet(row, idx.description)
	mode := normalizeMode(safeGet(row, idx.mode))

	stipendFlag := strings.ToLower(strings.TrimSpace(safeGet(row, idx.stipendFlag)))
	stipendAmount, stipendErr := normalizeStipend(safeGet(row, idx.stipend))
	if stipendErr != nil {
		stipendAmount = -1
	}
	if stipendFlag == "n" || stipendFlag == "no" || stipendFlag == "false" {
		stipendAmount = 0
	}
	if stipendFlag == "y" || stipendFlag == "yes" || stipendFlag == "true" {
		if strings.TrimSpace(safeGet(row, idx.stipend)) == "" {
			stipendAmount = -1
		}
	}

	return models.CreateInternshipRequest{
		PRN:            prn,
		Organization:   organization,
		StartDate:      start,
		EndDate:        end,
		Credits:        credits,
		Mode:           mode,
		MonthlyStipend: stipendAmount,
		Description:    description,
		RawStartDate:   safeGet(row, idx.startDate),
		RawEndDate:     safeGet(row, idx.endDate),
		RawMode:        safeGet(row, idx.mode),
	}
}

func parseCreditsValue(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if i, err := strconv.Atoi(raw); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return int(f)
	}
	return 0
}

func parseDateValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Handle Excel serial date values such as 45784 or 45784.0
	if f, err := strconv.ParseFloat(raw, 64); err == nil && f > 1000 {
		t, dateErr := excelize.ExcelDateToTime(f, false)
		if dateErr == nil {
			return t.Format("2006-01-02")
		}
	}
	return raw
}

func isRowEffectivelyEmpty(req models.CreateInternshipRequest) bool {
	hasInternshipPayload := strings.TrimSpace(req.Organization) != "" ||
		strings.TrimSpace(req.StartDate) != "" ||
		strings.TrimSpace(req.EndDate) != "" ||
		strings.TrimSpace(req.Mode) != "" ||
		strings.TrimSpace(req.Description) != "" ||
		req.Credits > 0 ||
		req.MonthlyStipend != 0

	return !hasInternshipPayload
}

// GetPendingInternships returns all pending internships
func (h *InternshipHandler) GetPendingInternships(c *gin.Context) {
	internships, err := h.internshipService.GetPendingInternships()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch pending internships")
		return
	}

	c.JSON(http.StatusOK, internships)
}

// ApproveInternship approves a specific internship
func (h *InternshipHandler) ApproveInternship(c *gin.Context) {
	// Extract userID from context
	userID, exists := c.Get("userID")
	if !exists {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	approvedBy, ok := userID.(int)
	if !ok {
		errorResponse(c, http.StatusInternalServerError, "Invalid user ID")
		return
	}

	// Get internship ID from URL
	idStr := c.Param("id")
	internshipID, err := strconv.Atoi(idStr)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid internship ID")
		return
	}

	var req models.ApprovalRequest
	if c.Request.ContentLength > 0 {
		if bindErr := c.ShouldBindJSON(&req); bindErr != nil && !errors.Is(bindErr, io.EOF) {
			errorResponse(c, http.StatusBadRequest, "Invalid request payload")
			return
		}
	}

	err = h.internshipService.ApproveInternship(internshipID, approvedBy, req.ReviewNote)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "already processed") {
			errorResponse(c, http.StatusNotFound, err.Error())
			return
		}
		reqID, _ := c.Get("requestID")
		log.Printf("approve internship failed: request_id=%v internship_id=%d approved_by=%d error=%v", reqID, internshipID, approvedBy, err)
		errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to approve internship: %v", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Internship approved successfully"})
}

// RejectInternship rejects a specific internship
func (h *InternshipHandler) RejectInternship(c *gin.Context) {
	// Extract userID from context
	userID, exists := c.Get("userID")
	if !exists {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	approvedBy, ok := userID.(int)
	if !ok {
		errorResponse(c, http.StatusInternalServerError, "Invalid user ID")
		return
	}

	// Get internship ID from URL
	idStr := c.Param("id")
	internshipID, err := strconv.Atoi(idStr)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid internship ID")
		return
	}

	var req models.ApprovalRequest
	if c.Request.ContentLength > 0 {
		if bindErr := c.ShouldBindJSON(&req); bindErr != nil && !errors.Is(bindErr, io.EOF) {
			errorResponse(c, http.StatusBadRequest, "Invalid request payload")
			return
		}
	}

	err = h.internshipService.RejectInternship(internshipID, approvedBy, req.ReviewNote)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "already processed") {
			errorResponse(c, http.StatusNotFound, err.Error())
			return
		}
		reqID, _ := c.Get("requestID")
		log.Printf("reject internship failed: request_id=%v internship_id=%d approved_by=%d error=%v", reqID, internshipID, approvedBy, err)
		errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Failed to reject internship: %v", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Internship rejected successfully"})
}

func (h *InternshipHandler) ListInternships(c *gin.Context) {
	page := 1
	pageSize := 10
	var year *int

	if q := c.Query("page"); q != "" {
		if parsed, err := strconv.Atoi(q); err == nil {
			page = parsed
		}
	}
	if q := c.Query("pageSize"); q != "" {
		if parsed, err := strconv.Atoi(q); err == nil {
			pageSize = parsed
		}
	}
	if q := c.Query("year"); q != "" {
		parsed, err := strconv.Atoi(q)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "invalid year parameter")
			return
		}
		year = &parsed
	}

	resp, err := h.internshipService.ListInternships(
		page,
		pageSize,
		c.Query("status"),
		c.Query("workflowStatus"),
		c.Query("organization"),
		c.Query("guide"),
		c.Query("prn"),
		c.Query("dateFrom"),
		c.Query("dateTo"),
		year,
		c.Query("division"),
	)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch internships")
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *InternshipHandler) BulkReviewInternships(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}
	approvedBy, ok := userID.(int)
	if !ok {
		errorResponse(c, http.StatusInternalServerError, "Invalid user ID")
		return
	}

	var req models.BulkReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	resp, err := h.internshipService.BulkReviewInternships(&req, approvedBy)
	if err != nil {
		if strings.Contains(err.Error(), "confirmation") ||
			strings.Contains(err.Error(), "limit") ||
			strings.Contains(err.Error(), "missing") ||
			strings.Contains(err.Error(), "provided") {
			errorResponse(c, http.StatusBadRequest, err.Error())
			return
		}
		errorResponse(c, http.StatusInternalServerError, "Bulk review failed")
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *InternshipHandler) ExportInternshipsCSV(c *gin.Context) {
	status, workflowStatus, organization, guide, prn, dateFrom, dateTo, year, division, err := parseInternshipExportFilters(c)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	items, err := h.internshipService.ListInternshipsForExport(
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
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to export internships")
		return
	}

	buf := &bytes.Buffer{}
	writer := csv.NewWriter(buf)
	headers := []string{"PRN", "Student Name", "Year", "Division", "Guide", "Organization", "Status", "Workflow Status", "Start Date", "End Date", "Mode", "Stipend", "Credits", "Review Note"}
	if err := writer.Write(headers); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to build CSV")
		return
	}

	for _, item := range items {
		reviewNote := ""
		if item.ReviewNote != nil {
			reviewNote = *item.ReviewNote
		}
		row := []string{
			item.StudentPRN,
			item.StudentName,
			strconv.Itoa(item.Year),
			item.Division,
			item.GuideName,
			item.Organization,
			item.Status,
			item.WorkflowStatus,
			item.StartDate.Format("2006-01-02"),
			item.EndDate.Format("2006-01-02"),
			item.Mode,
			fmt.Sprintf("%.2f", item.MonthlyStipend),
			strconv.Itoa(item.Credits),
			reviewNote,
		}
		if err := writer.Write(row); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to build CSV")
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to build CSV")
		return
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=internship_report.csv")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}

func (h *InternshipHandler) ExportInternshipsPDF(c *gin.Context) {
	status, workflowStatus, organization, guide, prn, dateFrom, dateTo, year, division, err := parseInternshipExportFilters(c)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	items, err := h.internshipService.ListInternshipsForExport(
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
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to export internships")
		return
	}

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	drawReportHeader(pdf, "PCCOE Internship Report")

	headers := []string{"PRN", "Student", "Year", "Div", "Guide", "Org", "Status", "Workflow", "Start", "End", "Credits"}
	widths := []float64{22, 30, 12, 10, 28, 40, 18, 28, 20, 20, 14}

	pdf.SetFont("Arial", "B", 8)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 7)
	for _, item := range items {
		row := []string{
			item.StudentPRN,
			item.StudentName,
			strconv.Itoa(item.Year),
			item.Division,
			item.GuideName,
			item.Organization,
			item.Status,
			item.WorkflowStatus,
			item.StartDate.Format("2006-01-02"),
			item.EndDate.Format("2006-01-02"),
			strconv.Itoa(item.Credits),
		}
		for i, col := range row {
			pdf.CellFormat(widths[i], 6, col, "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to build PDF")
		return
	}
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename=internship_report.pdf")
	c.Data(http.StatusOK, "application/pdf", out.Bytes())
}

func (h *InternshipHandler) GetInternshipAudit(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid internship id")
		return
	}
	logs, err := h.internshipService.GetInternshipAudit(id)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch internship audit trail")
		return
	}
	c.JSON(http.StatusOK, logs)
}

func parseInternshipExportFilters(c *gin.Context) (string, string, string, string, string, string, string, *int, string, error) {
	var year *int
	if q := c.Query("year"); q != "" {
		parsed, err := strconv.Atoi(q)
		if err != nil {
			return "", "", "", "", "", "", "", nil, "", fmt.Errorf("invalid year parameter")
		}
		year = &parsed
	}
	return c.Query("status"),
		c.Query("workflowStatus"),
		c.Query("organization"),
		c.Query("guide"),
		c.Query("prn"),
		c.Query("dateFrom"),
		c.Query("dateTo"),
		year,
		c.Query("division"),
		nil
}
