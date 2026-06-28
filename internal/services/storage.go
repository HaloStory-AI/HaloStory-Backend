package services

import (
	"context"
	"fmt"
	"io"
	"log"

	"rafiq8k-moga/halostory-go-backend/internal/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type StorageService struct {
	client *minio.Client
	bucket string
	cdnURL string
}

func NewStorageService(cfg *config.Config) *StorageService {
	if cfg.S3Endpoint == "" || cfg.S3Key == "" {
		log.Println("storage service: S3 not configured, running in dry-run mode")
		return &StorageService{bucket: cfg.S3Bucket}
	}

	client, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3Key, cfg.S3Secret, ""),
		Region: cfg.S3Region,
		Secure: true,
	})
	if err != nil {
		log.Printf("storage service: failed to create client: %v", err)
		return &StorageService{bucket: cfg.S3Bucket}
	}

	return &StorageService{
		client: client,
		bucket: cfg.S3Bucket,
		cdnURL: cfg.S3Endpoint,
	}
}

func (s *StorageService) Upload(botID, filename string, data io.Reader, size int64) (string, error) {
	if s.client == nil {
		log.Printf("[dry-run] upload %s/%s", botID, filename)
		return fmt.Sprintf("%s/Bot/%s/%s", s.cdnURL, botID, filename), nil
	}

	objectName := fmt.Sprintf("Bot/%s/%s", botID, filename)
	info, err := s.client.PutObject(context.Background(), s.bucket, objectName, data, size,
		minio.PutObjectOptions{ContentType: "application/octet-stream"},
	)
	if err != nil {
		return "", fmt.Errorf("storage upload: %w", err)
	}

	log.Printf("uploaded %s (%d bytes)", objectName, info.Size)
	return fmt.Sprintf("%s/%s/%s", s.cdnURL, s.bucket, objectName), nil
}

func (s *StorageService) GetURL(botID, filename string) (string, error) {
	return fmt.Sprintf("%s/%s/Bot/%s/%s", s.cdnURL, s.bucket, botID, filename), nil
}
