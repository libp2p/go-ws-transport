// Package websocket implements a websocket based transport for go-libp2p.
package websocket

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/transport"

	ws "github.com/gorilla/websocket"
	ma "github.com/multiformats/go-multiaddr"
	mafmt "github.com/multiformats/go-multiaddr-fmt"
	manet "github.com/multiformats/go-multiaddr/net"
)

// WsFmt is multiaddr formatter for WsProtocol
var WsFmt = mafmt.And(mafmt.TCP, mafmt.Base(ma.P_WS))

// This is _not_ WsFmt because we want the transport to stick to dialing fully
// resolved addresses.
var dialMatcher = mafmt.And(mafmt.IP, mafmt.Base(ma.P_TCP), mafmt.Or(mafmt.Base(ma.P_WS), mafmt.Base(ma.P_WSS)))

func init() {
	manet.RegisterFromNetAddr(ParseWebsocketNetAddr, "websocket")
	manet.RegisterToNetAddr(ConvertWebsocketMultiaddrToNetAddr, "ws")
	manet.RegisterToNetAddr(ConvertWebsocketMultiaddrToNetAddr, "wss")
}

// Default gorilla upgrader
var upgrader = ws.Upgrader{
	// Allow requests from *all* origins.
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Option func(*WebsocketTransport) error

// WithTLSClientConfig sets a TLS client configuration on the WebSocket Dialer. Only
// relevant for non-browser usages.
//
// Some useful use cases include setting InsecureSkipVerify to `true`, or
// setting user-defined trusted CA certificates.
func WithTLSClientConfig(c *tls.Config) Option {
	return func(t *WebsocketTransport) error {
		t.tlsClientConf = c
		return nil
	}
}

// WebsocketTransport is the actual go-libp2p transport
type WebsocketTransport struct {
	upgrader transport.Upgrader
	rcmgr    network.ResourceManager

	tlsClientConf *tls.Config
}

var _ transport.Transport = (*WebsocketTransport)(nil)

func New(u transport.Upgrader, rcmgr network.ResourceManager, opts ...Option) (*WebsocketTransport, error) {
	if rcmgr == nil {
		rcmgr = network.NullResourceManager
	}
	t := &WebsocketTransport{
		upgrader: u,
		rcmgr:    rcmgr,
	}
	for _, opt := range opts {
		if err := opt(t); err != nil {
			return nil, err
		}
	}
	return t, nil
}

func (t *WebsocketTransport) CanDial(a ma.Multiaddr) bool {
	return dialMatcher.Matches(a)
}

func (t *WebsocketTransport) Protocols() []int {
	return []int{ma.P_WS, ma.P_WSS}
}

func (t *WebsocketTransport) Proxy() bool {
	return false
}

func (t *WebsocketTransport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (transport.CapableConn, error) {
	connScope, err := t.rcmgr.OpenConnection(network.DirOutbound, true)
	if err != nil {
		return nil, err
	}
	macon, err := t.maDial(ctx, raddr)
	if err != nil {
		connScope.Done()
		return nil, err
	}
	return t.upgrader.Upgrade(ctx, t, macon, network.DirOutbound, p, connScope)
}

func (t *WebsocketTransport) maDial(ctx context.Context, raddr ma.Multiaddr) (manet.Conn, error) {
	wsurl, err := parseMultiaddr(raddr)
	if err != nil {
		return nil, err
	}

	wscon, _, err := ws.DefaultDialer.Dial(wsurl.String(), nil)
	if err != nil {
		return nil, err
	}

	mnc, err := manet.WrapNetConn(NewConn(wscon, wsurl.Scheme == "wss"))
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
	return t.upgrader.UpgradeListener(t, malist), nil
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
		laddr:    laddr,
		Listener: l,
		incoming: make(chan *Conn),
		closed:   make(chan struct{}),
	}, nil
}
