package gateway

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const Confirmation = "[SERVER]: ALERT RECEIVED AND VERIFIED"

type Mailer interface {
	Send(recipient, subject, text string) error
}

type Server struct {
	Listener  net.Listener
	Mailer    Mailer
	Recipient string
	Logger    *log.Logger
	Timeout   time.Duration
	wg        sync.WaitGroup
}

func (s *Server) Serve() error {
	defer s.wg.Wait()
	for {
		conn, err := s.Listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handle(conn)
		}()
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(s.Timeout)); err != nil {
		s.Logger.Printf("[ERROR] Set deadline: %v", err)
		return
	}
	secure, ok := conn.(*tls.Conn)
	if !ok {
		s.Logger.Print("[REJECTED] A TLS connection is required")
		return
	}
	if err := secure.Handshake(); err != nil {
		s.Logger.Printf("[REJECTED] TLS handshake from %s: %v", conn.RemoteAddr(), err)
		return
	}
	s.Logger.Printf("[CONNECTED] Verified mTLS connection from %s", conn.RemoteAddr())
	line, err := bufio.NewReader(secure).ReadString('\n')
	if err != nil {
		s.Logger.Printf("[ERROR] Read alert: %v", err)
		return
	}
	incident := strings.TrimSpace(line)
	if incident == "" {
		s.reply(secure, "[SERVER]: ERROR: alert must not be empty")
		return
	}
	registered := time.Now().Format(time.RFC3339)
	s.Logger.Printf("[ALERT] %s", incident)
	if s.Mailer != nil {
		body := fmt.Sprintf("Critical incident notification\nRegistered at: %s\n\nIncident: %s\n", registered, incident)
		if err := s.Mailer.Send(s.Recipient, "Critical incident notification", body); err != nil {
			s.Logger.Printf("[SMTP_ERROR] %v", err)
			s.reply(secure, "[SERVER]: ERROR: alert verified, email delivery failed")
			return
		}
		s.Logger.Printf("[SMTP] Alert email accepted by SMTP server for %s", s.Recipient)
	}
	s.reply(secure, Confirmation)
}

func (s *Server) reply(conn net.Conn, message string) {
	if err := conn.SetWriteDeadline(time.Now().Add(s.Timeout)); err != nil {
		s.Logger.Printf("[ERROR] Set reply deadline: %v", err)
		return
	}
	if _, err := fmt.Fprintln(conn, message); err != nil {
		s.Logger.Printf("[ERROR] Send confirmation: %v", err)
		return
	}
	s.Logger.Printf("[REPLY] %s", message)
}
