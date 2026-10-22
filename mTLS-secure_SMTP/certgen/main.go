package main

import (
	"flag"
	"fmt"
	"os"

	"securealert/pki"
)

func main() {
	directory := flag.String("dir", "certs", "Certificate output directory")
	force := flag.Bool("force", false, "Replace existing local certificates and keys")
	flag.Parse()
	if err := pki.Generate(*directory, *force); err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] Generate certificates:", err)
		os.Exit(1)
	}
	fmt.Println("[PKI] Local Root CA generated: ca.crt, ca.key")
	fmt.Println("[PKI] Server certificate generated: server.crt, server.key")
	fmt.Println("[PKI] Server SAN: localhost, 127.0.0.1, ::1")
	fmt.Println("[PKI] Client certificate generated: client.crt, client.key")
	fmt.Println("[PKI] Client CN: dispatcher-agent-01")
	fmt.Printf("[PKI] Certificate directory: %s\n", *directory)
}
