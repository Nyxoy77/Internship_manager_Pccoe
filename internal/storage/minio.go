package storage

import (
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func NewMinioClient() (*minio.Client, error) {
	return minio.New("minio:9000", &minio.Options{
		Creds: credentials.NewStaticV4(
			"minioadmin",
			"minioadmin",
			"",
		),
		Secure: false,
	})
}
