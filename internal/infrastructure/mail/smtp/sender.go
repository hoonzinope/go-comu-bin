package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	netmail "net/mail"
	netsmtp "net/smtp"
	"net/url"
	"strings"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/config"
)

var _ port.PasswordResetMailSender = (*Sender)(nil)
var _ port.EmailVerificationMailSender = (*Sender)(nil)

type Sender struct {
	host                     string
	port                     int
	username                 string
	password                 string
	from                     string
	startTLS                 bool
	implicitTLS              bool
	emailVerificationBaseURL string
	passwordResetBaseURL     string
	sendMail                 func(ctx context.Context, addr string, auth netsmtp.Auth, from string, to []string, msg []byte) error
}

func NewSender(cfg config.Config) *Sender {
	sender := &Sender{
		host:                     strings.TrimSpace(cfg.Delivery.Mail.SMTP.Host),
		port:                     cfg.Delivery.Mail.SMTP.Port,
		username:                 strings.TrimSpace(cfg.Delivery.Mail.SMTP.Username),
		password:                 cfg.Delivery.Mail.SMTP.Password,
		from:                     strings.TrimSpace(cfg.Delivery.Mail.SMTP.From),
		startTLS:                 cfg.Delivery.Mail.SMTP.StartTLS,
		implicitTLS:              cfg.Delivery.Mail.SMTP.ImplicitTLS,
		emailVerificationBaseURL: strings.TrimSpace(cfg.Delivery.Mail.EmailVerification.BaseURL),
		passwordResetBaseURL:     strings.TrimSpace(cfg.Delivery.Mail.PasswordReset.BaseURL),
	}
	sender.sendMail = sender.sendBlocking
	return sender
}

func (s *Sender) SendPasswordReset(ctx context.Context, email, token string, expiresAt time.Time) error {
	resetURL := s.passwordResetBaseURL
	if resetURL != "" {
		resetURL += "?token=" + url.QueryEscape(token)
	}
	body := fmt.Sprintf(
		"Use the following password reset link before %s:\n\n%s\n\nIf needed, you can also enter this token manually:\n\n%s\n",
		expiresAt.UTC().Format(time.RFC3339),
		resetURL,
		token,
	)
	return s.send(ctx, email, "Password reset", body)
}

func (s *Sender) SendEmailVerification(ctx context.Context, email, token string, expiresAt time.Time) error {
	verifyURL := s.emailVerificationBaseURL
	if verifyURL != "" {
		verifyURL += "?token=" + url.QueryEscape(token)
	}
	body := fmt.Sprintf(
		"Use the following email verification link before %s:\n\n%s\n\nIf needed, you can also enter this token manually:\n\n%s\n",
		expiresAt.UTC().Format(time.RFC3339),
		verifyURL,
		token,
	)
	return s.send(ctx, email, "Email verification", body)
}

func (s *Sender) send(ctx context.Context, recipient, subject, body string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	msg, err := s.buildMessage(recipient, subject, body)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.sendMail == nil {
		return s.sendBlocking(ctx, fmt.Sprintf("%s:%d", s.host, s.port), s.auth(), s.from, []string{recipient}, msg)
	}
	return s.sendMail(ctx, fmt.Sprintf("%s:%d", s.host, s.port), s.auth(), s.from, []string{recipient}, msg)
}

func (s *Sender) sendBlocking(ctx context.Context, addr string, auth netsmtp.Auth, from string, to []string, msg []byte) error {
	if len(to) == 0 {
		return fmt.Errorf("missing recipient")
	}
	recipient := to[0]
	if s.implicitTLS {
		return s.sendImplicitTLS(ctx, addr, auth, from, recipient, msg)
	}
	return s.sendSMTP(ctx, addr, auth, from, recipient, msg)
}

func (s *Sender) sendSMTP(ctx context.Context, addr string, auth netsmtp.Auth, from, recipient string, msg []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := watchContext(ctx, conn)
	defer stop()
	client, err := netsmtp.NewClient(conn, s.host)
	if err != nil {
		return err
	}
	defer client.Close()
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
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

func (s *Sender) sendImplicitTLS(ctx context.Context, addr string, auth netsmtp.Auth, from, recipient string, msg []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dialer := &net.Dialer{}
	rawConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer rawConn.Close()
	tlsConn := tls.Client(rawConn, &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12})
	stop := watchContext(ctx, tlsConn)
	defer stop()
	if err := tlsConn.Handshake(); err != nil {
		return err
	}
	client, err := netsmtp.NewClient(tlsConn, s.host)
	if err != nil {
		return err
	}
	defer client.Close()
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
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

func watchContext(ctx context.Context, closer net.Conn) func() {
	if ctx == nil {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = closer.Close()
		case <-done:
		}
	}()
	return func() {
		close(done)
	}
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
