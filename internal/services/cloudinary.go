package services

import (
	"context"
	"log"
	"regexp"
	"strings"

	"halostory_go_backend/internal/config"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

var CloudinaryClient *cloudinary.Cloudinary

func InitCloudinary() {
	cloudName := config.Env("CLOUDINARY_CLOUD_NAME", "")
	apiKey := config.Env("CLOUDINARY_API_KEY", "")
	apiSecret := config.Env("CLOUDINARY_API_SECRET", "")

	if cloudName == "" || apiKey == "" || apiSecret == "" {
		log.Println("Cloudinary credentials not fully configured, image CDN will fail.")
		return
	}

	cld, err := cloudinary.NewFromParams(cloudName, apiKey, apiSecret)
	if err != nil {
		log.Printf("Failed to initialize Cloudinary client: %v", err)
		return
	}

	CloudinaryClient = cld
	log.Println("Successfully initialized Cloudinary client.")
}

func ImageURL(path string) string {
	if path == "" {
		return ""
	}

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}

	if CloudinaryClient == nil {
		return path
	}

	publicID := stripExtension(path)
	image, err := CloudinaryClient.Image(publicID)
	if err != nil {
		log.Printf("Cloudinary: failed to create image URL for %s: %v", path, err)
		return path
	}

	url, err := image.String()
	if err != nil {
		log.Printf("Cloudinary: failed to generate URL for %s: %v", path, err)
		return path
	}

	return url
}

func UploadImage(contents []byte, publicID string) (*string, error) {
	if CloudinaryClient == nil {
		return nil, nil
	}

	ctx := context.Background()
	publicID = stripExtension(publicID)

	overwrite := true
	resp, err := CloudinaryClient.Upload.Upload(ctx, contents, uploader.UploadParams{
		PublicID:  publicID,
		Overwrite: &overwrite,
	})
	if err != nil {
		return nil, err
	}

	return &resp.SecureURL, nil
}

var reExtension = regexp.MustCompile(`(?i)\.(png|jpg|jpeg|gif|webp|avif|svg)$`)

func stripExtension(publicID string) string {
	return reExtension.ReplaceAllString(publicID, "")
}
