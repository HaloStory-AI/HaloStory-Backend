package api

import (
	"context"
	"halostory_go_backend/internal/services"

	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
)

func ImageProxy(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.String(http.StatusNotFound, "Not Found")
		return
	}

	client := services.MinioClient
	bucket := services.S3Bucket

	if client == nil {
		c.String(http.StatusInternalServerError, "S3 Client not configured")
		return
	}

	ctx := context.Background()

	// Dapatkan meta data untuk Content-Type
	stat, err := client.StatObject(ctx, bucket, path, minio.StatObjectOptions{})
	if err != nil {
		c.String(http.StatusNotFound, "Not Found")
		return
	}

	// Buka stream object
	obj, err := client.GetObject(ctx, bucket, path, minio.GetObjectOptions{})
	if err != nil {
		c.String(http.StatusNotFound, "Not Found")
		return
	}

	// Atur Header
	contentType := stat.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// DataFromReader mengirim Content-Length, Content-Type, dan extra headers.
	c.DataFromReader(http.StatusOK, stat.Size, contentType, obj, map[string]string{
		"Cache-Control": "public, max-age=86400",
	})
}
