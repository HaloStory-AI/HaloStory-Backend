package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/url"

	"halostory_go_backend/internal/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var MinioClient *minio.Client
var S3Bucket string

func InitS3() {
	s3Key := config.Env("S3_KEY", "")
	s3Secret := config.Env("S3_SECRET", "")
	s3Endpoint := config.Env("S3_ENDPOINT", "")
	s3Bucket := config.Env("S3_BUCKET", "")

	if s3Key == "" || s3Endpoint == "" {
		log.Println("S3 credentials not fully configured, S3 proxy might fail.")
		return
	}

	// Parse endpoint to extract host and secure boolean
	parsedUrl, err := url.Parse(s3Endpoint)
	if err != nil {
		log.Fatalf("Failed to parse S3_ENDPOINT: %v", err)
	}

	secure := parsedUrl.Scheme == "https"
	endpoint := parsedUrl.Host

	// Initialize minio client object.
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(s3Key, s3Secret, ""),
		Secure: secure,
	})

	if err != nil {
		log.Fatalf("Failed to initialize MinIO client: %v", err)
	}

	MinioClient = client
	S3Bucket = s3Bucket
	log.Println("Successfully initialized S3 client.")
}

// UploadObject menulis objek ke bucket S3/MinIO yang sama. Dipakai oleh admin
// untuk menyimpan avatar bot / logo kategori dengan key seperti "Bot/{botId}/{file}".
func UploadObject(objectKey string, data []byte, contentType string) error {
	if MinioClient == nil || S3Bucket == "" {
		return fmt.Errorf("S3 client is not initialized")
	}

	_, err := MinioClient.PutObject(
		context.Background(),
		S3Bucket,
		objectKey,
		bytes.NewReader(data),
		int64(len(data)),
		minio.PutObjectOptions{
			ContentType: contentType,
		},
	)
	return err
}

// ── Backup R2 helpers (overwrite latest.enc) ──

func backupKey(userID uint) string {
	return fmt.Sprintf("backups/%d/latest.enc", userID)
}

// PutBackupObject menulis backup terenkripsi user ke R2 (overwrite).
func PutBackupObject(userID uint, data []byte) (minio.UploadInfo, error) {
	if MinioClient == nil || S3Bucket == "" {
		return minio.UploadInfo{}, fmt.Errorf("S3 client is not initialized")
	}
	return MinioClient.PutObject(
		context.Background(),
		S3Bucket,
		backupKey(userID),
		bytes.NewReader(data),
		int64(len(data)),
		minio.PutObjectOptions{ContentType: "application/octet-stream"},
	)
}

// GetBackupObject membaca backup dari R2.
func GetBackupObject(userID uint) ([]byte, error) {
	if MinioClient == nil || S3Bucket == "" {
		return nil, fmt.Errorf("S3 client is not initialized")
	}
	obj, err := MinioClient.GetObject(context.Background(), S3Bucket, backupKey(userID), minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	stat, err := obj.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size == 0 {
		return nil, fmt.Errorf("backup empty")
	}
	buf, err := io.ReadAll(obj)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// StatBackupObject mengembalikan metadata objek backup.
func StatBackupObject(userID uint) (minio.ObjectInfo, error) {
	if MinioClient == nil || S3Bucket == "" {
		return minio.ObjectInfo{}, fmt.Errorf("S3 client is not initialized")
	}
	return MinioClient.StatObject(context.Background(), S3Bucket, backupKey(userID), minio.StatObjectOptions{})
}

// BackupObjectKey mengembalikan key R2 untuk user (dipakai controller untuk metadata).
func BackupObjectKey(userID uint) string { return backupKey(userID) }
