package client

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-pdf/fpdf"

	"github.com/yourusername/student-internship-manager/internal/service"
)

type StudentHandler struct {
	studentService *service.StudentService
}

func NewStudentHandler(studentService *service.StudentService) *StudentHandler {
	return &StudentHandler{
		studentService: studentService,
	}
}

// GetStudentSummary returns student summary with total credits
func (h *StudentHandler) GetStudentSummary(c *gin.Context) {
	prn := c.Param("prn")

	if prn == "" {
		errorResponse(c, http.StatusBadRequest, "PRN is required")
		return
	}

	summary, err := h.studentService.GetStudentSummary(prn)
	if err != nil {
		if err.Error() == "student not found" {
			errorResponse(c, http.StatusNotFound, "Student not found")
			return
		}
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch student summary")
		return
	}

	c.JSON(http.StatusOK, summary)
}

// ListStudents returns filtered list of students
func (h *StudentHandler) ListStudents(c *gin.Context) {
	page, pageSize, passingYear, err := parseStudentCommonQuery(c)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	division := strings.TrimSpace(c.Query("division"))
	prn := strings.TrimSpace(c.Query("prn"))
	name := strings.TrimSpace(c.Query("name"))
	guide := strings.TrimSpace(c.Query("guide"))

	students, err := h.studentService.ListStudents(page, pageSize, passingYear, division, prn, name, guide)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to fetch students")
		log.Printf("failed to fetch students: %v", err)
		return
	}

	c.JSON(http.StatusOK, students)
}

func (h *StudentHandler) ListStudentCreditReport(c *gin.Context) {
	page, pageSize, passingYear, err := parseStudentCommonQuery(c)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.studentService.ListStudentCreditReport(
		page,
		pageSize,
		passingYear,
		strings.TrimSpace(c.Query("division")),
		strings.TrimSpace(c.Query("prn")),
		strings.TrimSpace(c.Query("name")),
		strings.TrimSpace(c.Query("guide")),
		strings.TrimSpace(c.Query("creditFilter")),
	)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to fetch student credit report")
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *StudentHandler) ExportStudentCreditReportCSV(c *gin.Context) {
	_, _, passingYear, err := parseStudentCommonQuery(c)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	rows, err := h.studentService.ListStudentCreditReportForExport(
		passingYear,
		strings.TrimSpace(c.Query("division")),
		strings.TrimSpace(c.Query("prn")),
		strings.TrimSpace(c.Query("name")),
		strings.TrimSpace(c.Query("guide")),
		strings.TrimSpace(c.Query("creditFilter")),
	)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to export student credit report")
		return
	}

	buf := &bytes.Buffer{}
	writer := csv.NewWriter(buf)
	if err := writer.Write([]string{"PRN", "Name", "Division", "Passing Year", "Total Credits", "Guide"}); err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to build CSV")
		return
	}
	for _, row := range rows {
		if err := writer.Write([]string{
			row.PRN,
			row.Name,
			row.Division,
			strconv.Itoa(row.Year),
			strconv.Itoa(row.TotalCredits),
			row.GuideName,
		}); err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to build CSV")
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to build CSV")
		return
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=student_credit_report.csv")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}

func (h *StudentHandler) ExportStudentCreditReportPDF(c *gin.Context) {
	_, _, passingYear, err := parseStudentCommonQuery(c)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	rows, err := h.studentService.ListStudentCreditReportForExport(
		passingYear,
		strings.TrimSpace(c.Query("division")),
		strings.TrimSpace(c.Query("prn")),
		strings.TrimSpace(c.Query("name")),
		strings.TrimSpace(c.Query("guide")),
		strings.TrimSpace(c.Query("creditFilter")),
	)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to export student credit report")
		return
	}

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	drawReportHeader(pdf, "PCCOE Student Credit Report")

	headers := []string{"PRN", "Name", "Division", "Year", "Credits", "Guide"}
	widths := []float64{30, 60, 20, 20, 24, 80}
	pdf.SetFont("Arial", "B", 9)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 7, h, "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 8)
	for _, row := range rows {
		values := []string{
			row.PRN,
			row.Name,
			row.Division,
			strconv.Itoa(row.Year),
			strconv.Itoa(row.TotalCredits),
			row.GuideName,
		}
		for i, v := range values {
			pdf.CellFormat(widths[i], 6, v, "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
	}

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to build PDF")
		return
	}
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename=student_credit_report.pdf")
	c.Data(http.StatusOK, "application/pdf", out.Bytes())
}

func parseStudentCommonQuery(c *gin.Context) (int, int, *int, error) {
	page := 1
	pageSize := 10
	var passingYear *int

	if q := strings.TrimSpace(c.Query("page")); q != "" {
		if parsed, err := strconv.Atoi(q); err == nil {
			page = parsed
		}
	}
	if q := strings.TrimSpace(c.Query("pageSize")); q != "" {
		if parsed, err := strconv.Atoi(q); err == nil {
			pageSize = parsed
		}
	}

	yearStr := strings.TrimSpace(c.Query("year"))
	if yearStr != "" {
		year, err := strconv.Atoi(yearStr)
		if err != nil {
			return 0, 0, nil, fmt.Errorf("invalid year parameter")
		}
		passingYear = &year
	}

	return page, pageSize, passingYear, nil
}
