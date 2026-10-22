package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"dispatcher/gateway"
)

type peer struct {
	name   string
	conn   net.Conn
	reader *bufio.Reader
}

type scenario struct {
	address string
	peers   []*peer
	err     error
}

func (s *scenario) fail(err error) {
	if s.err == nil {
		s.err = err
	}
}

func (s *scenario) receive(p *peer) string {
	if s.err != nil {
		return ""
	}
	p.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := p.reader.ReadString('\n')
	if err != nil {
		s.fail(fmt.Errorf("client %s read: %w", p.name, err))
		return ""
	}
	line = strings.TrimSpace(line)
	fmt.Printf("[%s] << %s\n", p.name, line)
	return line
}

func (s *scenario) expect(p *peer, want string) {
	if got := s.receive(p); s.err == nil && got != want {
		s.fail(fmt.Errorf("client %s: got %q, want %q", p.name, got, want))
	}
}

func (s *scenario) send(p *peer, command string) {
	if s.err != nil {
		return
	}
	fmt.Printf("[%s] >> %s\n", p.name, command)
	p.conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if _, err := fmt.Fprintln(p.conn, command); err != nil {
		s.fail(err)
	}
}

func (s *scenario) connect(name string) *peer {
	if s.err != nil {
		return nil
	}
	conn, err := net.DialTimeout("tcp", s.address, 3*time.Second)
	if err != nil {
		s.fail(err)
		return nil
	}
	p := &peer{name: name, conn: conn, reader: bufio.NewReader(conn)}
	s.peers = append(s.peers, p)
	fmt.Printf("[%s] Connected | Local address: %s\n", name, conn.LocalAddr())
	s.expect(p, "[SERVER] Welcome to Device & Dispatcher Gateway")
	s.expect(p, "[SERVER] Commands: "+gateway.Commands)
	return p
}

func (s *scenario) disconnect(p *peer, command string) {
	s.send(p, command)
	s.expect(p, "[SERVER] Goodbye")
	if s.err != nil {
		return
	}
	p.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := p.reader.ReadString('\n'); err != io.EOF {
		s.fail(fmt.Errorf("client %s: expected connection close, got %v", p.name, err))
		return
	}
	fmt.Printf("[%s] Connection closed by server\n", p.name)
}

func (s *scenario) close() {
	for _, p := range s.peers {
		p.conn.Close()
	}
}

func checkCommands(address string) error {
	fmt.Println("=== LEVEL 1: COMMANDS AND TWO INDEPENDENT CLIENTS ===")
	s := &scenario{address: address}
	defer s.close()
	a, b := s.connect("A"), s.connect("B")
	if s.err != nil {
		return s.err
	}
	s.expect(a, "[SYSTEM] Client joined: "+b.conn.LocalAddr().String())
	s.send(a, "ECHO device online")
	s.expect(a, "[ECHO]: device online")
	s.send(b, "TIME")
	value := s.receive(b)
	if !strings.HasPrefix(value, "[TIME]: ") {
		s.fail(fmt.Errorf("invalid time response: %q", value))
	} else if _, err := time.Parse(time.RFC3339, strings.TrimPrefix(value, "[TIME]: ")); err != nil {
		s.fail(err)
	}
	s.send(a, "UNKNOWN")
	s.expect(a, "[ERROR] Unknown command. Available commands: "+gateway.Commands)
	s.disconnect(a, "CLOSE")
	s.expect(b, "[SYSTEM] Client left: "+a.conn.LocalAddr().String())
	s.disconnect(b, "QUIT")
	return s.err
}

func checkBroadcast(address string) error {
	fmt.Println("=== LEVEL 2: BROADCAST AND THREE INDEPENDENT CLIENTS ===")
	s := &scenario{address: address}
	defer s.close()
	a, b := s.connect("A"), s.connect("B")
	if s.err != nil {
		return s.err
	}
	s.expect(a, "[SYSTEM] Client joined: "+b.conn.LocalAddr().String())
	c := s.connect("C")
	if s.err != nil {
		return s.err
	}
	for _, p := range []*peer{a, b} {
		s.expect(p, "[SYSTEM] Client joined: "+c.conn.LocalAddr().String())
	}
	s.send(a, "BROADCAST sensor online")
	s.expect(a, "[SERVER] Broadcast delivered to 2 clients")
	for _, p := range []*peer{b, c} {
		s.expect(p, "[MESSAGE from "+a.conn.LocalAddr().String()+"]: sensor online")
	}
	s.send(b, "MSG dispatcher ready")
	s.expect(b, "[SERVER] Broadcast delivered to 2 clients")
	for _, p := range []*peer{a, c} {
		s.expect(p, "[MESSAGE from "+b.conn.LocalAddr().String()+"]: dispatcher ready")
	}
	s.disconnect(c, "QUIT")
	for _, p := range []*peer{a, b} {
		s.expect(p, "[SYSTEM] Client left: "+c.conn.LocalAddr().String())
	}
	s.send(a, "MSG remaining clients")
	s.expect(a, "[SERVER] Broadcast delivered to 1 client")
	s.expect(b, "[MESSAGE from "+a.conn.LocalAddr().String()+"]: remaining clients")
	s.disconnect(a, "CLOSE")
	s.expect(b, "[SYSTEM] Client left: "+a.conn.LocalAddr().String())
	s.disconnect(b, "QUIT")
	return s.err
}

func main() {
	address := flag.String("addr", "127.0.0.1:9000", "TCP server address")
	name := flag.String("scenario", "all", "all | commands | broadcast")
	flag.Parse()
	if *name != "all" && *name != "commands" && *name != "broadcast" {
		fmt.Fprintln(os.Stderr, "[ERROR] Scenario must be all, commands or broadcast")
		os.Exit(2)
	}
	checks := []struct {
		name string
		run  func(string) error
	}{{"commands", checkCommands}, {"broadcast", checkBroadcast}}
	for _, check := range checks {
		if *name != "all" && *name != check.name {
			continue
		}
		if err := check.run(*address); err != nil {
			fmt.Fprintln(os.Stderr, "[FAIL]", err)
			os.Exit(1)
		}
		fmt.Printf("[PASS] %s scenario\n\n", check.name)
	}
}
