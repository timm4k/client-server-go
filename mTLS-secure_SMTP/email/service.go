package email

import (
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type Service struct {
	Address string
	From    string
	Timeout time.Duration
}

func (s Service) Send(recipient, subject, text string) error {
	if strings.ContainsAny(s.From+recipient+subject, "\r\n") {
		return fmt.Errorf("email headers must not contain line breaks")
	}
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return fmt.Errorf("sender address: %w", err)
	}
	to, err := mail.ParseAddress(recipient)
	if err != nil {
		return fmt.Errorf("recipient address: %w", err)
	}
	host, _, err := net.SplitHostPort(s.Address)
	if err != nil {
		return fmt.Errorf("SMTP address: %w", err)
	}
	conn, err := net.DialTimeout("tcp", s.Address, s.Timeout)
	if err != nil {
		return fmt.Errorf("SMTP connect: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(s.Timeout)); err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()
	if supported, _ := client.Extension("STARTTLS"); supported {
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("SMTP STARTTLS: %w", err)
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return err
	}
	if err := client.Rcpt(to.Address); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\n", "\r\n")
	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n",
		from.String(), to.String(), mime.QEncoding.Encode("UTF-8", subject), time.Now().Format(time.RFC1123Z), text)
	if _, err := writer.Write([]byte(message)); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
