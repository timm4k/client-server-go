package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	address := flag.String("addr", "127.0.0.1:9000", "TCP server address")
	flag.Parse()
	conn, err := net.DialTimeout("tcp", *address, 3*time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, "[ERROR] Connect:", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Printf("[CLIENT] Connected to %s | Local address: %s\n", conn.RemoteAddr(), conn.LocalAddr())
	done := make(chan struct{})
	go func() {
		defer close(done)
		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			fmt.Print(line)
		}
	}()
	input := make(chan string)
	go func() {
		defer close(input)
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			select {
			case input <- scanner.Text():
			case <-done:
				return
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "[ERROR] Read input:", err)
		}
	}()
	for {
		select {
		case <-done:
			fmt.Println("[CLIENT] Connection closed")
			return
		case line, open := <-input:
			if !open {
				if tcp, ok := conn.(*net.TCPConn); ok {
					tcp.CloseWrite()
				}
				input = nil
				continue
			}
			if _, err := fmt.Fprintln(conn, line); err != nil {
				fmt.Fprintln(os.Stderr, "[ERROR] Send:", err)
				return
			}
		}
	}
}
