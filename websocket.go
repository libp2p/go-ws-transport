// Package websocket implements a websocket based transport for go-libp2p.
package websocket

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/transport"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	ma "github.com/multiformats/go-multiaddr"
	mafmt "github.com/multiformats/go-multiaddr-fmt"
	manet "github.com/multiformats/go-multiaddr-net"
)

// WsProtocol is the multiaddr protocol definition for this transport.
//
// Deprecated: use `ma.ProtocolWithCode(ma.P_WS)
var WsProtocol = ma.ProtocolWithCode(ma.P_WS)
var WssProtocol = ma.Protocol{
	Code:  478,
	Name:  "wss",
	VCode: ma.CodeToVarint(478),
}

// WsFmt is multiaddr formatter for WsProtocol
var WsFmt = mafmt.And(mafmt.TCP, mafmt.Or(
	mafmt.Base(WsProtocol.Code),
	mafmt.Base(WssProtocol.Code),
))

// WsCodec is the multiaddr-net codec definition for the websocket transport
var WsCodec = &manet.NetCodec{
	NetAddrNetworks:  []string{"websocket"},
	ProtocolName:     "ws",
	ConvertMultiaddr: ConvertWebsocketMultiaddrToNetAddr,
	ParseNetAddr:     ParseWebsocketNetAddr,
}
var WssCodec = &manet.NetCodec{
	NetAddrNetworks:  []string{"websocket secure"},
	ProtocolName:     "wss",
	ConvertMultiaddr: ConvertWebsocketMultiaddrToNetAddr,
	ParseNetAddr:     ParseWebsocketNetAddr,
}

func init() {
	err := ma.AddProtocol(WssProtocol)
	if err != nil {
		panic(fmt.Errorf("error registering websocket secure protocol: %s", err))
	}

	manet.RegisterNetCodec(WsCodec)
	manet.RegisterNetCodec(WssCodec)
}

var _ transport.Transport = (*WebsocketTransport)(nil)

// WebsocketTransport is the actual go-libp2p transport
type WebsocketTransport struct {
	Upgrader *tptu.Upgrader
}

func New(u *tptu.Upgrader) *WebsocketTransport {
	return &WebsocketTransport{u}
}

func (t *WebsocketTransport) CanDial(a ma.Multiaddr) bool {
	return WsFmt.Matches(a)
}

func (t *WebsocketTransport) Protocols() []int {
	return []int{WsProtocol.Code}
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
