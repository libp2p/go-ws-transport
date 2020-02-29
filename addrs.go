package websocket

import (
	"fmt"
	"net"
	"net/url"

	ma "github.com/multiformats/go-multiaddr"
	mafmt "github.com/multiformats/go-multiaddr-fmt"
	manet "github.com/multiformats/go-multiaddr-net"
)

// WsProtocol is the multiaddr protocol definition for this transport.
//
// Deprecated: use `ma.ProtocolWithCode(ma.P_WS)
var WsProtocol = ma.ProtocolWithCode(ma.P_WS)

// WsFmt is multiaddr formatter for WsProtocol
var WsFmt = mafmt.And(mafmt.TCP, mafmt.Or(
	mafmt.Base(ma.P_WS),
	mafmt.Base(ma.P_WSS),
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
	manet.RegisterNetCodec(WsCodec)
	manet.RegisterNetCodec(WssCodec)
}

// Addr is an implementation of net.Addr for WebSocket.
type Addr struct {
	*url.URL
}

var _ net.Addr = (*Addr)(nil)

// Network returns the network type for a WebSocket, "websocket".
func (addr *Addr) Network() string {
	return "websocket"
}

// NewAddr creates an Addr with `ws` scheme (insecure).
//
// Deprecated. Use NewAddrWithScheme.
func NewAddr(host string) *Addr {
	// Older versions of the transport only supported insecure connections (i.e.
	// WS instead of WSS). Assume that is the case here.
	return NewAddrWithScheme(host, false)
}

// NewAddrWithScheme creates a new Addr using the given host string. isSecure
// should be true for WSS connections and false for WS.
func NewAddrWithScheme(host string, isSecure bool) *Addr {
	scheme := "ws"
	if isSecure {
		scheme = "wss"
	}
	return &Addr{
		URL: &url.URL{
			Scheme: scheme,
			Host:   host,
		},
	}
}

func ConvertWebsocketMultiaddrToNetAddr(maddr ma.Multiaddr) (net.Addr, error) {
	_, host, err := manet.DialArgs(maddr)
	if err != nil {
		return nil, err
	}

	// Assume ws scheme, then check if this is a wss multiaddr.
	var isSecure = false
	switch _, err := maddr.ValueForProtocol(ma.P_WSS); err {
	case nil:
		// This is a wss multiaddr, i.e. secure.
		isSecure = true
	case ma.ErrProtocolNotFound:
	default:
		// This is an unexpected error. Return it.
		return nil, err
	}

	return NewAddrWithScheme(host, isSecure), nil
}

func ParseWebsocketNetAddr(a net.Addr) (ma.Multiaddr, error) {
	wsa, ok := a.(*Addr)
	if !ok {
		return nil, fmt.Errorf("not a websocket address")
	}

	// Detect if host is IP address or DNS
	var tcpma ma.Multiaddr
	if ip := net.ParseIP(wsa.Hostname()); ip != nil {
		// Assume IP address
		tcpaddr, err := net.ResolveTCPAddr("tcp", wsa.Host)
		if err != nil {
			return nil, err
		}
		tcpma, err = manet.FromNetAddr(tcpaddr)
		if err != nil {
			return nil, err
		}
	} else {
		// Assume DNS name
		var err error
		tcpma, err = ma.NewMultiaddr(fmt.Sprintf("/dns/%s/tcp/%s", wsa.Hostname(), wsa.Port()))
		if err != nil {
			return nil, err
		}
	}

	wsma, err := ma.NewMultiaddr("/" + wsa.Scheme)
	if err != nil {
		return nil, err
	}

	return tcpma.Encapsulate(wsma), nil
}

func parseMultiaddr(a ma.Multiaddr) (string, error) {
	p := a.Protocols()

	_, host, err := manet.DialArgs(a)
	if err != nil {
		return "", err
	}

	return p[2].Name + "://" + host, nil
}
