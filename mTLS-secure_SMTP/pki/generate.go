package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

func Generate(directory string, overwrite bool) error {
	if !overwrite {
		for _, name := range []string{"ca", "server", "client"} {
			for _, extension := range []string{".crt", ".key"} {
				filename := filepath.Join(directory, name+extension)
				if _, err := os.Stat(filename); err == nil {
					return fmt.Errorf("%s already exists; use -force to regenerate the complete set", filename)
				} else if !os.IsNotExist(err) {
					return err
				}
			}
		}
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	ca, err := certificate("Secure Alert Gateway Root CA")
	if err != nil {
		return err
	}
	ca.IsCA = true
	ca.BasicConstraintsValid = true
	ca.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	if err := writePair(directory, "ca", ca, ca, caKey, caKey); err != nil {
		return err
	}
	for _, name := range []string{"server", "client"} {
		commonName := "localhost"
		if name == "client" {
			commonName = "dispatcher-agent-01"
		}
		cert, err := certificate(commonName)
		if err != nil {
			return err
		}
		cert.KeyUsage = x509.KeyUsageDigitalSignature
		if name == "server" {
			cert.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
			cert.DNSNames = []string{"localhost"}
			cert.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
		} else {
			cert.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		}
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return err
		}
		if err := writePair(directory, name, cert, ca, key, caKey); err != nil {
			return err
		}
	}
	return nil
}

func certificate(commonName string) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	serial.Add(serial, big.NewInt(1))
	now := time.Now()
	return &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    now.Add(-5 * time.Minute),
		NotAfter:     now.AddDate(1, 0, 0),
	}, nil
}

func writePair(directory, name string, cert, parent *x509.Certificate, key, parentKey *ecdsa.PrivateKey) error {
	der, err := x509.CreateCertificate(rand.Reader, cert, parent, &key.PublicKey, parentKey)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(directory, name+".crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, name+".key"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600)
}
