package websocket

import (
	"encoding/hex"
	"net/url"
	"testing"

	ma "github.com/multiformats/go-multiaddr"
)

func TestMultiaddrParsing(t *testing.T) {
	addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5555/ws/libp2pEndpoint")
	if err != nil {
		t.Fatal(err)
	}

	wsaddr, err := parseMultiaddr(addr)
	if err != nil {
		t.Fatal(err)
	}
	if wsaddr != "ws://127.0.0.1:5555/libp2pEndpoint" {
		t.Fatalf("expected ws://127.0.0.1:5555/libp2pEndpoint, got %s", wsaddr)
	}
}

type httpAddr struct {
	*url.URL
}

func (addr *httpAddr) Network() string {
	return "http"
}

func TestParseWebsocketNetAddr(t *testing.T) {
	notWs := &httpAddr{&url.URL{Host: "http://127.0.0.1:1234/libp2pEndpoint"}}
	_, err := ParseWebsocketNetAddr(notWs)
	if err.Error() != "not a websocket address" {
		t.Fatalf("expect \"not a websocket address\", got \"%s\"", err)
	}

	wsAddr := NewAddr("127.0.0.1:5555", "libp2pEndpoint")
	parsed, err := ParseWebsocketNetAddr(wsAddr)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.String() != "/ip4/127.0.0.1/tcp/5555/ws/libp2pEndpoint" {
		t.Fatalf("expected \"/ip4/127.0.0.1/tcp/5555/ws/libp2pEndpoint\", got \"%s\"", parsed.String())
	}
}

func TestConvertWebsocketMultiaddrToNetAddr(t *testing.T) {
	addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5555/ws/libp2pEndpoint")
	if err != nil {
		t.Fatal(err)
	}

	wsaddr, err := ConvertWebsocketMultiaddrToNetAddr(addr)
	if err != nil {
		t.Fatal(err)
	}
	if wsaddr.String() != "//127.0.0.1:5555/libp2pEndpoint" {
		t.Fatalf("expected //127.0.0.1:5555/libp2pEndpoint, got %s", wsaddr)
	}
	if wsaddr.Network() != "websocket" {
		t.Fatalf("expected network: \"websocket\", got \"%s\"", wsaddr.Network())
	}
}

func TestTranscoder(t *testing.T) {
	bytes, err := WsTranscoder.StringToBytes("libp2pEndpoint")
	if err != nil {
		t.Fatal(err)
	}

	str, err := WsTranscoder.BytesToString(bytes)
	if err != nil {
		t.Fatal(err)
	}

	if str != "libp2pEndpoint" {
		t.Fatalf("expected \"libp2pEndpoint\" but got \"%s\"", str)
	}

	// Now testing with a /
	bytes, err = WsTranscoder.StringToBytes("libp2p/endpoint")
	if bytes != nil || err == nil {
		t.Fatalf("endpoint with a / shouldn't works, but here got : %s", hex.Dump(bytes))
	}
}
