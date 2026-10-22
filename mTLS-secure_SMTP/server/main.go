package main

import (
	"crypto/tls"
	"flag"
	"log"
	"os"
	"time"

	"securealert/email"
	"securealert/gateway"
	"securealert/pki"
)

func main() {
	address := flag.String("addr", "127.0.0.1:8443", "mTLS listener address")
	directory := flag.String("certs", "certs", "Certificate directory")
	smtpAddress := flag.String("smtp", "127.0.0.1:1025", "SMTP server address")
	recipient := flag.String("to", "duty-engineer@internal.net", "Alert email recipient")
	from := flag.String("from", "gateway@internal.net", "Alert email sender")
	emailEnabled := flag.Bool("email", true, "Send email notifications after verified alerts")
	timeout := flag.Duration("timeout", 5*time.Second, "Network operation timeout")
	flag.Parse()
	logger := log.New(os.Stdout, "", log.Ltime)
	if *timeout <= 0 || flag.NArg() != 0 {
		logger.Print("[ERROR] Use a positive timeout and no positional arguments")
		os.Exit(2)
	}
	config, err := pki.ServerConfig(*directory)
	if err != nil {
		logger.Printf("[ERROR] TLS configuration: %v", err)
		os.Exit(1)
	}
	listener, err := tls.Listen("tcp", *address, config)
	if err != nil {
		logger.Printf("[ERROR] TLS listen: %v", err)
		os.Exit(1)
	}
	defer listener.Close()
	server := &gateway.Server{Listener: listener, Recipient: *recipient, Logger: logger, Timeout: *timeout}
	if *emailEnabled {
		server.Mailer = email.Service{Address: *smtpAddress, From: *from, Timeout: *timeout}
		logger.Printf("[SMTP] Target: %s | Recipient: %s", *smtpAddress, *recipient)
	} else {
		logger.Print("[SMTP] Email notifications disabled | mTLS-only mode")
	}
	logger.Printf("[SERVER] mTLS listener ready at %s | TLS 1.3 | Client certificates required", listener.Addr())
	if err := server.Serve(); err != nil {
		logger.Printf("[ERROR] Server: %v", err)
	}
}
