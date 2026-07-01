package utils

import (
	"bytes"
	"html/template"
	"log"
	"strconv"

	"halostory_go_backend/internal/config"

	"gopkg.in/gomail.v2"
)

type OtpEmailData struct {
	Title   string
	Body    string
	Otp     string
	AppName string
}

func SendOtpEmail(to, otp string) error {
	host := config.Env("MAIL_HOST", "127.0.0.1")
	portStr := config.Env("MAIL_PORT", "2525")
	user := config.Env("MAIL_USERNAME", "")
	pass := config.Env("MAIL_PASSWORD", "")
	from := config.Env("MAIL_FROM_ADDRESS", "noreply@halostory.ai")
	fromName := config.Env("MAIL_FROM_NAME", "HaloStory AI")
	appName := config.Env("APP_NAME", "HaloStory")

	port, _ := strconv.Atoi(portStr)

	// Siapkan data untuk template
	data := OtpEmailData{
		Title:   "Email Verification OTP",
		Body:    "Terima kasih telah bergabung dengan HaloStory AI! Untuk menyelesaikan pendaftaran dan mengamankan akun Anda, silakan verifikasi email Anda menggunakan kode berikut:",
		Otp:     otp,
		AppName: appName,
	}

	// Parse template HTML dari file
	t, err := template.ParseFiles("internal/templates/otp.html")
	if err != nil {
		log.Println("Gagal membaca template otp.html:", err)
		return err
	}

	var body bytes.Buffer
	if err := t.Execute(&body, data); err != nil {
		return err
	}

	m := gomail.NewMessage()
	m.SetHeader("From", m.FormatAddress(from, fromName))
	m.SetHeader("To", to)
	m.SetHeader("Subject", appName+" - Email Verification OTP")
	m.SetBody("text/html", body.String())

	d := gomail.NewDialer(host, port, user, pass)

	if err := d.DialAndSend(m); err != nil {
		log.Println("Gagal mengirim email ke", to, ":", err)
		return err
	}

	return nil
}
