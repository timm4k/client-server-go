package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"securealert/gateway"
	"securealert/pki"
)

func main() {
	address := flag.String("addr", "127.0.0.1:8443", "mTLS gateway address")
	directory := flag.String("certs", "certs", "Certificate directory")
	message := flag.String("message", "CRITICAL_ERROR: Database connection timeout on node-04", "One-line incident message")
	noCertificate := flag.Bool("no-cert", false, "Attempt a connection without a client certificate")
	timeout := flag.Duration("timeout", 10*time.Second, "Connection and response timeout")
	flag.Parse()
	if *timeout <= 0 || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "[ERROR] Use a positive timeout and no positional arguments")
		os.Exit(2)
	}
	config, err := pki.ClientConfig(*directory, !*noCertificate)
	if err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] TLS configuration:", err)
		os.Exit(1)
	}
	fmt.Printf("[CLIENT] Gateway: %s | Minimum TLS: 1.3 | Client certificate: %t\n", *address, !*noCertificate)
	fmt.Printf("[CLIENT] Alert: %s\n", *message)
	response, err := gateway.SendAlert(*address, config, *message, *timeout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR]", err)
		os.Exit(1)
	}
	fmt.Println("[CLIENT] Mutual TLS verification and alert exchange succeeded")
	fmt.Println(response)
}
