package websocket

import (
	"fmt"
	"net"
	"net/url"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

// Addr is an implementation of net.Addr for WebSocket.
type Addr struct {
	*url.URL
}

var _ net.Addr = (*Addr)(nil)

// Network returns the network type for a WebSocket, "websocket".
func (addr *Addr) Network() string {
	return "websocket"
}

// Deprecated. Use NewAddrWithSchemeAndDNSVersion.
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
	if _, err := maddr.ValueForProtocol(WssProtocol.Code); err == nil {
		// This is a wss multiaddr, set scheme to wss.
		isSecure = true
	} else if err != nil && err != ma.ErrProtocolNotFound {
		// Unexpected error
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
	host, err := a.ValueForProtocol(p[0].Code)
	if err != nil {
		return "", err
	}
	if p[0].Code == ma.P_IP6 {
		host = "[" + host + "]"
	}
	port, err := a.ValueForProtocol(ma.P_TCP)
	if err != nil {
		return "", err
	}
	return p[2].Name + "://" + host + ":" + port, nil
}
