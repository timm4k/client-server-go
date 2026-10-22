package pki_test

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"

	"securealert/pki"
)

func TestCertificateChainAndIdentity(t *testing.T) {
	directory := t.TempDir()
	if err := pki.Generate(directory, false); err != nil {
		t.Fatal(err)
	}
	caPEM, err := os.ReadFile(filepath.Join(directory, "ca.crt"))
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("invalid Root CA")
	}
	for _, name := range []string{"server", "client"} {
		pair, err := tls.LoadX509KeyPair(filepath.Join(directory, name+".crt"), filepath.Join(directory, name+".key"))
		if err != nil {
			t.Fatal(err)
		}
		cert, err := x509.ParseCertificate(pair.Certificate[0])
		if err != nil {
			t.Fatal(err)
		}
		options := x509.VerifyOptions{Roots: roots}
		if name == "server" {
			options.DNSName = "localhost"
			options.KeyUsages = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
			if err := cert.VerifyHostname("127.0.0.1"); err != nil {
				t.Fatal(err)
			}
		} else {
			options.KeyUsages = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
			if cert.Subject.CommonName != "dispatcher-agent-01" {
				t.Fatal("incorrect client CN")
			}
		}
		if _, err := cert.Verify(options); err != nil {
			t.Fatal(err)
		}
	}
}
