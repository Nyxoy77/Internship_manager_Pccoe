package client

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "PRN is required"})
		return
	}

	summary, err := h.studentService.GetStudentSummary(prn)
	if err != nil {
		if err.Error() == "student not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Student not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch student summary"})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// ListStudents returns filtered list of students
func (h *StudentHandler) ListStudents(c *gin.Context) {
	var passingYear *int

	yearStr := c.Query("year")
	if yearStr != "" {
		year, err := strconv.Atoi(yearStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid year parameter",
			})
			return
		}
		passingYear = &year
	}

	division := c.Query("division")

	students, err := h.studentService.ListStudents(passingYear, division)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to fetch students",
		})
		log.Printf("failed to fetch students: %v", err)
		return
	}

	c.JSON(http.StatusOK, students)
}
