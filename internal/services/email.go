package services

import (
	"bytes"
	"embed"
	"html/template"
	"log"

	"rafiq8k-moga/halostory-go-backend/internal/config"

	gomail "github.com/wneessen/go-mail"
)

//go:embed templates/email_otp.html
var emailTemplates embed.FS

type EmailService struct {
	cfg     *config.Config
	client  *gomail.Client
	from    string
	fromName string
}

type OTPData struct {
	Title   string
	AppName string
	Body    string
	OTP     string
}

func NewEmailService(cfg *config.Config) *EmailService {
	if cfg.MailHost == "" {
		log.Println("email service: SMTP not configured, running in dry-run mode")
		return &EmailService{cfg: cfg}
	}

	client, err := gomail.NewClient(cfg.MailHost,
		gomail.WithPort(cfg.MailPort),
		gomail.WithUsername(cfg.MailUsername),
		gomail.WithPassword(cfg.MailPassword),
	)
	if err != nil {
		log.Printf("email service: failed to create client: %v", err)
		return &EmailService{cfg: cfg}
	}

	return &EmailService{
		cfg:      cfg,
		client:   client,
		from:     cfg.MailFrom,
		fromName: cfg.MailFromName,
	}
}

func (s *EmailService) SendOTP(to, otp, subject string) error {
	if s.client == nil {
		log.Printf("[dry-run] OTP to %s: %s", to, otp)
		return nil
	}

	tmpl, err := template.ParseFS(emailTemplates, "templates/email_otp.html")
	if err != nil {
		return err
	}

	data := OTPData{
		Title:   subject,
		AppName: s.cfg.AppName,
		Body:    "We received a request to verify your email. Please use the following code to proceed:",
		OTP:     otp,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	m := gomail.NewMsg()
	if err := m.From(s.from); err != nil {
		return err
	}
	if err := m.To(to); err != nil {
		return err
	}
	m.Subject(subject)
	m.SetBodyString(gomail.TypeTextHTML, buf.String())

	return s.client.DialAndSend(m)
}
