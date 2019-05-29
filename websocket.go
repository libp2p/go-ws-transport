// Package websocket implements a websocket based transport for go-libp2p.
package websocket

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/transport"

	tptu "github.com/libp2p/go-libp2p-transport-upgrader"

	ws "github.com/gorilla/websocket"
	ma "github.com/multiformats/go-multiaddr"
	mafmt "github.com/multiformats/go-multiaddr-fmt"
	manet "github.com/multiformats/go-multiaddr-net"
)

// WsProtocol is the multiaddr protocol definition for this transport.
var WsProtocol = ma.Protocol{
	Code:  477,
	Name:  "ws",
	VCode: ma.CodeToVarint(477),
}
var WssProtocol = ma.Protocol{
	Code:  478,
	Name:  "wss",
	VCode: ma.CodeToVarint(478),
}

// WsFmt is multiaddr formatter for WsProtocol
var WsFmt = mafmt.Or(
	mafmt.And(mafmt.TCP, mafmt.Base(WsProtocol.Code)),
	mafmt.And(mafmt.And(mafmt.DNS, mafmt.Base(ma.P_TCP)), mafmt.Base(WssProtocol.Code)),
)

// WsCodec is the multiaddr-net codec definition for the websocket transport
var WsCodec = &manet.NetCodec{
	NetAddrNetworks:  []string{"websocket"},
	ProtocolName:     "ws",
	ConvertMultiaddr: ConvertWebsocketMultiaddrToNetAddr,
	ParseNetAddr:     ParseWebsocketNetAddr,
}
var WssCodec = &manet.NetCodec{
	NetAddrNetworks:  []string{"websocket+tls"},
	ProtocolName:     "wss",
	ConvertMultiaddr: ConvertWebsocketMultiaddrToNetAddr,
	ParseNetAddr:     ParseWebsocketNetAddr,
}

// Default gorilla upgrader
var upgrader = ws.Upgrader{
	// Allow requests from *all* origins.
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func init() {
	err := ma.AddProtocol(WsProtocol)
	if err != nil {
		panic(fmt.Errorf("error registering websocket protocol: %s", err))
	}
	err = ma.AddProtocol(WssProtocol)
	if err != nil {
		panic(fmt.Errorf("error registering websocket+tls protocol: %s", err))
	}

	manet.RegisterNetCodec(WsCodec)
	manet.RegisterNetCodec(WssCodec)
}

// WebsocketTransport is the actual go-libp2p transport
type WebsocketTransport struct {
	Upgrader *tptu.Upgrader
	CertCfg  *tls.Config
}

func New(u *tptu.Upgrader) *WebsocketTransport {
	p, err := x509.SystemCertPool()
	var cfg *tls.Config
	if err == nil {
		cfg = &tls.Config{RootCAs: p}
	} else {
		cfg = &tls.Config{RootCAs: nil, InsecureSkipVerify: true}
	}
	return &WebsocketTransport{
		Upgrader: u,
		CertCfg:  cfg,
	}
}

var _ transport.Transport = (*WebsocketTransport)(nil)

func (t *WebsocketTransport) CanDial(a ma.Multiaddr) bool {
	return WsFmt.Matches(a)
}

func (t *WebsocketTransport) Protocols() []int {
	return []int{WsProtocol.Code, WssProtocol.Code}
}

func (t *WebsocketTransport) Proxy() bool {
	return false
}

func (t *WebsocketTransport) maDial(ctx context.Context, raddr ma.Multiaddr) (manet.Conn, error) {
	wsurl, err := parseMultiaddr(raddr)
	if err != nil {
		return nil, err
	}

	var wscon *ws.Conn
	if raddr.Protocols()[2].Code == WsProtocol.Code {
		wscon, _, err = ws.DefaultDialer.Dial(wsurl, nil)
	} else {
		u, err := url.Parse(wsurl)
		if err != nil {
			return nil, err
		}

		wsHeaders := http.Header{
			"Origin": {u.Host},
			// your milage may differ
			"Sec-WebSocket-Extensions": {"permessage-deflate; client_max_window_bits, x-webkit-deflate-frame"},
		}
		tlsconn, err := tls.Dial("tcp", u.Host, t.CertCfg)
		if err != nil {
			return nil, err
		}
		wscon, _, err = ws.NewClient(tlsconn, u, wsHeaders, 1024, 1024)
	}
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

func (t *WebsocketTransport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (transport.CapableConn, error) {
	macon, err := t.maDial(ctx, raddr)
	if err != nil {
		return nil, err
	}
	return t.Upgrader.UpgradeOutbound(ctx, t, macon, p)
}

var listenWss = fmt.Errorf("Unable to listen wss, you should use a reverse proxy like nginx or apache.")

func (t *WebsocketTransport) maListen(a ma.Multiaddr) (manet.Listener, error) {
	if a.Protocols()[2].Code == WsProtocol.Code {
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
	} else {
		return nil, listenWss
	}
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
		laddr:    laddr,
		Listener: l,
		incoming: make(chan *Conn),
		closed:   make(chan struct{}),
	}, nil
}
