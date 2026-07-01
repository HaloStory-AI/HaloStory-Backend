package api

import (
	"context"
	"halostory_go_backend/internal/services"

	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
)

func ImageProxy(c *fiber.Ctx) error {
	path := c.Query("path")
	if path == "" {
		return c.Status(fiber.StatusNotFound).SendString("Not Found")
	}

	client := services.MinioClient
	bucket := services.S3Bucket

	if client == nil {
		return c.Status(fiber.StatusInternalServerError).SendString("S3 Client not configured")
	}

	ctx := context.Background()

	// Dapatkan meta data untuk Content-Type
	stat, err := client.StatObject(ctx, bucket, path, minio.StatObjectOptions{})
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Not Found")
	}

	// Buka stream object
	obj, err := client.GetObject(ctx, bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Not Found")
	}

	// Atur Header
	contentType := stat.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	c.Set("Content-Type", contentType)
	c.Set("Cache-Control", "public, max-age=86400")

	// Pass-through stream dari MinIO (S3) langsung ke client HTTP Fiber
	// obj mengimplementasikan io.Reader
	return c.SendStream(obj)
}
