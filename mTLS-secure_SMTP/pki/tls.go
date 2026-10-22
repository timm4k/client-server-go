package pki

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
)

func roots(directory string) (*x509.CertPool, error) {
	data, err := os.ReadFile(filepath.Join(directory, "ca.crt"))
	if err != nil {
		return nil, fmt.Errorf("read Root CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("ca.crt contains no valid certificates")
	}
	return pool, nil
}

func ServerConfig(directory string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(filepath.Join(directory, "server.crt"), filepath.Join(directory, "server.key"))
	if err != nil {
		return nil, fmt.Errorf("load server certificate: %w", err)
	}
	pool, err := roots(directory)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

func ClientConfig(directory string, withCertificate bool) (*tls.Config, error) {
	pool, err := roots(directory)
	if err != nil {
		return nil, err
	}
	config := &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS13}
	if withCertificate {
		cert, err := tls.LoadX509KeyPair(filepath.Join(directory, "client.crt"), filepath.Join(directory, "client.key"))
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		config.Certificates = []tls.Certificate{cert}
	}
	return config, nil
}
