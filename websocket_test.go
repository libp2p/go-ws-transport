// +build !js

package websocket

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net"
	"testing"
	"testing/iotest"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/sec/insecure"
	mplex "github.com/libp2p/go-libp2p-mplex"
	ttransport "github.com/libp2p/go-libp2p-testing/suites/transport"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	ma "github.com/multiformats/go-multiaddr"
)

func TestCanDial(t *testing.T) {
	addrWs, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5555/ws")
	if err != nil {
		t.Fatal(err)
	}
	addrWss, err := ma.NewMultiaddr("/dns/example.com/tcp/5555/wss")
	if err != nil {
		t.Fatal(err)
	}

	addrTCP, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5555")
	if err != nil {
		t.Fatal(err)
	}

	d := &WebsocketTransport{}
	if !d.CanDial(addrWs) {
		t.Fatal("expected to match websocket maddr, but did not")
	}
	if !d.CanDial(addrWss) {
		t.Fatal("expected to match secure websocket maddr, but did not")
	}
	if d.CanDial(addrTCP) {
		t.Fatal("expected to not match tcp maddr, but did")
	}
}

func TestDialWss(t *testing.T) {
	if _, err := net.LookupIP("nyc-1.bootstrap.libp2p.io"); err != nil {
		t.Skip("this test requries an internet connection and it seems like we currently don't have one")
	}
	raddr, err := ma.NewMultiaddr("/dns/nyc-1.bootstrap.libp2p.io/tcp/443/wss")
	if err != nil {
		t.Fatal(err)
	}
	rid, err := peer.IDB58Decode("QmSoLueR4xBeUbY9WZ9xGUUxunbKWcrNFTDAadQJmocnWm")
	if err != nil {
		t.Fatal(err)
	}

	tpt := New(&tptu.Upgrader{
		Secure: insecure.New(rid),
		Muxer:  new(mplex.Transport),
	})
	conn, err := tpt.Dial(context.Background(), raddr, rid)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := conn.OpenStream()
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
}

func TestWebsocketTransport(t *testing.T) {
	ta := New(&tptu.Upgrader{
		Secure: insecure.New("peerA"),
		Muxer:  new(mplex.Transport),
	})
	tb := New(&tptu.Upgrader{
		Secure: insecure.New("peerB"),
		Muxer:  new(mplex.Transport),
	})

	zero := "/ip4/127.0.0.1/tcp/0/ws"
	ttransport.SubtestTransport(t, ta, tb, zero, "peerA")
}

func TestWebsocketListen(t *testing.T) {
	zero, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/0/ws")
	if err != nil {
		t.Fatal(err)
	}

	tpt := &WebsocketTransport{}
	l, err := tpt.maListen(zero)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	msg := []byte("HELLO WORLD")

	go func() {
		c, err := tpt.maDial(context.Background(), l.Multiaddr())
		if err != nil {
			t.Error(err)
			return
		}

		_, err = c.Write(msg)
		if err != nil {
			t.Error(err)
		}
		err = c.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	c, err := l.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	obr := iotest.OneByteReader(c)

	out, err := ioutil.ReadAll(obr)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out, msg) {
		t.Fatal("got wrong message", out, msg)
	}
}

func TestConcurrentClose(t *testing.T) {
	zero, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/0/ws")
	if err != nil {
		t.Fatal(err)
	}

	tpt := &WebsocketTransport{}
	l, err := tpt.maListen(zero)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	msg := []byte("HELLO WORLD")

	go func() {
		for i := 0; i < 100; i++ {
			c, err := tpt.maDial(context.Background(), l.Multiaddr())
			if err != nil {
				t.Error(err)
				return
			}

			go func() {
				_, _ = c.Write(msg)
			}()
			go func() {
				_ = c.Close()
			}()
		}
	}()

	for i := 0; i < 100; i++ {
		c, err := l.Accept()
		if err != nil {
			t.Fatal(err)
		}
		c.Close()
	}
}

func TestWriteZero(t *testing.T) {
	zero, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/0/ws")
	if err != nil {
		t.Fatal(err)
	}

	tpt := &WebsocketTransport{}
	l, err := tpt.maListen(zero)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	msg := []byte(nil)

	go func() {
		c, err := tpt.maDial(context.Background(), l.Multiaddr())
		defer c.Close()
		if err != nil {
			t.Error(err)
			return
		}

		for i := 0; i < 100; i++ {
			n, err := c.Write(msg)
			if n != 0 {
				t.Errorf("expected to write 0 bytes, wrote %d", n)
			}
			if err != nil {
				t.Error(err)
				return
			}
		}
	}()

	c, err := l.Accept()
	defer c.Close()
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 100)
	n, err := c.Read(buf)
	if n != 0 {
		t.Errorf("read %d bytes, expected 0", n)
	}
	if err != io.EOF {
		t.Errorf("expected EOF, got err: %s", err)
	}
}
