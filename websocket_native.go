// +build !js

package websocket

import (
	"context"
	"net"
	"net/url"

	ws "github.com/gorilla/websocket"
	"github.com/libp2p/go-libp2p-core/transport"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

// WebsocketTransport is the actual go-libp2p transport
type WebsocketTransport struct {
	Upgrader *tptu.Upgrader
	Config   *WebsocketConfig
	dialer   *ws.Dialer
}

func New(u *tptu.Upgrader) *WebsocketTransport {
	return &WebsocketTransport{
		Upgrader: u,
		Config:   DefaultWebsocketConfig(),
		dialer:   ws.DefaultDialer,
	}
}

// Option is the type implemented by functional options.
//
// Actual options that one can use vary based on build target environment, i.e.
// the options available on the browser differ from those available natively.
type Option func(cfg *WebsocketConfig) error

// NewWithOptions returns a WebsocketTransport constructor function compatible
// with the libp2p.New host constructor.
func NewWithOptions(opts ...Option) func(u *tptu.Upgrader) *WebsocketTransport {
	c := DefaultWebsocketConfig()

	// Apply functional options.
	for _, o := range opts {
		o(c)
	}

	// Configure ws.Dialer based on given TLSClientConfig
	dialer := new(ws.Dialer)
	(*dialer) = *ws.DefaultDialer
	dialer.TLSClientConfig = c.TLSClientConfig

	return func(u *tptu.Upgrader) *WebsocketTransport {
		t := &WebsocketTransport{
			Upgrader: u,
			Config:   c,
			dialer:   dialer,
		}
		return t
	}
}

func (t *WebsocketTransport) maDial(ctx context.Context, raddr ma.Multiaddr) (manet.Conn, error) {
	wsurl, err := parseMultiaddr(raddr)
	if err != nil {
		return nil, err
	}

	wscon, _, err := t.dialer.Dial(wsurl, nil)
	if err != nil {
		return nil, err
	}

	mnc, err := manet.WrapNetConn(NewConn(wscon))
	if err != nil {
		wscon.Close()
		return nil, err
	}
	return mnc, nil
}

func (t *WebsocketTransport) maListen(a ma.Multiaddr) (manet.Listener, error) {
	lnet, lnaddr, err := manet.DialArgs(a)
	if err != nil {
		return nil, err
	}

	nl, err := net.Listen(lnet, lnaddr)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse("http://" + nl.Addr().String())
	if err != nil {
		nl.Close()
		return nil, err
	}

	malist, err := t.wrapListener(nl, u)
	if err != nil {
		nl.Close()
		return nil, err
	}

	go malist.serve()

	return malist, nil
}

func (t *WebsocketTransport) Listen(a ma.Multiaddr) (transport.Listener, error) {
	malist, err := t.maListen(a)
	if err != nil {
		return nil, err
	}
	return t.Upgrader.UpgradeListener(t, malist), nil
}

func (t *WebsocketTransport) wrapListener(l net.Listener, origin *url.URL) (*listener, error) {
	laddr, err := manet.FromNetAddr(l.Addr())
	if err != nil {
		return nil, err
	}
	wsma, err := ma.NewMultiaddr("/ws")
	if err != nil {
		return nil, err
	}
	laddr = laddr.Encapsulate(wsma)

	return &listener{
		websocketUpgrader: t.Config.WebsocketUpgrader,
		laddr:             laddr,
		Listener:          l,
		incoming:          make(chan *Conn),
		closed:            make(chan struct{}),
	}, nil
}
