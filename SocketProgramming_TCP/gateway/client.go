package gateway

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type client struct {
	conn    net.Conn
	writeMu sync.Mutex
	timeout time.Duration
}

func (c *client) send(message string) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return err
	}
	_, err := fmt.Fprintln(c.conn, message)
	return err
}
