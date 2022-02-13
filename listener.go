package websocket

import (
	"fmt"
	"net"
	"net/http"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

type listener struct {
	net.Listener
	laddr ma.Multiaddr

	closed   chan struct{}
	incoming chan *Conn
}

func newListener(l net.Listener) (*listener, error) {
	laddr, err := manet.FromNetAddr(l.Addr())
	if err != nil {
		return nil, err
	}
	return &listener{
		Listener: l,
		laddr:    laddr.Encapsulate(wsma),
		incoming: make(chan *Conn),
		closed:   make(chan struct{}),
	}, nil
}

func (l *listener) serve() {
	defer close(l.closed)
	_ = http.Serve(l.Listener, l)
}

func (l *listener) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// The upgrader writes a response for us.
		return
	}

	select {
	case l.incoming <- NewConn(c, false):
	case <-l.closed:
		c.Close()
	}
	// The connection has been hijacked, it's safe to return.
}

func (l *listener) Accept() (manet.Conn, error) {
	select {
	case c, ok := <-l.incoming:
		if !ok {
			return nil, fmt.Errorf("listener is closed")
		}

		mnc, err := manet.WrapNetConn(c)
		if err != nil {
			c.Close()
			return nil, err
		}

		return mnc, nil
	case <-l.closed:
		return nil, fmt.Errorf("listener is closed")
	}
}

func (l *listener) Multiaddr() ma.Multiaddr {
	return l.laddr
}
