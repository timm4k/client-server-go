package gateway_test

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"dispatcher/gateway"
)

type peer struct {
	conn   net.Conn
	reader *bufio.Reader
}

func startServer(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := gateway.New(log.New(io.Discard, "", 0), 300*time.Millisecond)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Close()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("server did not stop")
		}
	})
	return listener.Addr().String()
}

func connect(t *testing.T, address string) *peer {
	t.Helper()
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	p := &peer{conn: conn, reader: bufio.NewReader(conn)}
	expect(t, p, "[SERVER] Welcome to Device & Dispatcher Gateway")
	expect(t, p, "[SERVER] Commands: "+gateway.Commands)
	return p
}

func read(t *testing.T, p *peer) string {
	t.Helper()
	p.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := p.reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(line)
}

func expect(t *testing.T, p *peer, want string) {
	t.Helper()
	if got := read(t, p); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func send(t *testing.T, p *peer, line string) {
	t.Helper()
	p.conn.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := fmt.Fprint(p.conn, line); err != nil {
		t.Fatal(err)
	}
}

func TestCommandsAndFraming(t *testing.T) {
	address := startServer(t)
	a, b := connect(t, address), connect(t, address)
	expect(t, a, "[SYSTEM] Client joined: "+b.conn.LocalAddr().String())
	for _, tc := range []struct{ input, output string }{
		{"  ECHO hello world  \r\n", "[ECHO]: hello world"},
		{"echo lower case\n", "[ECHO]: lower case"},
		{"UNKNOWN\n", "[ERROR] Unknown command. Available commands: " + gateway.Commands},
		{"\n", "[ERROR] Empty command. Available commands: " + gateway.Commands},
		{"ECHO\n", "[ERROR] Usage: ECHO <text>"},
		{"MSG\n", "[ERROR] Usage: MSG <text>"},
		{"BROADCAST\n", "[ERROR] Usage: BROADCAST <text>"},
		{"TIME extra\n", "[ERROR] Usage: TIME"},
		{"QUIT extra\n", "[ERROR] Usage: QUIT"},
	} {
		send(t, a, tc.input)
		expect(t, a, tc.output)
	}
	send(t, b, "TIME\n")
	if value := read(t, b); !strings.HasPrefix(value, "[TIME]: ") {
		t.Fatalf("invalid time response: %q", value)
	} else if _, err := time.Parse(time.RFC3339, strings.TrimPrefix(value, "[TIME]: ")); err != nil {
		t.Fatal(err)
	}
	send(t, a, "ECHO split")
	a.conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	if _, err := a.reader.Peek(1); err == nil {
		t.Fatal("server replied before the newline delimiter")
	} else if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatalf("expected a read timeout, got %v", err)
	}
	send(t, a, " message\nECHO next\n")
	expect(t, a, "[ECHO]: split message")
	expect(t, a, "[ECHO]: next")
	for _, item := range []struct {
		p       *peer
		command string
	}{{a, "CLOSE"}, {b, "QUIT"}} {
		send(t, item.p, item.command+"\n")
		expect(t, item.p, "[SERVER] Goodbye")
		item.p.conn.SetReadDeadline(time.Now().Add(time.Second))
		if _, err := item.p.reader.ReadString('\n'); err != io.EOF {
			t.Fatalf("connection was not closed: %v", err)
		}
		if item.p == a {
			expect(t, b, "[SYSTEM] Client left: "+a.conn.LocalAddr().String())
		}
	}
}

func TestBroadcastAndMembership(t *testing.T) {
	address := startServer(t)
	a := connect(t, address)
	b := connect(t, address)
	expect(t, a, "[SYSTEM] Client joined: "+b.conn.LocalAddr().String())
	c := connect(t, address)
	for _, p := range []*peer{a, b} {
		expect(t, p, "[SYSTEM] Client joined: "+c.conn.LocalAddr().String())
	}
	send(t, a, "BROADCAST sensor online\n")
	expect(t, a, "[SERVER] Broadcast delivered to 2 clients")
	for _, p := range []*peer{b, c} {
		expect(t, p, "[MESSAGE from "+a.conn.LocalAddr().String()+"]: sensor online")
	}
	send(t, b, "MSG system ready\n")
	expect(t, b, "[SERVER] Broadcast delivered to 2 clients")
	for _, p := range []*peer{a, c} {
		expect(t, p, "[MESSAGE from "+b.conn.LocalAddr().String()+"]: system ready")
	}
	c.conn.Close()
	for _, p := range []*peer{a, b} {
		expect(t, p, "[SYSTEM] Client left: "+c.conn.LocalAddr().String())
	}
	send(t, a, "MSG remaining clients\n")
	expect(t, a, "[SERVER] Broadcast delivered to 1 client")
	expect(t, b, "[MESSAGE from "+a.conn.LocalAddr().String()+"]: remaining clients")
}

func TestConcurrentBroadcasts(t *testing.T) {
	address := startServer(t)
	const clients, messages = 4, 20
	peers := make([]*peer, 0, clients)
	for i := 0; i < clients; i++ {
		p := connect(t, address)
		for _, previous := range peers {
			expect(t, previous, "[SYSTEM] Client joined: "+p.conn.LocalAddr().String())
		}
		peers = append(peers, p)
	}
	var wg sync.WaitGroup
	errors := make(chan error, clients)
	for id, p := range peers {
		wg.Add(1)
		go func(id int, p *peer) {
			defer wg.Done()
			expected := make(map[string]bool)
			for senderID, sender := range peers {
				if senderID == id {
					continue
				}
				for n := 0; n < messages; n++ {
					expected[fmt.Sprintf("[MESSAGE from %s]: client-%d-message-%d", sender.conn.LocalAddr(), senderID, n)] = true
				}
			}
			p.conn.SetDeadline(time.Now().Add(5 * time.Second))
			for n := 0; n < messages; n++ {
				if _, err := fmt.Fprintf(p.conn, "MSG client-%d-message-%d\n", id, n); err != nil {
					errors <- err
					return
				}
			}
			acks, received := 0, make(map[string]bool)
			for n := 0; n < clients*messages; n++ {
				line, err := p.reader.ReadString('\n')
				if err != nil {
					errors <- err
					return
				}
				line = strings.TrimSpace(line)
				if line == "[SERVER] Broadcast delivered to 3 clients" {
					acks++
				} else if expected[line] && !received[line] {
					received[line] = true
				} else {
					errors <- fmt.Errorf("unexpected response: %s", line)
					return
				}
			}
			if acks != messages || len(received) != (clients-1)*messages {
				errors <- fmt.Errorf("client %d: acknowledgements=%d, unique messages=%d", id, acks, len(received))
			}
		}(id, p)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}
