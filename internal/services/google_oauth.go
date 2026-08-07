package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"halostory_go_backend/internal/app/models"
	"halostory_go_backend/internal/config"
	"halostory_go_backend/internal/database"

	"golang.org/x/oauth2"
	drive "google.golang.org/api/drive/v3"
	googleoauth2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
}

var GOOGLE_OAUTH *oauth2.Config

func InitGoogleAuthorization() {
	CLIENT_ID := config.Env("CLIENT_ID", "")
	CLIENT_SECRET := config.Env("CLIENT_SECRET", "")
	REDIRECT_URL := config.Env("REDIRECT_URL", "")

	if CLIENT_ID == "" || CLIENT_SECRET == "" || REDIRECT_URL == "" {
		log.Println("Gagal terhubung ke google.")
		log.Fatal("Pastikan CLIENT_ID, CLIENT_SECRET, dan REDIRECT_URL ada di env.")
		return
	}

	GOOGLE_OAUTH = &oauth2.Config{
		ClientID:     CLIENT_ID,
		ClientSecret: CLIENT_SECRET,
		RedirectURL:  REDIRECT_URL,
		Scopes: []string{
			"openid",
			"email",
			"profile",
			"https://www.googleapis.com/auth/drive.appdata",
			"https://www.googleapis.com/auth/drive.file",
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
	}
}

func ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	if GOOGLE_OAUTH == nil {
		return nil, fmt.Errorf("GOOGLE_OAUTH belum di setup! periksa kembali env anda.")
	}

	return GOOGLE_OAUTH.Exchange(
		ctx,
		code,
	)
}

func ExchangeCodeCustom(ctx context.Context, code, clientID, clientSecret, redirectURL string) (*oauth2.Token, error) {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"openid",
			"email",
			"profile",
			"https://www.googleapis.com/auth/drive.appdata",
			"https://www.googleapis.com/auth/drive.file",
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL: "https://oauth2.googleapis.com/token",
		},
	}
	return config.Exchange(ctx, code)
}

func GetOAuthClient(ctx context.Context, token *oauth2.Token) *http.Client {
	if strings.Contains(token.RefreshToken, "|") {
		parts := strings.Split(token.RefreshToken, "|")
		realToken := parts[0]
		clientID := parts[1]
		clientSecret := parts[2]

		config := &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
				TokenURL: "https://oauth2.googleapis.com/token",
			},
		}
		tempToken := &oauth2.Token{
			RefreshToken: realToken,
		}
		return config.Client(ctx, tempToken)
	}

	return GOOGLE_OAUTH.Client(ctx, token)
}

func GetTokenOauthFromDB(user_id uint) (oauth2.Token, error) {
	var userOauth models.UserOAuth
	err := database.DB.Where("user_id = ?", user_id).First(&userOauth).Error
	if err != nil {
		return oauth2.Token{}, fmt.Errorf("Error saat mencari refresh token: %w", err)
	}

	return oauth2.Token{
		RefreshToken: userOauth.RefreshToken,
	}, nil
}

func getGoogleIdentityService(ctx context.Context, token *oauth2.Token) (*googleoauth2.Service, error) {
	if GOOGLE_OAUTH == nil {
		return nil, fmt.Errorf("GOOGLE_OAUTH belum di setup! periksa kembali env anda.")
	}
	
	client := GetOAuthClient(ctx, token)
	oauth2Service, err := googleoauth2.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("gagal membuat google identity service: %w", err)
	}

	return oauth2Service, nil
}

func GetGoogleIdentity(ctx context.Context, token *oauth2.Token) (GoogleUserInfo, error) {
	oauth2Service, err := getGoogleIdentityService(ctx, token)
	if err != nil {
		return GoogleUserInfo{}, err
	}
	userInfo, err := oauth2Service.Userinfo.Get().Do()
	if err != nil {
		return GoogleUserInfo{}, fmt.Errorf("gagal mengambil profil dari google userinfo: %w", err)
	}

	return GoogleUserInfo{
		ID:            userInfo.Id,
		Email:         userInfo.Email,
		VerifiedEmail: *userInfo.VerifiedEmail,
		Name:          userInfo.Name,
	}, nil
}

func getDriveService(ctx context.Context, token *oauth2.Token) (*drive.Service, error) {
	if GOOGLE_OAUTH == nil {
		return nil, fmt.Errorf("GOOGLE_OAUTH belum di-setup! periksa kembali env anda")
	}

	client := GetOAuthClient(ctx, token)
	driveService, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("gagal membuat drive service: %w", err)
	}

	return driveService, nil
}

func DriveUploadBackup(ctx context.Context, token *oauth2.Token, encryptedData string) (bool, error) {
	srv, err := getDriveService(ctx, token)
	if err != nil {
		return false, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	timestamp := strings.NewReplacer(":", "-", ".", "-").Replace(now)
	fileName := fmt.Sprintf("HaloStory_Backup_%s.hsai.enc", timestamp)

	fileMetadata := &drive.File{
		Name:        fileName,
		Parents:     []string{"appDataFolder"},
		MimeType:    "application/octet-stream",
		Description: fmt.Sprintf("HaloStory backup created at %s", timestamp),
	}

	mediaReader := bytes.NewReader([]byte(encryptedData))

	_, err = srv.Files.Create(fileMetadata).
		Media(mediaReader).
		Context(ctx).
		Do()

	if err != nil {
		return false, fmt.Errorf("gagal upload backup: %w", err)
	}

	return true, nil
}

func DriveDownloadBackup(ctx context.Context, token *oauth2.Token, fileId string) (string, error) {
	srv, err := getDriveService(ctx, token)
	if err != nil {
		return "", err
	}

	res, err := srv.Files.Get(fileId).
		Download()
	if err != nil {
		return "", fmt.Errorf("gagal download backup: %w", err)
	}
	defer res.Body.Close()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, res.Body); err != nil {
		return "", fmt.Errorf("gagal membaca stream file: %w", err)
	}

	return buf.String(), nil
}

func DriveDeleteBackup(ctx context.Context, token *oauth2.Token, fileId string) (bool, error) {
	srv, err := getDriveService(ctx, token)
	if err != nil {
		return false, err
	}

	err = srv.Files.Delete(fileId).
		Context(ctx).
		Do()
	if err != nil {
		return false, fmt.Errorf("gagal menghapus backup: %w", err)
	}

	return true, nil
}

func DriveListBackups(ctx context.Context, token *oauth2.Token) ([]*drive.File, error) {
	srv, err := getDriveService(ctx, token)
	if err != nil {
		return nil, err
	}

	res, err := srv.Files.List().
		Q("name contains 'HaloStory_Backup_' and trashed = false").
		OrderBy("createdTime desc").
		PageSize(20).
		Spaces("appDataFolder").
		Context(ctx).
		Do()

	if err != nil {
		return nil, fmt.Errorf("gagal mengambil daftar backup: %w", err)
	}

	return res.Files, nil
}
