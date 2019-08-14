// +build js,wasm

package websocket

import (
	"context"
	"errors"
	"syscall/js"

	"github.com/libp2p/go-libp2p-core/transport"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

func (t *WebsocketTransport) maDial(ctx context.Context, raddr ma.Multiaddr) (manet.Conn, error) {
	wsurl, err := parseMultiaddr(raddr)
	if err != nil {
		return nil, err
	}

	rawConn := js.Global().Get("WebSocket").New(wsurl)
	rawConn.Call("addEventListener", "error", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		js.Global().Get("console").Call("log", args[0])
		return nil
	}))
	conn := NewConn(rawConn)
	conn.waitForOpen()
	mnc, err := manet.WrapNetConn(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return mnc, nil
}

func (t *WebsocketTransport) Listen(a ma.Multiaddr) (transport.Listener, error) {
	return nil, errors.New("Listen not implemented on js/wasm")
}
