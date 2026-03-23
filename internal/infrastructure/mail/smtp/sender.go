package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	netmail "net/mail"
	netsmtp "net/smtp"
	"strings"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/config"
)

var _ port.PasswordResetMailSender = (*Sender)(nil)
var _ port.EmailVerificationMailSender = (*Sender)(nil)

type Sender struct {
	host        string
	port        int
	username    string
	password    string
	from        string
	startTLS    bool
	implicitTLS bool
	sendMail    func(addr string, auth netsmtp.Auth, from string, to []string, msg []byte) error
	dialTLS     func(network, addr string, tlsConfig *tls.Config) (*tls.Conn, error)
}

func NewSender(cfg config.Config) *Sender {
	return &Sender{
		host:        strings.TrimSpace(cfg.Delivery.Mail.SMTP.Host),
		port:        cfg.Delivery.Mail.SMTP.Port,
		username:    strings.TrimSpace(cfg.Delivery.Mail.SMTP.Username),
		password:    cfg.Delivery.Mail.SMTP.Password,
		from:        strings.TrimSpace(cfg.Delivery.Mail.SMTP.From),
		startTLS:    cfg.Delivery.Mail.SMTP.StartTLS,
		implicitTLS: cfg.Delivery.Mail.SMTP.ImplicitTLS,
		sendMail:    netsmtp.SendMail,
		dialTLS:     tls.Dial,
	}
}

func (s *Sender) SendPasswordReset(ctx context.Context, email, token string, expiresAt time.Time) error {
	return s.send(ctx, email, "Password reset", fmt.Sprintf("Use this password reset token before %s:\n\n%s\n", expiresAt.UTC().Format(time.RFC3339), token))
}

func (s *Sender) SendEmailVerification(ctx context.Context, email, token string, expiresAt time.Time) error {
	return s.send(ctx, email, "Email verification", fmt.Sprintf("Use this email verification token before %s:\n\n%s\n", expiresAt.UTC().Format(time.RFC3339), token))
}

func (s *Sender) send(ctx context.Context, recipient, subject, body string) error {
	_ = ctx
	msg, err := s.buildMessage(recipient, subject, body)
	if err != nil {
		return err
	}
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	if s.implicitTLS {
		return s.sendImplicitTLS(addr, recipient, msg)
	}
	return s.sendMail(addr, s.auth(), s.from, []string{recipient}, msg)
}

func (s *Sender) sendImplicitTLS(addr, recipient string, msg []byte) error {
	conn, err := s.dialTLS("tcp", addr, &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	defer conn.Close()
	client, err := netsmtp.NewClient(conn, s.host)
	if err != nil {
		return err
	}
	defer client.Close()
	if auth := s.auth(); auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(s.from); err != nil {
		return err
	}
	if err := client.Rcpt(recipient); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func (s *Sender) auth() netsmtp.Auth {
	if s.username == "" {
		return nil
	}
	return netsmtp.PlainAuth("", s.username, s.password, s.host)
}

func (s *Sender) buildMessage(recipient, subject, body string) ([]byte, error) {
	if _, err := netmail.ParseAddress(s.from); err != nil {
		return nil, err
	}
	if _, err := netmail.ParseAddress(recipient); err != nil {
		return nil, err
	}
	payload := strings.Join([]string{
		fmt.Sprintf("From: %s", s.from),
		fmt.Sprintf("To: %s", recipient),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")
	return []byte(payload), nil
}
