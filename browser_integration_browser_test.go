// +build js,wasm

package websocket

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/sec/insecure"
	mplex "github.com/libp2p/go-libp2p-mplex"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	ma "github.com/multiformats/go-multiaddr"
)

func TestInBrowser(t *testing.T) {
	fmt.Println("Inside browser test")

	testPingPong(t)
	testDialWss(t)
}

func testDialWss(t *testing.T) {
	if _, err := http.Get("nyc-1.bootstrap.libp2p.io"); err != nil {
		t.Skip("this test requries an internet connection and it seems like we currently don't have one")
	}
	tpt := New(&tptu.Upgrader{
		Secure: insecure.New("browserPeer"),
		Muxer:  new(mplex.Transport),
	})
	knownWssAddr, err := ma.NewMultiaddr("/dns/nyc-1.bootstrap.libp2p.io/tcp/443/wss")
	if err != nil {
		t.Fatal(err)
	}
	knownWssPeerID, err := peer.IDB58Decode("QmSoLueR4xBeUbY9WZ9xGUUxunbKWcrNFTDAadQJmocnWm")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := tpt.Dial(context.Background(), knownWssAddr, knownWssPeerID)
	if err != nil {
		t.Fatal("could not dial known WSS addr: " + err.Error())
	}
	stream, err := conn.OpenStream()
	if err != nil {
		t.Fatal("could not open stream: " + err.Error())
	}
	defer stream.Close()
}

func testPingPong(t *testing.T) {
	tpt := New(&tptu.Upgrader{
		Secure: insecure.New("browserPeer"),
		Muxer:  new(mplex.Transport),
	})
	addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5555/ws")
	if err != nil {
		t.Fatal("could not parse multiaddress: " + err.Error())
	}
	conn, err := tpt.Dial(context.Background(), addr, "serverPeer")
	if err != nil {
		t.Fatal("could not dial server: " + err.Error())
	}
	defer conn.Close()

	stream, err := conn.AcceptStream()
	if err != nil {
		t.Fatal("could not accept stream: " + err.Error())
	}
	defer stream.Close()

	buf := bufio.NewReader(stream)
	msg, err := buf.ReadString('\n')
	if err != nil {
		t.Fatal("could not read ping message: " + err.Error())
	}
	expected := "ping\n"
	if msg != expected {
		t.Fatalf("Received wrong message. Expected %q but got %q", expected, msg)
	}

	_, err = stream.Write([]byte("pong\n"))
	if err != nil {
		t.Fatal("could not write pong message: " + err.Error())
	}

	// TODO(albrow): This hack is necessary in order to give the reader time to
	// finish. As soon as this test function returns, the browser window is
	// closed, which means there is no time for the other end of the connection to
	// read the "pong" message. We should find some way to remove this hack if
	// possible.
	time.Sleep(1 * time.Second)
}
