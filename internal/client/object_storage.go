package client

import (
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/student-internship-manager/internal/service"
)

type CertificateClient struct {
	Service *service.ObjectStorageService
}

func NewCertificateClient(s *service.ObjectStorageService) *CertificateClient {
	return &CertificateClient{Service: s}
}

func (cc *CertificateClient) UploadCertificate(c *gin.Context) {
	internshipID, err := strconv.Atoi(c.Param("internshipId"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid internship id")
		return
	}

	// assume auth middleware sets this
	userID, exists := c.Get("userID")
	if !exists {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	uploadedBy, ok := userID.(int)
	if !ok {
		errorResponse(c, http.StatusInternalServerError, "Invalid user ID")
		return
	}

	// limit request size
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20)

	file, header, err := c.Request.FormFile("certificate")
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "certificate file required")
		return
	}
	defer file.Close()

	err = cc.Service.UploadCertificate(
		c.Request.Context(),
		internshipID,
		uploadedBy,
		file,
		header,
	)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "certificate uploaded successfully"})
}

func (cc *CertificateClient) RemoveCertificate(c *gin.Context) {
	internshipID, err := strconv.Atoi(c.Param("internshipId"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid internship id")
		return
	}
	userID, exists := c.Get("userID")
	if !exists {
		errorResponse(c, http.StatusUnauthorized, "User not authenticated")
		return
	}
	removedBy, ok := userID.(int)
	if !ok {
		errorResponse(c, http.StatusInternalServerError, "Invalid user ID")
		return
	}
	if err := cc.Service.RemoveCertificate(c.Request.Context(), internshipID, removedBy); err != nil {
		log.Println("error while removing the certificate", err)
		errorResponse(c, http.StatusBadRequest, "error while removing the certificate")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"messaage": "Certificate removed successfully",
	})
}

func (cc *CertificateClient) DownloadViewCertificate(c *gin.Context) {
	internshipID, err := strconv.Atoi(c.Param("internshipId"))
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid internship id")
		return
	}

	object, mimeType, err := cc.Service.GetCertificate(c.Request.Context(), internshipID)
	if err != nil {
		errorResponse(c, http.StatusNotFound, err.Error())
		return
	}
	defer object.Close()

	c.Header("Content-Type", mimeType)
	c.Header("Content-Disposition", "inline")

	_, err = io.Copy(c.Writer, object)
	if err != nil {
		// Client disconnected / stream error
		c.Status(http.StatusInternalServerError)
		return
	}
}
