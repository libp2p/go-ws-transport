package websocket

import (
	"fmt"
	"testing"

	ma "github.com/multiformats/go-multiaddr"
)

func TestMultiaddrParsing(t *testing.T) {
	addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5555/ws")
	if err != nil {
		t.Fatal(err)
	}

	res, err := parseMultiaddr(addr)
	if err != nil {
		t.Fatal(err)
	}
	if res != "ws://127.0.0.1:5555" {
		t.Fatal(fmt.Errorf("%s != ws://127.0.0.1:5555", res))
	}
}
