// +build js,wasm

package websocket

import (
	"context"
	"errors"
	"fmt"
	"syscall/js"

	"github.com/libp2p/go-libp2p-core/transport"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

// WebsocketTransport is the actual go-libp2p transport
type WebsocketTransport struct {
	Upgrader *tptu.Upgrader
	Config   *WebsocketConfig
}

func New(u *tptu.Upgrader) *WebsocketTransport {
	return &WebsocketTransport{
		Upgrader: u,
		Config:   DefaultWebsocketConfig(),
	}
}

// NewWithOptions returns a WebsocketTransport constructor function compatible
// with the libp2p.New host constructor. Currently in the browser no options
// are supported.
func NewWithOptions(opts ...Option) func(u *tptu.Upgrader) *WebsocketTransport {
	c := DefaultWebsocketConfig()

	// Apply functional options.
	for _, o := range opts {
		o(c)
	}

	return func(u *tptu.Upgrader) *WebsocketTransport {
		t := &WebsocketTransport{
			Upgrader: u,
			Config:   c,
		}
		return t
	}
}

func (t *WebsocketTransport) maDial(ctx context.Context, raddr ma.Multiaddr) (mnc manet.Conn, err error) {
	defer func() {
		if r := recover(); r != nil {
			mnc = nil
			switch e := r.(type) {
			case error:
				err = e
			default:
				err = fmt.Errorf("recovered from non-error value: (%T) %+v", e, e)
			}
		}
	}()

	wsurl, err := parseMultiaddr(raddr)
	if err != nil {
		return nil, err
	}

	rawConn := js.Global().Get("WebSocket").New(wsurl)
	conn := NewConn(rawConn)
	if err := conn.waitForOpen(); err != nil {
		conn.Close()
		return nil, err
	}
	mnc, err = manet.WrapNetConn(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return mnc, nil
}

func (t *WebsocketTransport) Listen(a ma.Multiaddr) (transport.Listener, error) {
	return nil, errors.New("Listen not implemented on js/wasm")
}
