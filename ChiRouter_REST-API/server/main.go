package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"deviceregistry/internal/handler"
	"deviceregistry/internal/repository"
)

func main() {
	address := flag.String("addr", "127.0.0.1:8080", "HTTP listener address")
	flag.Parse()
	logger := log.New(os.Stdout, "", log.Ltime)
	if flag.NArg() != 0 {
		logger.Print("[ERROR] No positional arguments are accepted")
		os.Exit(2)
	}
	server := &http.Server{
		Addr: *address, Handler: handler.NewRouter(repository.NewMemory(), logger),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
	}
	logger.Print("[SERVER] In-memory device repository ready | sync.RWMutex enabled")
	logger.Print("[SERVER] Chi routes: /api/v1/devices | GET, POST, PUT, DELETE")
	logger.Printf("[SERVER] Listening at http://%s | HTTP request logging enabled", *address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Printf("[ERROR] HTTP server: %v", err)
		os.Exit(1)
	}
}
