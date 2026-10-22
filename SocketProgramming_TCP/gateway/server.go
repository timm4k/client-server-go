package gateway

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type Server struct {
	mu           sync.Mutex
	clients      map[*client]struct{}
	listener     net.Listener
	closed       bool
	wg           sync.WaitGroup
	logger       *log.Logger
	writeTimeout time.Duration
}

func New(logger *log.Logger, writeTimeout time.Duration) *Server {
	return &Server{
		clients:      make(map[*client]struct{}),
		logger:       logger,
		writeTimeout: writeTimeout,
	}
}

func (s *Server) Serve(listener net.Listener) error {
	s.mu.Lock()
	if s.closed || s.listener != nil {
		s.mu.Unlock()
		return errors.New("server is closed or already running")
	}
	s.listener = listener
	s.mu.Unlock()
	s.logger.Printf("[SERVER] TCP listener ready at %s", listener.Addr())
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept connection: %w", err)
		}
		c := &client{conn: conn, timeout: s.writeTimeout}
		if !s.register(c) {
			conn.Close()
			continue
		}
		go s.handle(c)
	}
}

func (s *Server) register(c *client) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	if err := c.send("[SERVER] Welcome to Device & Dispatcher Gateway"); err != nil {
		return false
	}
	if err := c.send("[SERVER] Commands: " + Commands); err != nil {
		return false
	}
	s.clients[c] = struct{}{}
	s.wg.Add(1)
	s.logger.Printf("[CONNECT] %s | Active clients: %d", c.conn.RemoteAddr(), len(s.clients))
	s.broadcastLocked("[SYSTEM] Client joined: "+c.conn.RemoteAddr().String(), c)
	return true
}

func (s *Server) remove(c *client) {
	c.conn.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, c)
	s.logger.Printf("[DISCONNECT] %s | Active clients: %d", c.conn.RemoteAddr(), len(s.clients))
	if !s.closed {
		s.broadcastLocked("[SYSTEM] Client left: "+c.conn.RemoteAddr().String(), c)
	}
}

func (s *Server) handle(c *client) {
	defer s.wg.Done()
	defer s.remove(c)
	reader := bufio.NewReader(c.conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				s.logger.Printf("[READ_ERROR] %s: %v", c.conn.RemoteAddr(), err)
			}
			return
		}
		if !s.execute(c, strings.TrimSpace(line)) {
			return
		}
	}
}

func (s *Server) broadcastLocked(message string, sender *client) int {
	delivered := 0
	for target := range s.clients {
		if target == sender {
			continue
		}
		if err := target.send(message); err != nil {
			s.logger.Printf("[WRITE_ERROR] %s: %v", target.conn.RemoteAddr(), err)
			target.conn.Close()
			continue
		}
		delivered++
	}
	return delivered
}

func (s *Server) broadcast(sender *client, message string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0
	}
	delivered := s.broadcastLocked(fmt.Sprintf("[MESSAGE from %s]: %s", sender.conn.RemoteAddr(), message), sender)
	s.logger.Printf("[BROADCAST] From %s | Recipients: %d | Message: %s", sender.conn.RemoteAddr(), delivered, message)
	return delivered
}

func (s *Server) Close() {
	s.mu.Lock()
	s.closed = true
	listener := s.listener
	connections := make([]net.Conn, 0, len(s.clients))
	for c := range s.clients {
		connections = append(connections, c.conn)
	}
	s.mu.Unlock()
	if listener != nil {
		listener.Close()
	}
	for _, conn := range connections {
		conn.Close()
	}
	s.wg.Wait()
}
