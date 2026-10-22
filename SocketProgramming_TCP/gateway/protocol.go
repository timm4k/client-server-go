package gateway

import (
	"fmt"
	"strings"
	"time"
)

const Commands = "ECHO <text>, TIME, CLOSE, QUIT, BROADCAST <text>, MSG <text>"

func (s *Server) execute(c *client, line string) bool {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return c.send("[ERROR] Empty command. Available commands: "+Commands) == nil
	}
	command := strings.ToUpper(fields[0])
	payload := strings.TrimSpace(line[len(fields[0]):])
	var response string
	switch command {
	case "ECHO":
		if payload == "" {
			response = "[ERROR] Usage: ECHO <text>"
		} else {
			response = "[ECHO]: " + payload
		}
	case "TIME":
		if payload != "" {
			response = "[ERROR] Usage: TIME"
		} else {
			response = "[TIME]: " + time.Now().Format(time.RFC3339)
		}
	case "CLOSE", "QUIT":
		if payload != "" {
			response = "[ERROR] Usage: " + command
			break
		}
		c.send("[SERVER] Goodbye")
		return false
	case "BROADCAST", "MSG":
		if payload == "" {
			response = "[ERROR] Usage: " + command + " <text>"
		} else {
			count := s.broadcast(c, payload)
			noun := "clients"
			if count == 1 {
				noun = "client"
			}
			response = fmt.Sprintf("[SERVER] Broadcast delivered to %d %s", count, noun)
		}
	default:
		response = "[ERROR] Unknown command. Available commands: " + Commands
	}
	return c.send(response) == nil
}
