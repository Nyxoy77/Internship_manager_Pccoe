package client

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
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

	err := h.internshipService.CreateInternship(&req, createdBy)
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

	c.JSON(http.StatusCreated, gin.H{"message": "Internship created successfully"})
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

	// Get uploaded file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
		return
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
func (h *InternshipHandler) parseCSV(file io.Reader) ([]models.CreateInternshipRequest, error) {
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("file must contain header and at least one data row")
	}

	// Skip header row
	var requests []models.CreateInternshipRequest
	for i := 1; i < len(records); i++ {
		record := records[i]
		if len(record) < 6 {
			continue
		}

		credits, _ := strconv.Atoi(record[5])
		raw := strings.TrimSpace(record[7])

		stipend, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid stipend value '%s'", raw)
		}

		// Intentionally ignored the student name as it is not required for updation. Prn forms single source of truth
		req := models.CreateInternshipRequest{
			PRN:            strings.TrimSpace(record[0]),
			Organization:   strings.TrimSpace(record[2]),
			StartDate:      strings.TrimSpace(record[3]),
			EndDate:        strings.TrimSpace(record[4]),
			Credits:        credits,
			Mode:           strings.TrimSpace(record[6]),
			MonthlyStipend: stipend,
			Description:    strings.TrimSpace(record[8]),
		}

		// if len(record) > 6 {
		// 	req.Description = strings.TrimSpace(record[6])
		// }

		requests = append(requests, req)
	}

	return requests, nil
}

// parseXLSX parses Excel file and returns internship requests
func (h *InternshipHandler) parseXLSX(file io.Reader) ([]models.CreateInternshipRequest, error) {
	// Read file into memory
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Open Excel file
	f, err := excelize.OpenReader(strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Get first sheet
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found in Excel file")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("file must contain header and at least one data row")
	}

	// Skip header row
	var requests []models.CreateInternshipRequest
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) < 6 {
			continue
		}

		credits, _ := strconv.Atoi(row[5])
		raw := strings.TrimSpace(row[7])

		stipend, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid stipend value '%s'", raw)
		}
		req := models.CreateInternshipRequest{
			PRN:            strings.TrimSpace(row[0]),
			Organization:   strings.TrimSpace(row[2]),
			StartDate:      strings.TrimSpace(row[3]),
			EndDate:        strings.TrimSpace(row[4]),
			Credits:        credits,
			Mode:           strings.TrimSpace(row[6]),
			MonthlyStipend: stipend,
			Description:    strings.TrimSpace(row[8]),
		}

		// if len(row) > 6 {
		// 	req.Description = strings.TrimSpace(row[6])
		// }

		requests = append(requests, req)
	}

	return requests, nil
}

// GetPendingInternships returns all pending internships
func (h *InternshipHandler) GetPendingInternships(c *gin.Context) {
	internships, err := h.internshipService.GetPendingInternships()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch pending internships"})
		return
	}

	c.JSON(http.StatusOK, internships)
}

// ApproveInternship approves a specific internship
func (h *InternshipHandler) ApproveInternship(c *gin.Context) {
	// Extract userID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	approvedBy, ok := userID.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get internship ID from URL
	idStr := c.Param("id")
	internshipID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid internship ID"})
		return
	}

	err = h.internshipService.ApproveInternship(internshipID, approvedBy)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve internship"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Internship approved successfully"})
}

// RejectInternship rejects a specific internship
func (h *InternshipHandler) RejectInternship(c *gin.Context) {
	// Extract userID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	approvedBy, ok := userID.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get internship ID from URL
	idStr := c.Param("id")
	internshipID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid internship ID"})
		return
	}

	err = h.internshipService.RejectInternship(internshipID, approvedBy)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject internship"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Internship rejected successfully"})
}
