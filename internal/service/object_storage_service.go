package service

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"mime/multipart"
	"regexp"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/minio/minio-go/v7"
)

type ObjectStorageService struct {
	DB         *sqlx.DB
	Minio      *minio.Client
	BucketName string
}

func NewObjectStorageService(db *sqlx.DB, minio *minio.Client) *ObjectStorageService {
	return &ObjectStorageService{
		DB:         db,
		Minio:      minio,
		BucketName: "internship-certificates",
	}
}

func (s *ObjectStorageService) UploadCertificate(ctx context.Context, internshipID int, userID int, file multipart.File, header *multipart.FileHeader) error {

	// 1️⃣ Check if certificate already exists
	var result struct {
		PRN               string `db:"student_prn"`
		Year              int    `db:"passing_year"`
		Organization      string `db:"organization"`
		CertificateExists bool   `db:"certificate_exists"`
	}

	err := s.DB.Get(&result, `
	SELECT
		i.student_prn,
		s.passing_year,
		i.organization,
		EXISTS (
			SELECT 1
			FROM certificates c
			WHERE c.internship_id = i.id
		) AS certificate_exists
	FROM internships i
	JOIN students s ON s.prn = i.student_prn
	WHERE i.id = $1
`, internshipID)

	if err != nil {
		return fmt.Errorf("internship not found: %w", err)
	}

	if result.CertificateExists {
		return fmt.Errorf("certificate already uploaded for this internship")
	}

	// 2️⃣ Validate file
	if header.Size > 10<<20 {
		return fmt.Errorf("file too large")
	}
	if header.Header.Get("Content-Type") != "application/pdf" {
		return fmt.Errorf("only pdf allowed")
	}
	safeOrg := strings.ToLower(result.Organization)
	safeOrg = regexp.MustCompile(`[^a-z0-9_-]`).ReplaceAllString(safeOrg, "_")

	// 3️⃣ Build object key
	objectKey := fmt.Sprintf("certificates/%d/%s/%s/%s.pdf", result.Year, result.PRN, safeOrg, safeOrg)
	log.Println(objectKey)
	// 4️⃣ Upload to MinIO
	_, err = s.Minio.PutObject(
		ctx,
		s.BucketName,
		objectKey,
		file,
		header.Size,
		minio.PutObjectOptions{ContentType: "application/pdf"},
	)
	if err != nil {
		return err
	}

	// 5️⃣ Insert DB record
	_, err = s.DB.Exec(`
		INSERT INTO certificates
		(internship_id, object_key, original_filename, mime_type, file_size, uploaded_by)
		VALUES ($1,$2,$3,$4,$5,$6)
	`,
		internshipID,
		objectKey,
		header.Filename,
		"application/pdf",
		header.Size,
		userID,
	)
	if err != nil {
		_ = s.Minio.RemoveObject(ctx, s.BucketName, objectKey, minio.RemoveObjectOptions{})
		return err
	}
	_, _ = s.DB.Exec(`
		UPDATE internships
		SET workflow_status = 'certificate_uploaded'
		WHERE id = $1
		  AND status = 'pending'
	`, internshipID)
	_ = logInternshipAudit(s.DB, internshipID, "certificate_uploaded", header.Filename, &userID)

	return nil
}

func (s *ObjectStorageService) RemoveCertificate(
	ctx context.Context,
	internshipID int,
	removedBy int,
) error {

	var cert struct {
		ObjectKey string `db:"object_key"`
	}

	err := s.DB.Get(&cert, `
		SELECT object_key FROM certificates WHERE internship_id = $1
	`, internshipID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("certificate not found")
	}
	if err != nil {
		return err
	}

	tx, err := s.DB.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1️⃣ Delete DB row
	_, err = tx.Exec(`
		DELETE FROM certificates WHERE internship_id = $1
	`, internshipID)
	if err != nil {
		return err
	}
	_, _ = tx.Exec(`
		UPDATE internships
		SET workflow_status = 'certificate_pending'
		WHERE id = $1
		  AND status = 'pending'
	`, internshipID)
	_ = logInternshipAudit(tx, internshipID, "certificate_removed", "", &removedBy)

	// 2️⃣ Delete object from storage
	err = s.Minio.RemoveObject(
		ctx,
		s.BucketName,
		cert.ObjectKey,
		minio.RemoveObjectOptions{},
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *ObjectStorageService) GetCertificate(
	ctx context.Context,
	internshipID int,
) (*minio.Object, string, error) {

	var cert struct {
		ObjectKey string `db:"object_key"`
		MimeType  string `db:"mime_type"`
	}

	err := s.DB.Get(&cert, `
		SELECT object_key, mime_type
		FROM certificates
		WHERE internship_id = $1
	`, internshipID)
	if err == sql.ErrNoRows {
		return nil, "", fmt.Errorf("certificate not found")
	}
	if err != nil {
		return nil, "", err
	}

	obj, err := s.Minio.GetObject(
		ctx,
		s.BucketName,
		cert.ObjectKey,
		minio.GetObjectOptions{},
	)
	if err != nil {
		return nil, "", err
	}

	return obj, cert.MimeType, nil
}
