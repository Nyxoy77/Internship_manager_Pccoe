package client

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/student-internship-manager/internal/models"
	"github.com/yourusername/student-internship-manager/internal/service"
	"github.com/yourusername/student-internship-manager/internal/utils"
)

type StudentAdminHandler struct {
	service *service.AdminService
}

func NewStudentAdminHandler(s *service.AdminService) *StudentAdminHandler {
	return &StudentAdminHandler{service: s}
}

/* -----------------------------
   1️⃣ Individual student
--------------------------------*/

func (h *StudentAdminHandler) CreateStudent(c *gin.Context) {

	var req models.CreateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.CreateStudent(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "student created"})
}



func (h *StudentAdminHandler) BatchUploadStudents(c *gin.Context) {

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	students, err := utils.ParseStudentFile(file, header.Filename)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp := h.service.BatchCreateStudents(students)
	c.JSON(http.StatusOK, resp)
}
