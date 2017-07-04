package websocket

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	tpt "github.com/libp2p/go-libp2p-transport"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
	mafmt "github.com/whyrusleeping/mafmt"
	ws "golang.org/x/net/websocket"
	wsGorilla "gx/ipfs/QmdKzkTPWmQ4nc8McYVb9TJYzRGmux9sqySQBD4nhbjQpf/."
)

var WsProtocol = ma.Protocol{
	Code:  477,
	Name:  "ws",
	VCode: ma.CodeToVarint(477),
}

var WsFmt = mafmt.And(mafmt.TCP, mafmt.Base(WsProtocol.Code))

var WsCodec = &manet.NetCodec{
	NetAddrNetworks:  []string{"websocket"},
	ProtocolName:     "ws",
	ConvertMultiaddr: ConvertWebsocketMultiaddrToNetAddr,
	ParseNetAddr:     ParseWebsocketNetAddr,
}

// Default gorilla upgrader
var upgrader = wsGorilla.Upgrader{}

func init() {
	err := ma.AddProtocol(WsProtocol)
	if err != nil {
		panic(fmt.Errorf("error registering websocket protocol: %s", err))
	}

	manet.RegisterNetCodec(WsCodec)
}

func ConvertWebsocketMultiaddrToNetAddr(maddr ma.Multiaddr) (net.Addr, error) {
	_, host, err := manet.DialArgs(maddr)
	if err != nil {
		return nil, err
	}

	a := &ws.Addr{
		URL: &url.URL{
			Host: host,
		},
	}
	return a, nil
}

func ParseWebsocketNetAddr(a net.Addr) (ma.Multiaddr, error) {
	wsa, ok := a.(*ws.Addr)
	if !ok {
		return nil, fmt.Errorf("not a websocket address")
	}

	tcpaddr, err := net.ResolveTCPAddr("tcp", wsa.Host)
	if err != nil {
		return nil, err
	}

	tcpma, err := manet.FromNetAddr(tcpaddr)
	if err != nil {
		return nil, err
	}

	wsma, err := ma.NewMultiaddr("/ws")
	if err != nil {
		return nil, err
	}

	return tcpma.Encapsulate(wsma), nil
}

type WebsocketTransport struct{}

func (t *WebsocketTransport) Matches(a ma.Multiaddr) bool {
	return WsFmt.Matches(a)
}

func (t *WebsocketTransport) Dialer(_ ma.Multiaddr, opts ...tpt.DialOpt) (tpt.Dialer, error) {
	return &dialer{}, nil
}

type dialer struct{}

func parseMultiaddr(a ma.Multiaddr) (string, error) {
	_, host, err := manet.DialArgs(a)
	if err != nil {
		return "", err
	}

	return "ws://" + host, nil
}

func (d *dialer) Dial(raddr ma.Multiaddr) (tpt.Conn, error) {
	return d.DialContext(context.Background(), raddr)
}

func (d *dialer) DialContext(ctx context.Context, raddr ma.Multiaddr) (tpt.Conn, error) {
	wsurl, err := parseMultiaddr(raddr)
	if err != nil {
		return nil, err
	}

	// TODO: figure out origins, probably don't work for us
	// header := http.Header{}
	// header.Set("Origin", "http://127.0.0.1:0/")
	wscon, _, err := wsGorilla.DefaultDialer.Dial(wsurl, nil)
	if err != nil {
		return nil, err
	}

	mnc, err := manet.WrapNetConn(NewGorillaNetConn(wscon))
	if err != nil {
		return nil, err
	}

	return &wsConn{
		Conn: mnc,
	}, nil
}

func (d *dialer) Matches(a ma.Multiaddr) bool {
	return WsFmt.Matches(a)
}

type wsConn struct {
	manet.Conn
	t tpt.Transport
}

func (c *wsConn) Transport() tpt.Transport {
	return c.t
}

type listener struct {
	manet.Listener

	incoming chan *conn

	tpt tpt.Transport

	origin *url.URL
}

type conn struct {
	*GorillaNetConn

	done func()
}

func (c *conn) Close() error {
	c.done()
	return c.GorillaNetConn.Close()
}

func (t *WebsocketTransport) Listen(a ma.Multiaddr) (tpt.Listener, error) {
	list, err := manet.Listen(a)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse("http://" + list.Addr().String())
	if err != nil {
		return nil, err
	}

	tlist := t.wrapListener(list, u)

	http.HandleFunc("/", tlist.handleWsConn)
	go http.Serve(list.NetListener(), nil)

	return tlist, nil
}

func (t *WebsocketTransport) wrapListener(l manet.Listener, origin *url.URL) *listener {
	return &listener{
		Listener: l,
		incoming: make(chan *conn),
		tpt:      t,
		origin:   origin,
	}
}

func (l *listener) handleWsConn(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade websocket", 400)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	wrapped := NewGorillaNetConn(c)
	l.incoming <- &conn{
		GorillaNetConn: &wrapped,
		done:           cancel,
	}

	// wait until conn gets closed, otherwise the handler closes it early
	<-ctx.Done()
}

func (l *listener) Accept() (tpt.Conn, error) {
	c, ok := <-l.incoming
	if !ok {
		return nil, fmt.Errorf("listener is closed")
	}

	mnc, err := manet.WrapNetConn(c)
	if err != nil {
		return nil, err
	}

	return &wsConn{
		Conn: mnc,
		t:    l.tpt,
	}, nil
}

func (l *listener) Multiaddr() ma.Multiaddr {
	wsma, err := ma.NewMultiaddr("/ws")
	if err != nil {
		panic(err)
	}

	return l.Listener.Multiaddr().Encapsulate(wsma)
}

var _ tpt.Transport = (*WebsocketTransport)(nil)

type GorillaNetConn struct {
	Inner              *wsGorilla.Conn
	DefaultMessageType int
}

func (c GorillaNetConn) Read(b []byte) (n int, err error) {
	fmt.Println("reading")
	_, r, err := c.Inner.NextReader()
	if err != nil {
		return 0, err
	}

	return r.Read(b)
}

func (c GorillaNetConn) Write(b []byte) (n int, err error) {
	fmt.Printf("write %s\n", string(b))
	if err := c.Inner.WriteMessage(c.DefaultMessageType, b); err != nil {
		return 0, err
	}

	return len(b), nil
}

func (c GorillaNetConn) Close() error {
	return c.Inner.Close()
}

func (c GorillaNetConn) LocalAddr() net.Addr {
	return c.Inner.LocalAddr()
}

func (c GorillaNetConn) RemoteAddr() net.Addr {
	return c.Inner.RemoteAddr()
}

func (c GorillaNetConn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}

	return c.SetReadDeadline(t)
}

func (c GorillaNetConn) SetReadDeadline(t time.Time) error {
	return c.Inner.SetReadDeadline(t)
}

func (c GorillaNetConn) SetWriteDeadline(t time.Time) error {
	return c.Inner.SetWriteDeadline(t)
}

func NewGorillaNetConn(raw *wsGorilla.Conn) GorillaNetConn {
	return GorillaNetConn{
		Inner:              raw,
		DefaultMessageType: wsGorilla.BinaryMessage,
	}
}
