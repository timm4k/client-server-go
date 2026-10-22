package main

import (
	"flag"
	"log"
	"net"
	"os"
	"time"

	"dispatcher/gateway"
)

func main() {
	address := flag.String("addr", "127.0.0.1:9000", "TCP address to listen on")
	writeTimeout := flag.Duration("write-timeout", 2*time.Second, "Timeout for each client write")
	flag.Parse()
	logger := log.New(os.Stdout, "", log.Ltime)
	if *writeTimeout <= 0 || flag.NArg() != 0 {
		logger.Print("[ERROR] Use a positive -write-timeout and no positional arguments")
		os.Exit(2)
	}
	listener, err := net.Listen("tcp", *address)
	if err != nil {
		logger.Printf("[ERROR] Listen: %v", err)
		os.Exit(1)
	}
	server := gateway.New(logger, *writeTimeout)
	defer server.Close()
	logger.Print("=== Device & Dispatcher Gateway | TCP | Levels 1 and 2 ===")
	if err := server.Serve(listener); err != nil {
		logger.Printf("[ERROR] Server: %v", err)
	}
}
