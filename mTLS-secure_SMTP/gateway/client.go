package gateway

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"
)

func SendAlert(address string, config *tls.Config, message string, timeout time.Duration) (string, error) {
	if strings.TrimSpace(message) == "" || strings.ContainsAny(message, "\r\n") {
		return "", fmt.Errorf("alert must contain one non-empty line")
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", address, config)
	if err != nil {
		return "", fmt.Errorf("TLS connection: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return "", err
	}
	if _, err := fmt.Fprintln(conn, message); err != nil {
		return "", fmt.Errorf("send alert: %w", err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("receive confirmation: %w", err)
	}
	response := strings.TrimSpace(line)
	if response != Confirmation {
		return response, fmt.Errorf("gateway rejected alert: %s", response)
	}
	return response, nil
}
