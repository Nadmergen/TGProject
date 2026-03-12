package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"os"
)

type SMTPEmailService struct {
	host     string
	port     string
	from     string
	password string
	fromName string
}

func NewSMTPEmailService() *SMTPEmailService {
	return &SMTPEmailService{
		host:     os.Getenv("SMTP_HOST"),
		port:     os.Getenv("SMTP_PORT"),
		from:     os.Getenv("SMTP_FROM"),
		password: os.Getenv("SMTP_PASSWORD"),
		fromName: os.Getenv("SMTP_FROM_NAME"),
	}
}

func (s *SMTPEmailService) SendVerificationEmail(toEmail, code, subject string) error {
	// Если нет конфигурации - просто логируем
	if s.from == "" || s.password == "" {
		log.Println("⚠️ SMTP credentials not configured")
		log.Printf("📧 [MOCK] Email to %s | Subject: %s | Code: %s\n", toEmail, subject, code)
		return nil
	}

	htmlBody := s.buildHTMLEmail(code)

	// Формируем письмо
	headers := map[string]string{
		"From":         fmt.Sprintf("%s <%s>", s.fromName, s.from),
		"To":           toEmail,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/html; charset=UTF-8",
	}

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + htmlBody

	// Отправляем
	if err := s.sendSMTP(toEmail, message); err != nil {
		log.Printf("❌ SMTP Error sending to %s: %v\n", toEmail, err)
		return err
	}

	log.Printf("✅ Email sent via SMTP to %s | Code: %s\n", toEmail, code)
	return nil
}

// === ПРАВИЛЬНАЯ ОТПРАВКА - ПОРТ 465 + TLS DIAL ===
func (s *SMTPEmailService) sendSMTP(toEmail, message string) error {
	// Адрес SMTP сервера
	addr := fmt.Sprintf("%s:%s", s.host, s.port)

	// TLS конфигурация
	tlsConfig := &tls.Config{
		ServerName:         s.host,
		InsecureSkipVerify: false, // Проверяем сертификат
	}

	// ✅ tls.Dial для порта 465 (SSL сразу)
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		log.Printf("❌ TLS connection error: %v\n", err)
		return err
	}
	defer conn.Close()

	// Создаем SMTP клиент
	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		log.Printf("❌ SMTP client error: %v\n", err)
		return err
	}
	defer client.Close()

	// Аутентификация
	auth := smtp.PlainAuth("", s.from, s.password, s.host)
	if err = client.Auth(auth); err != nil {
		log.Printf("❌ SMTP auth error: %v\n", err)
		return err
	}

	// Отправляем письмо
	if err = client.Mail(s.from); err != nil {
		log.Printf("❌ Mail error: %v\n", err)
		return err
	}

	if err = client.Rcpt(toEmail); err != nil {
		log.Printf("❌ Rcpt error: %v\n", err)
		return err
	}

	// Отправляем данные письма
	w, err := client.Data()
	if err != nil {
		log.Printf("❌ Data error: %v\n", err)
		return err
	}

	_, err = w.Write([]byte(message))
	if err != nil {
		log.Printf("❌ Write error: %v\n", err)
		return err
	}

	err = w.Close()
	if err != nil {
		log.Printf("❌ Close error: %v\n", err)
		return err
	}

	client.Quit()
	return nil
}

// === ТЕСТИРОВАНИЕ СОЕДИНЕНИЯ ===
func (s *SMTPEmailService) TestConnection() error {
	if s.from == "" {
		return fmt.Errorf("SMTP_FROM not configured")
	}

	addr := fmt.Sprintf("%s:%s", s.host, s.port)

	// TLS конфигурация
	tlsConfig := &tls.Config{
		ServerName:         s.host,
		InsecureSkipVerify: false,
	}

	// ✅ tls.Dial для порта 465
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		log.Printf("❌ TLS connection test failed: %v\n", err)
		return err
	}
	defer conn.Close()

	// Создаем SMTP клиент
	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		log.Printf("❌ SMTP client test failed: %v\n", err)
		return err
	}
	defer client.Close()

	// Auth test
	auth := smtp.PlainAuth("", s.from, s.password, s.host)
	if err = client.Auth(auth); err != nil {
		log.Printf("❌ Auth test failed: %v\n", err)
		return err
	}

	log.Printf("✅ SMTP connection test successful\n")
	return nil
}

func (s *SMTPEmailService) buildHTMLEmail(code string) string {
	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background: #0e1621; color: white; padding: 20px; margin: 0;">
	<div style="max-width: 500px; margin: 0 auto;">
		<!-- Header -->
		<div style="background: linear-gradient(135deg, #4faeef 0%, #2b5278 100%); border-radius: 12px 12px 0 0; padding: 40px 20px; text-align: center;">
			<h1 style="font-size: 32px; margin: 0; color: white;">💬</h1>
			<h2 style="font-size: 24px; margin: 10px 0 0 0; color: white;">Messenger</h2>
		</div>

		<!-- Content -->
		<div style="background: #17212b; padding: 40px 20px; border-radius: 0 0 12px 12px; text-align: center;">
			<h3 style="color: white; margin: 0 0 10px 0; font-size: 18px;">Verification Code</h3>
			<p style="color: #b0b9c1; margin: 0 0 30px 0; font-size: 14px;">Enter this code to verify your email address</p>

			<!-- Code Box -->
			<div style="background: #242f3d; border: 2px solid #4faeef; border-radius: 8px; padding: 30px 20px; margin: 20px 0;">
				<p style="color: #4faeef; font-size: 12px; text-transform: uppercase; letter-spacing: 2px; margin: 0 0 10px 0;">Your Code</p>
				<h1 style="color: #4faeef; font-size: 42px; letter-spacing: 8px; margin: 0; font-weight: bold; font-family: 'Courier New', monospace;">%s</h1>
			</div>

			<!-- Info -->
			<p style="color: #7f91a4; font-size: 12px; margin: 20px 0 0 0;">
				This code will expire in <strong>10 minutes</strong>
			</p>
			<p style="color: #7f91a4; font-size: 12px; margin: 10px 0 0 0;">
				If you didn't request this, please ignore this email.
			</p>

			<!-- Security Note -->
			<div style="background: rgba(79, 174, 239, 0.1); border-left: 4px solid #4faeef; padding: 15px; margin-top: 20px; border-radius: 4px; text-align: left;">
				<p style="color: #4faeef; font-size: 12px; margin: 0; font-weight: bold;">🔒 Security Notice</p>
				<p style="color: #b0b9c1; font-size: 11px; margin: 5px 0 0 0;">
					Never share this code with anyone. Messenger staff will never ask for your code.
				</p>
			</div>
		</div>

		<!-- Footer -->
		<div style="background: #0e1621; padding: 20px; text-align: center; border-radius: 8px; margin-top: 20px;">
			<p style="color: #7f91a4; font-size: 11px; margin: 0;">
				© 2026 Messenger. All rights reserved.
			</p>
		</div>
	</div>
</body>
</html>
	`, code)
}
