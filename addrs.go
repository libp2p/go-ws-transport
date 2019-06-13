package websocket

import (
	"fmt"
	"net"
	"net/url"
	"strings"

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

// NewAddr creates a new Addr using the given host string
func NewAddr(host string, path string) *Addr {
	return &Addr{
		URL: &url.URL{
			Host: host,
			Path: path,
		},
	}
}

func ConvertWebsocketMultiaddrToNetAddr(maddr ma.Multiaddr) (net.Addr, error) {
	_, host, err := manet.DialArgs(maddr)
	if err != nil {
		return nil, err
	}
	path, err := maddr.ValueForProtocol(WsProtocol.Code)
	if err != nil {
		return nil, err
	}

	return NewAddr(host, path), nil
}

var notWebsocketError = fmt.Errorf("not a websocket address")

func ParseWebsocketNetAddr(a net.Addr) (ma.Multiaddr, error) {
	wsa, ok := a.(*Addr)
	if !ok {
		return nil, notWebsocketError
	}

	pathl := strings.SplitN(wsa.Path, "/", 2)
	path := ""
	if len(pathl) == 1 {
		path = wsa.Path
	} else {
		path = pathl[1]
	}

	if strings.Contains(path, "/") {
		return nil, fmt.Errorf("Endpoint must be under root, not %s", path)
	}

	tcpaddr, err := net.ResolveTCPAddr("tcp", wsa.Host)
	if err != nil {
		return nil, err
	}

	tcpma, err := manet.FromNetAddr(tcpaddr)
	if err != nil {
		return nil, err
	}

	wsma, err := ma.NewMultiaddr("/ws/" + path)
	if err != nil {
		return nil, err
	}

	return tcpma.Encapsulate(wsma), nil
}

func parseMultiaddr(a ma.Multiaddr) (string, error) {
	_, host, err := manet.DialArgs(a)
	if err != nil {
		return "", err
	}

	endpoint, err := a.ValueForProtocol(WsProtocol.Code)
	if err != nil {
		return "", err
	}

	return "ws://" + host + "/" + endpoint, nil
}
