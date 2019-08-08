// +build js,wasm

package websocket

import (
	"bufio"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/sec/insecure"
	mplex "github.com/libp2p/go-libp2p-mplex"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	ma "github.com/multiformats/go-multiaddr"
)

func TestInBrowser(t *testing.T) {
	tpt := New(&tptu.Upgrader{
		Secure: insecure.New("browserPeer"),
		Muxer:  new(mplex.Transport),
	})
	addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5555/ws")
	if err != nil {
		t.Fatal("could not parse multiaddress:" + err.Error())
	}
	fmt.Println("parsed addr")
	conn, err := tpt.Dial(context.Background(), addr, "serverPeer")
	if err != nil {
		t.Fatal("could not dial server:" + err.Error())
	}
	fmt.Println("dialed server")
	defer conn.Close()

	stream, err := conn.AcceptStream()
	if err != nil {
		t.Fatal("could not accept stream:" + err.Error())
	}
	fmt.Println("accepted stream")
	defer stream.Close()

	buf := bufio.NewReader(stream)
	msg, err := buf.ReadString('\n')
	if err != nil {
		t.Fatal("could not read ping message:" + err.Error())
	}
	fmt.Println("read ping")
	expected := "ping\n"
	if msg != expected {
		t.Fatalf("Received wrong message. Expected %q but got %q", expected, msg)
	}

	_, err = stream.Write([]byte("pong\n"))
	if err != nil {
		t.Fatal("could not write pong message:" + err.Error())
	}
	fmt.Println("wrote pong")

	// TODO(albrow): This hack is necessary in order to give the reader time to
	// finish. We should find some way to remove it.
	time.Sleep(1 * time.Second)
}
