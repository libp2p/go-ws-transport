package websocket

import (
	"io"
	"net"
	"time"

	ws "github.com/gorilla/websocket"
)

var _ net.Conn = (*Conn)(nil)

// Conn implements net.Conn interface for gorilla/websocket.
type Conn struct {
	*ws.Conn
	DefaultMessageType int
	done               func()
	reader             io.Reader
}

func (c *Conn) Read(b []byte) (int, error) {
	if c.reader == nil {
		if err := c.prepNextReader(); err != nil {
			return 0, err
		}
	}

	for {
		n, err := c.reader.Read(b)
		switch err {
		case io.EOF:
			c.reader = nil

			if n > 0 {
				return n, nil
			}

			if err := c.prepNextReader(); err != nil {
				return 0, err
			}

			// explicitly looping
		default:
			return n, err
		}
	}
}

func (c *Conn) prepNextReader() error {
	t, r, err := c.Conn.NextReader()
	if err != nil {
		if wserr, ok := err.(*ws.CloseError); ok {
			if wserr.Code == 1000 || wserr.Code == 1005 {
				return io.EOF
			}
		}
		return err
	}

	if t == ws.CloseMessage {
		return io.EOF
	}

	c.reader = r
	return nil
}

func (c *Conn) Write(b []byte) (n int, err error) {
	if err := c.Conn.WriteMessage(c.DefaultMessageType, b); err != nil {
		return 0, err
	}

	return len(b), nil
}

func (c *Conn) Close() error {
	if c.done != nil {
		c.done()
	}

	c.Conn.WriteMessage(ws.CloseMessage, nil)
	return c.Conn.Close()
}

func (c *Conn) LocalAddr() net.Addr {
	return NewAddr(c.Conn.LocalAddr().String())
}

func (c *Conn) RemoteAddr() net.Addr {
	return NewAddr(c.Conn.RemoteAddr().String())
}

func (c *Conn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}

	return c.SetWriteDeadline(t)
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.Conn.SetReadDeadline(t)
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.Conn.SetWriteDeadline(t)
}

// NewConn creates a Conn given a regular gorilla/websocket Conn.
func NewConn(raw *ws.Conn, done func()) *Conn {
	return &Conn{
		Conn:               raw,
		DefaultMessageType: ws.BinaryMessage,
		done:               done,
	}
}
