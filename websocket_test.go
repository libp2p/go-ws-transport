package websocket

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/sec"
	"github.com/libp2p/go-libp2p-core/sec/insecure"
	"github.com/libp2p/go-libp2p-core/test"
	"github.com/libp2p/go-libp2p-core/transport"

	csms "github.com/libp2p/go-conn-security-multistream"
	mplex "github.com/libp2p/go-libp2p-mplex"
	ttransport "github.com/libp2p/go-libp2p-testing/suites/transport"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	ma "github.com/multiformats/go-multiaddr"
)

func newUpgrader(t *testing.T) (peer.ID, transport.Upgrader) {
	t.Helper()
	id, m := newSecureMuxer(t)
	u, err := tptu.New(m, new(mplex.Transport))
	if err != nil {
		t.Fatal(err)
	}
	return id, u
}

func newSecureMuxer(t *testing.T) (peer.ID, sec.SecureMuxer) {
	t.Helper()
	priv, _, err := test.RandTestKeyPair(crypto.Ed25519, 256)
	if err != nil {
		t.Fatal(err)
	}
	id, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	var secMuxer csms.SSMuxer
	secMuxer.AddTransport(insecure.ID, insecure.NewWithIdentity(id, priv))
	return id, &secMuxer
}

func TestCanDial(t *testing.T) {
	d := &WebsocketTransport{}
	if !d.CanDial(ma.StringCast("/ip4/127.0.0.1/tcp/5555/ws")) {
		t.Fatal("expected to match websocket maddr, but did not")
	}
	if !d.CanDial(ma.StringCast("/ip4/127.0.0.1/tcp/5555/wss")) {
		t.Fatal("expected to match secure websocket maddr, but did not")
	}
	if d.CanDial(ma.StringCast("/ip4/127.0.0.1/tcp/5555")) {
		t.Fatal("expected to not match tcp maddr, but did")
	}
}

func TestDialWss(t *testing.T) {
	if _, err := net.LookupIP("nyc-1.bootstrap.libp2p.io"); err != nil {
		t.Skip("this test requries an internet connection and it seems like we currently don't have one")
	}
	raddr := ma.StringCast("/dns4/nyc-1.bootstrap.libp2p.io/tcp/443/wss")
	rid, err := peer.Decode("QmSoLueR4xBeUbY9WZ9xGUUxunbKWcrNFTDAadQJmocnWm")
	if err != nil {
		t.Fatal(err)
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	_, u := newUpgrader(t)
	tpt, err := New(u, network.NullResourceManager, WithTLSClientConfig(tlsConfig))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := tpt.Dial(context.Background(), raddr, rid)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := conn.OpenStream(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
}

func TestWebsocketTransport(t *testing.T) {
	t.Skip("This test is failing, see https://github.com/libp2p/go-ws-transport/issues/99")
	_, ua := newUpgrader(t)
	ta, err := New(ua, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, ub := newUpgrader(t)
	tb, err := New(ub, nil)
	if err != nil {
		t.Fatal(err)
	}

	ttransport.SubtestTransport(t, ta, tb, "/ip4/127.0.0.1/tcp/0/ws", "peerA")
}

func TestWebsocketListen(t *testing.T) {
	id, u := newUpgrader(t)
	tpt, err := New(u, network.NullResourceManager)
	if err != nil {
		t.Fatal(err)
	}
	l, err := tpt.Listen(ma.StringCast("/ip4/127.0.0.1/tcp/0/ws"))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	msg := []byte("HELLO WORLD")

	go func() {
		c, err := tpt.Dial(context.Background(), l.Multiaddr(), id)
		if err != nil {
			t.Error(err)
			return
		}
		str, err := c.OpenStream(context.Background())
		if err != nil {
			t.Error(err)
		}
		defer str.Close()

		if _, err = str.Write(msg); err != nil {
			t.Error(err)
			return
		}
	}()

	c, err := l.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	str, err := c.AcceptStream()
	if err != nil {
		t.Fatal(err)
	}
	defer str.Close()

	out, err := ioutil.ReadAll(str)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out, msg) {
		t.Fatal("got wrong message", out, msg)
	}
}

func TestConcurrentClose(t *testing.T) {
	_, u := newUpgrader(t)
	tpt, err := New(u, network.NullResourceManager)
	if err != nil {
		t.Fatal(err)
	}
	l, err := tpt.maListen(ma.StringCast("/ip4/127.0.0.1/tcp/0/ws"))
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
	_, u := newUpgrader(t)
	tpt, err := New(u, network.NullResourceManager)
	if err != nil {
		t.Fatal(err)
	}
	l, err := tpt.maListen(ma.StringCast("/ip4/127.0.0.1/tcp/0/ws"))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	msg := []byte(nil)

	go func() {
		c, err := tpt.maDial(context.Background(), l.Multiaddr())
		if err != nil {
			t.Error(err)
			return
		}
		defer c.Close()

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
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	buf := make([]byte, 100)
	n, err := c.Read(buf)
	if n != 0 {
		t.Errorf("read %d bytes, expected 0", n)
	}
	if err != io.EOF {
		t.Errorf("expected EOF, got err: %s", err)
	}
}
