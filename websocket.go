// Package websocket implements a websocket based transport for go-libp2p.
package websocket

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/transport"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	ma "github.com/multiformats/go-multiaddr"
)

var _ transport.Transport = (*WebsocketTransport)(nil)

// Option is the type implemented by functional options.
//
// Actual options that one can use vary based on build target environment, i.e.
// the options available on the browser differ from those available natively.
type Option func(cfg *WebsocketConfig) error

// WebsocketTransport is the actual go-libp2p transport
type WebsocketTransport struct {
	Upgrader *tptu.Upgrader
	Config   *WebsocketConfig
}

func New(u *tptu.Upgrader) *WebsocketTransport {
	return &WebsocketTransport{u, DefaultWebsocketConfig()}
}

// NewWithOptions returns a WebsocketTransport constructor function compatible
// with the libp2p.New host constructor.
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

func (t *WebsocketTransport) CanDial(a ma.Multiaddr) bool {
	return WsFmt.Matches(a)
}

func (t *WebsocketTransport) Protocols() []int {
	return []int{WsProtocol.Code, WssProtocol.Code}
}

func (t *WebsocketTransport) Proxy() bool {
	return false
}

func (t *WebsocketTransport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (transport.CapableConn, error) {
	macon, err := t.maDial(ctx, raddr)
	if err != nil {
		return nil, err
	}
	return t.Upgrader.UpgradeOutbound(ctx, t, macon, p)
}
