package services

import (
	"context"
	"fmt"
	"log"

	"halostory_go_backend/internal/config"

	"golang.org/x/oauth2"
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

func GetGoogleIdentity(ctx context.Context, token *oauth2.Token) (GoogleUserInfo, error) {
	if GOOGLE_OAUTH == nil {
		return GoogleUserInfo{}, fmt.Errorf("GOOGLE_OAUTH belum di setup! periksa kembali env anda.")
	}

	client := GOOGLE_OAUTH.Client(ctx, token)

	oauth2Service, err := googleoauth2.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return GoogleUserInfo{}, fmt.Errorf("gagal mengambil profil dari google service: %w", err)
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
