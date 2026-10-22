package gateway_test

import (
	"crypto/tls"
	"errors"
	"io"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"securealert/gateway"
	"securealert/pki"
)

type notification struct {
	recipient string
	subject   string
	text      string
}

type recorder struct {
	mu    sync.Mutex
	items []notification
	err   error
}

func (r *recorder) Send(recipient, subject, text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = append(r.items, notification{recipient, subject, text})
	return r.err
}

func (r *recorder) snapshot() []notification {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]notification(nil), r.items...)
}

func start(t *testing.T, mailer gateway.Mailer) (string, string) {
	t.Helper()
	directory := t.TempDir()
	if err := pki.Generate(directory, false); err != nil {
		t.Fatal(err)
	}
	config, err := pki.ServerConfig(directory)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", config)
	if err != nil {
		t.Fatal(err)
	}
	server := &gateway.Server{
		Listener: listener, Mailer: mailer, Recipient: "duty-engineer@internal.net",
		Logger: log.New(io.Discard, "", 0), Timeout: 2 * time.Second,
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve() }()
	t.Cleanup(func() {
		listener.Close()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server did not finish")
		}
	})
	return listener.Addr().String(), directory
}

func TestVerifiedAlertDispatch(t *testing.T) {
	mailer := &recorder{}
	address, directory := start(t, mailer)
	config, err := pki.ClientConfig(directory, true)
	if err != nil {
		t.Fatal(err)
	}
	message := "CRITICAL_ERROR: Database connection timeout on node-04"
	response, err := gateway.SendAlert(address, config, message, 2*time.Second)
	if err != nil || response != gateway.Confirmation {
		t.Fatalf("response=%q error=%v", response, err)
	}
	items := mailer.snapshot()
	if len(items) != 1 || items[0].recipient != "duty-engineer@internal.net" || items[0].subject == "" || !strings.Contains(items[0].text, message) {
		t.Fatalf("incorrect email notification: %+v", items)
	}
	var registered string
	for _, line := range strings.Split(items[0].text, "\n") {
		if strings.HasPrefix(line, "Registered at: ") {
			registered = strings.TrimPrefix(line, "Registered at: ")
		}
	}
	if _, err := time.Parse(time.RFC3339, registered); err != nil {
		t.Fatalf("missing or invalid registration time: %v", err)
	}
}

func TestRejectUnverifiedConnections(t *testing.T) {
	mailer := &recorder{}
	address, directory := start(t, mailer)
	foreignDirectory := t.TempDir()
	if err := pki.Generate(foreignDirectory, false); err != nil {
		t.Fatal(err)
	}
	foreignCert, err := tls.LoadX509KeyPair(filepath.Join(foreignDirectory, "client.crt"), filepath.Join(foreignDirectory, "client.key"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"no-client-certificate", "foreign-client-certificate", "wrong-server-name", "expired-certificates"} {
		t.Run(name, func(t *testing.T) {
			config, err := pki.ClientConfig(directory, name != "no-client-certificate")
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "foreign-client-certificate":
				config.Certificates = []tls.Certificate{foreignCert}
			case "wrong-server-name":
				config.ServerName = "wrong.local"
			case "expired-certificates":
				config.Time = func() time.Time { return time.Now().AddDate(2, 0, 0) }
			}
			if _, err := gateway.SendAlert(address, config, "Rejected incident", 2*time.Second); err == nil {
				t.Fatal("unverified connection was accepted")
			}
		})
	}
	if items := mailer.snapshot(); len(items) != 0 {
		t.Fatalf("unverified clients triggered %d emails", len(items))
	}
}

func TestMTLSOnlyMode(t *testing.T) {
	address, directory := start(t, nil)
	config, err := pki.ClientConfig(directory, true)
	if err != nil {
		t.Fatal(err)
	}
	if response, err := gateway.SendAlert(address, config, "Level 1 check", 2*time.Second); err != nil || response != gateway.Confirmation {
		t.Fatalf("response=%q error=%v", response, err)
	}
}

func TestSMTPFailureIsReported(t *testing.T) {
	mailer := &recorder{err: errors.New("test SMTP failure")}
	address, directory := start(t, mailer)
	config, err := pki.ClientConfig(directory, true)
	if err != nil {
		t.Fatal(err)
	}
	response, err := gateway.SendAlert(address, config, "SMTP failure check", 2*time.Second)
	if err == nil || !strings.Contains(response, "email delivery failed") {
		t.Fatalf("SMTP failure was hidden: response=%q error=%v", response, err)
	}
}
