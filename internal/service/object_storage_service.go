package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/minio/minio-go/v7"
)

type ObjectStorageService struct {
	DB          *sqlx.DB
	MinioClient *minio.Client
	BucketName  string
}

func NewObjectStorageService(
	db *sqlx.DB,
	minioClient *minio.Client,
) *ObjectStorageService {
	return &ObjectStorageService{
		DB:          db,
		MinioClient: minioClient,
		BucketName:  "internship-certificates",
	}
}

func (s *ObjectStorageService) UploadCertificateHandler(c *gin.Context) {
	ctx := context.Background()

	internshipID := 4
	prn := "123B1B168"
	userID := 1

	// ✅ Limit upload size (10MB)
	c.Request.Body = http.MaxBytesReader(
		c.Writer,
		c.Request.Body,
		10<<20,
	)

	file, header, err := c.Request.FormFile("certificate")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "certificate file required"})
		log.Println(err)
		return
	}
	defer file.Close()

	// ✅ Validate file type
	contentType := header.Header.Get("Content-Type")
	if contentType != "application/pdf" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only PDF allowed"})
		return
	}

	objectKey := fmt.Sprintf(
		"certificates/%d/%s/%d/%s.pdf",
		time.Now().Year(),
		prn,
		internshipID,
		uuid.New().String(),
	)

	// ✅ Upload to MinIO
	_, err = s.MinioClient.PutObject(
		ctx,
		s.BucketName,
		objectKey,
		file,
		header.Size,
		minio.PutObjectOptions{
			ContentType: contentType,
		},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload to storage failed"})
		log.Println(err)

		return
	}

	// ✅ DB transaction
	tx, err := s.DB.Beginx()
	if err != nil {
		_ = s.MinioClient.RemoveObject(ctx, s.BucketName, objectKey, minio.RemoveObjectOptions{})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	_, err = tx.Exec(`
		INSERT INTO certificates
		(internship_id, object_key, original_filename, mime_type, file_size, uploaded_by)
		VALUES ($1,$2,$3,$4,$5,$6)
	`,
		internshipID,
		objectKey,
		header.Filename,
		contentType,
		header.Size,
		userID,
	)

	if err != nil {
		tx.Rollback()
		_ = s.MinioClient.RemoveObject(ctx, s.BucketName, objectKey, minio.RemoveObjectOptions{})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save metadata"})
		log.Println(err)
		return
	}

	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"message":   "certificate uploaded successfully",
		"objectKey": objectKey,
	})
}
