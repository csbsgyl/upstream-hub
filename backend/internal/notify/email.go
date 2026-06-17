package notify

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/worryzyy/upstream-hub/internal/storage"
)

func init() {
	Register(storage.NotifyEmail, func(raw string) (Notifier, error) { return newEmail(raw) })
}

type emailConfig struct {
	Host     string   `json:"host"`     // smtp.example.com
	Port     int      `json:"port"`     // 465 / 587
	Username string   `json:"username"` // SMTP 用户名
	Password string   `json:"password"` // SMTP 密码 / 授权码
	From     string   `json:"from"`     // 发件人（可与 Username 不同）
	To       []string `json:"to"`       // 收件人列表
	UseTLS   bool     `json:"use_tls"`  // 是否使用隐式 TLS（一般 465 端口）
}

type email struct{ cfg emailConfig }

const smtpTimeout = 45 * time.Second

func newEmail(raw string) (*email, error) {
	var cfg emailConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, err
	}
	if cfg.Host == "" || cfg.Port == 0 || cfg.From == "" || len(cfg.To) == 0 {
		return nil, errors.New("email config requires host/port/from/to")
	}
	if hasHeaderNewline(cfg.From) {
		return nil, errors.New("email from contains invalid newline")
	}
	for _, to := range cfg.To {
		if hasHeaderNewline(to) {
			return nil, errors.New("email recipient contains invalid newline")
		}
	}
	return &email{cfg: cfg}, nil
}

func (e *email) Type() storage.NotificationChannelType { return storage.NotifyEmail }

func (e *email) Send(ctx context.Context, msg Message) error {
	addr := fmt.Sprintf("%s:%d", e.cfg.Host, e.cfg.Port)
	var auth smtp.Auth
	if e.cfg.Username != "" || e.cfg.Password != "" {
		auth = smtp.PlainAuth("", e.cfg.Username, e.cfg.Password, e.cfg.Host)
	}

	body := buildEmailBody(e.cfg.From, e.cfg.To, msg.Subject, msg.Body)

	if e.cfg.UseTLS {
		return sendTLS(ctx, addr, e.cfg.Host, auth, e.cfg.From, e.cfg.To, []byte(body))
	}
	return sendSMTP(ctx, addr, e.cfg.Host, auth, e.cfg.From, e.cfg.To, []byte(body))
}

func buildEmailBody(from string, to []string, subject, body string) string {
	headers := []string{
		"From: " + from,
		"To: " + strings.Join(to, ", "),
		"Subject: " + sanitizeHeaderValue(subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
	}
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + body
}

func hasHeaderNewline(s string) bool {
	return strings.ContainsAny(s, "\r\n")
}

func sanitizeHeaderValue(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func sendSMTP(ctx context.Context, addr, host string, auth smtp.Auth, from string, to []string, body []byte) error {
	conn, err := (&net.Dialer{Timeout: smtpTimeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(deadlineFromContext(ctx)); err != nil {
		return fmt.Errorf("smtp set deadline: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer client.Quit()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}
	return sendSMTPWithClient(client, auth, from, to, body)
}

// sendTLS 通过 SMTPS（隐式 TLS，常见于 465）发送邮件。
func sendTLS(ctx context.Context, addr, host string, auth smtp.Auth, from string, to []string, body []byte) error {
	tlsConfig := &tls.Config{ServerName: host}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: smtpTimeout}, "tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("smtp tls dial: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(deadlineFromContext(ctx)); err != nil {
		return fmt.Errorf("smtp set deadline: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer client.Quit()
	return sendSMTPWithClient(client, auth, from, to, body)
}

func sendSMTPWithClient(client *smtp.Client, auth smtp.Auth, from string, to []string, body []byte) error {
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp rcpt %s: %w", rcpt, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}
	return nil
}

func deadlineFromContext(ctx context.Context) time.Time {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < smtpTimeout {
		return deadline
	}
	return time.Now().Add(smtpTimeout)
}
